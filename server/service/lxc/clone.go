package lxc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/model"
)

// CloneParams 「从快照克隆容器」异步任务参数（task.Params JSON）。
type CloneParams struct {
	SrcName string `json:"src_name"`           // 源容器名
	Snap    string `json:"snap"`               // 源 zfs 快照名
	DstName string `json:"dst_name"`           // 新容器名
	Remark  string `json:"remark"`             // 新容器备注
}

// ParseCloneParams 反序列化克隆任务参数。
func ParseCloneParams(s string) (*CloneParams, error) {
	var p CloneParams
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ContainerSpecForQuota 返回容器的 CPU/Mem 规格（供 handler 做克隆配额校验）。
func ContainerSpecForQuota(name string) (cpu, memMB int, err error) {
	var row model.LXCCache
	if err := model.DB.Where("name = ?", name).First(&row).Error; err != nil {
		return 0, 0, err
	}
	return row.CPUShares, row.MemoryMB, nil
}

// containerExists 报告容器是否真实存在（lxcpath 下有 config 文件）。
func containerExists(name string) bool {
	if _, err := os.Stat(filepath.Join(config.GlobalConfig.LXCLxcPath, name, "config")); err == nil {
		return true
	}
	return false
}

// srcPrimaryVPC 读源容器 order0 的 VPC 绑定（switch/security group）；无绑定返回 0,0。
func srcPrimaryVPC(src string) (switchID, sgID uint) {
	var b model.VPCVMBinding
	if err := model.DB.Where("vm_name = ? AND kind = ? AND interface_order = ?", src, "lxc", 0).First(&b).Error; err == nil {
		return b.SwitchID, b.SecurityGroupID
	}
	return 0, 0
}

// SourcePrimarySwitchID 透出源容器主卡（order0）所属交换机 ID，供克隆 handler 预检固定 IP。
// 无绑定返回 0。
func SourcePrimarySwitchID(src string) uint {
	switchID, _ := srcPrimaryVPC(src)
	return switchID
}

// ValidateCloneFromSnapshot 同步预校验（handler 即时返回 400 用）。
// 任务体内 CloneFromSnapshot 会再跑一次（异步期间状态可能变化）。
func ValidateCloneFromSnapshot(srcName, snap, dstName string) error {
	if err := validateContainerName(dstName); err != nil {
		return err
	}
	if srcName == "" {
		return errors.New("源容器不能为空")
	}
	if snap == "" {
		return errors.New("必须选择一个快照")
	}
	if srcName == dstName {
		return errors.New("新容器名不能与源容器同名")
	}
	if !containerExists(srcName) {
		return fmt.Errorf("源容器 %s 不存在", srcName)
	}
	if containerExists(dstName) {
		return fmt.Errorf("容器 %s 已存在", dstName)
	}
	// 仅 zfs 支持快照克隆（lxc-copy 无「从指定快照克隆」能力）
	if !isZfsContainer(srcName) {
		return errors.New("当前容器非 zfs 存储，暂不支持快照克隆")
	}
	snaps, err := ListSnapshots(srcName)
	if err != nil {
		return fmt.Errorf("查询源快照失败: %w", err)
	}
	found := false
	for _, s := range snaps {
		if s.Name == snap {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("快照 %s 不存在", snap)
	}
	return nil
}

// rewriteCloneRootfsPath 把克隆 config 里继承自源的 rootfs.path 改写为新容器自己的。
func rewriteCloneRootfsPath(src, dst string) error {
	cfgPath := filepath.Join(config.GlobalConfig.LXCLxcPath, dst, "config")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("读克隆 config 失败: %w", err)
	}
	rewritten := rewriteRootfsPathForClone(string(data),
		filepath.Join(config.GlobalConfig.LXCLxcPath, src, "rootfs"),
		filepath.Join(config.GlobalConfig.LXCLxcPath, dst, "rootfs"))
	if err := os.WriteFile(cfgPath, []byte(rewritten), 0644); err != nil {
		return fmt.Errorf("写克隆 config 失败: %w", err)
	}
	return nil
}

// applyCloneOverrides 把克隆 config 里继承自源的主机名/MAC 改写为新容器的值（公用 SetConfigKeys 原地改写）。
// 必须改写而非追加：zfs CoW 克隆继承了源的整个 config（其中已有源的 lxc.uts.name / lxc.net.0.hwaddr），
// 追加会留下「源值 + 克隆值」的重复键。
func applyCloneOverrides(dst, mac string) error {
	cfgPath := filepath.Join(config.GlobalConfig.LXCLxcPath, dst, "config")
	return SetConfigKeys(cfgPath, []ConfigKV{
		{"lxc.uts.name", dst},
		{"lxc.net.0.hwaddr", mac},
	})
}

// CloneFromSnapshot 从源容器的指定 zfs 快照 CoW 克隆出新容器（异步任务调用）。
// 新容器继承源 config（rootfs/网卡/cgroup/autostart），仅刷新 MAC+主机名，并接入与源相同的 VPC。
// zfs origin 克隆：新容器 dataset 依赖源快照，源快照在新容器存在期间不可删除。
func CloneFromSnapshot(params *CloneParams, progress func(int, string)) error {
	if progress == nil {
		progress = func(int, string) {}
	}
	if err := ValidateCloneFromSnapshot(params.SrcName, params.Snap, params.DstName); err != nil {
		return err
	}
	lxcpath := config.GlobalConfig.LXCLxcPath

	// 读源容器缓存行（继承 CPU/Mem/Autostart/Group/Backing/Owner）
	var src model.LXCCache
	if err := model.DB.Where("name = ?", params.SrcName).First(&src).Error; err != nil {
		return fmt.Errorf("查询源容器记录失败: %w", err)
	}
	switchID, sgID := srcPrimaryVPC(params.SrcName)
	mac := genMacByName(params.DstName)

	// 1) zfs CoW 克隆 + 改写 rootfs.path
	progress(20, "克隆快照为容器（zfs）")
	parent, err := ZfsResolveParent(lxcpath)
	if err != nil {
		return err
	}
	if err := zfsCloneContainerFromSnapshot(parent, params.SrcName, params.Snap, params.DstName); err != nil {
		return fmt.Errorf("zfs 克隆失败: %w", err)
	}
	if err := rewriteCloneRootfsPath(params.SrcName, params.DstName); err != nil {
		_ = DestroyContainer(params.DstName)
		return err
	}

	// 2) 覆盖 MAC + 主机名（LXC last-wins）
	progress(50, "写入新容器配置")
	if err := applyCloneOverrides(params.DstName, mac); err != nil {
		_ = DestroyContainer(params.DstName)
		return err
	}

	// 3) 写缓存行（继承源规格）
	row := model.LXCCache{
		Name:          params.DstName,
		OwnerUsername: src.OwnerUsername,
		Status:        "STOPPED",
		Template:      "clone:" + params.SrcName,
		CPUShares:     src.CPUShares,
		MemoryMB:      src.MemoryMB,
		Backing:       src.Backing,
		Autostart:     src.Autostart,
		Remark:        params.Remark,
		GroupName:     src.GroupName,
		MacAddress:    mac,
		Present:       true,
	}
	if err := model.DB.Create(&row).Error; err != nil {
		_ = DestroyContainer(params.DstName)
		return fmt.Errorf("保存容器记录失败: %w", err)
	}

	// 4) 自动开机 + 接入与源相同的 VPC + 回填运行态
	// 改写 rootfs /etc/hostname 为本容器名（systemd/OpenRC 启动读它、覆盖 lxc.uts.name）
	setRootfsHostname(params.DstName)
	progress(80, "启动容器")
	if err := StartContainer(params.DstName); err != nil {
		logger.App.Warn("克隆容器启动失败（已创建，保持停止态）", "name", params.DstName, "error", err)
	}
	progress(90, "接入 VPC 网络")
	if err := AttachContainerToVPC(params.DstName, switchID, sgID); err != nil {
		logger.App.Warn("克隆容器 VPC 接入失败", "name", params.DstName, "error", err)
	}
	_ = RefreshRuntimeFields(params.DstName)

	progress(100, "完成")
	return nil
}
