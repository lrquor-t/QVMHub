package template

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/service/lxc/zfsbacking"
	"qvmhub/utils"
)

// MakeTemplateParams 「从容器制作模板」任务参数（task.Params JSON）。
type MakeTemplateParams struct {
	SrcName           string `json:"src_name"`
	Name              string `json:"name"`
	DisplayName       string `json:"display_name"`
	Description       string `json:"description"`
	PostCreateCommand string `json:"post_create_command"`
	OwnerUsername     string `json:"owner_username"`
}

// ValidateMakeFromContainer 同步预校验（handler 即时返回 400 用）。
// 任务体内 MakeFromContainer 会再跑一次防御性校验（异步期间状态可能变化）。
func ValidateMakeFromContainer(srcName, name string) error {
	if err := validateTemplateName(name); err != nil {
		return err
	}
	if srcName == "" {
		return errors.New("源容器不能为空")
	}
	if isBaseContainer(srcName) {
		return errors.New("不能从模板容器制作模板")
	}
	if srcName == name {
		return errors.New("模板名称不能与源容器同名")
	}
	if !existsContainer(srcName) {
		return fmt.Errorf("源容器 %s 不存在", srcName)
	}
	if state := containerState(srcName); state != "STOPPED" {
		return fmt.Errorf("源容器必须处于停止状态（当前：%s）", state)
	}
	var cnt int64
	model.DB.Model(&model.LXCTemplate{}).Where("name = ?", name).Count(&cnt)
	if cnt > 0 {
		return fmt.Errorf("模板 %s 已存在", name)
	}
	if existsContainer(baseContainerName(name)) {
		return fmt.Errorf("基底容器 %s 已存在", baseContainerName(name))
	}
	return nil
}

// containerState 取容器 State（lxc-info 解析；失败返回空串）。
func containerState(name string) string {
	res := utils.ExecCommandQuiet("lxc-info", "-n", name)
	if res.ExitCode != 0 {
		return ""
	}
	return parseLxcInfoState(res.Stdout)
}

// detectSourceBacking 优先取 LXCCache.Backing，回退解析源 config 的 rootfs.path scheme，再回退全局默认。
func detectSourceBacking(srcName string) string {
	var row model.LXCCache
	if err := model.DB.Where("name = ?", srcName).First(&row).Error; err == nil && row.Backing != "" {
		return row.Backing
	}
	cfgPath := filepath.Join(config.GlobalConfig.LXCLxcPath, srcName, "config")
	if data, err := os.ReadFile(cfgPath); err == nil {
		return backingFromRootfsPath(extractRootfsPathFromConfig(string(data)))
	}
	return config.GlobalConfig.LXCDefaultBacking
}

// sanitizeClonedBaseConfig 净化克隆出的基底 config：
//  1. rootfs.path 指向源容器的 → 改写为基底自己的（zfs 克隆会继承源路径；dir/overlay 的 lxc-copy 已正确，无匹配则 no-op）；
//  2. 经 composeBaseConfig 剥 net/cgroup/主机名，追加干净 net 块 + lxc.uts.name=base。
func sanitizeClonedBaseConfig(base, srcName, arch string) error {
	cfgPath := filepath.Join(config.GlobalConfig.LXCLxcPath, base, "config")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("读克隆基底 config 失败: %w", err)
	}
	lxcpath := config.GlobalConfig.LXCLxcPath
	rewritten := zfsbacking.RewriteRootfsPathForClone(string(data),
		filepath.Join(lxcpath, srcName, "rootfs"),
		filepath.Join(lxcpath, base, "rootfs"))
	return os.WriteFile(cfgPath, []byte(composeBaseConfig(rewritten, arch, base)), 0644)
}

// MakeFromContainer 把已停止的源容器 rootfs 克隆成新金底模板（异步任务调用）。
//	dir/overlay：lxc-copy（与 cloneContainer 一致）；
//	zfs：快照源→send|receive 全量拷贝成独立基底 dataset→净化 config→打 @base（cloneContainer 克隆建容器依赖 @base）。
func MakeFromContainer(params *MakeTemplateParams, progress func(int, string)) error {
	if progress == nil {
		progress = func(int, string) {}
	}
	if err := ValidateMakeFromContainer(params.SrcName, params.Name); err != nil {
		return err
	}
	if params.OwnerUsername == "" {
		params.OwnerUsername = "admin"
	}
	hostArch, err := HostArchLXC()
	if err != nil {
		return err
	}
	lxcpath := config.GlobalConfig.LXCLxcPath
	backing := detectSourceBacking(params.SrcName)
	if backing == "zfs" {
		if _, err := zfsbacking.ResolveParent(lxcpath); err != nil {
			return fmt.Errorf("backing=zfs 校验失败（lxc 目录不在 zfs 上？）: %w", err)
		}
	}
	// os-release best-effort；arch 跟随宿主；rootfs 目录恒为 <lxcpath>/<src>/rootfs。
	rootfsDir := filepath.Join(lxcpath, params.SrcName, "rootfs")
	distro, release := readOSReleaseFromDir(rootfsDir)
	base := baseContainerName(params.Name)

	progress(20, "克隆容器为模板基底（"+backing+"）")
	if backing == "zfs" {
		parent, err := zfsbacking.ResolveParent(lxcpath)
		if err != nil {
			return err
		}
		snap := "_tmplmake"
		// 清理上次进程被杀可能残留的临时快照，否则 zfs 拒绝重复创建导致重试失败（best-effort，忽略错误）。
		_ = zfsbacking.DestroyContainerSnapshot(parent, params.SrcName, snap)
		if err := zfsbacking.SnapshotContainer(parent, params.SrcName, snap); err != nil {
			return err
		}
		cleanupSnap := func() { _ = zfsbacking.DestroyContainerSnapshot(parent, params.SrcName, snap) }
		if err := zfsbacking.CopyContainerBySendRecv(parent, params.SrcName, snap, base); err != nil {
			cleanupSnap()
			return err
		}
		progress(60, "净化基底配置")
		if err := sanitizeClonedBaseConfig(base, params.SrcName, hostArch); err != nil {
			_ = zfsbacking.DestroyBase(parent, base)
			cleanupSnap()
			return err
		}
		if err := zfsbacking.SnapshotBase(parent, base); err != nil {
			_ = zfsbacking.DestroyBase(parent, base)
			cleanupSnap()
			return err
		}
		cleanupSnap()
	} else {
		// lxc-copy 的真实错误常打到 stdout（stderr 多为空），错误必须带 stdout。
		res := utils.ExecCommandLongRunning("lxc-copy", "-n", params.SrcName, "-N", base, "-B", backing)
		if res.Error != nil {
			return fmt.Errorf("克隆失败: %w (lxc-copy stdout: %q)", res.Error, res.Stdout)
		}
		progress(60, "净化基底配置")
		if err := sanitizeClonedBaseConfig(base, params.SrcName, hostArch); err != nil {
			_ = destroyContainerQuiet(base)
			return err
		}
	}

	progress(85, "写入模板记录")
	tpl := model.LXCTemplate{
		Name:              params.Name,
		DisplayName:       orDefault(params.DisplayName, params.Name),
		Distro:            distro,
		Release:           release,
		Arch:              orDefault(hostArch, "amd64"),
		Description:       params.Description,
		BaseContainerName: base,
		Backing:           backing,
		RootfsSizeBytes:   rootfsSizeBytes(base), // 克隆完成后计算基底 rootfs 表观大小（du -sb）
		CloneVisible:      true,
		OwnerUsername:     params.OwnerUsername,
		PostCreateCommand: params.PostCreateCommand,
	}
	if err := model.DB.Create(&tpl).Error; err != nil {
		_ = destroyBase(base, backing)
		return fmt.Errorf("保存模板记录失败: %w", err)
	}
	logger.App.Info("LXC 模板制作完成（从容器）", "name", params.Name, "base", base, "src", params.SrcName)
	progress(100, "完成")
	return nil
}
