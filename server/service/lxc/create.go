package lxc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/service/lxc/template"
	"qvmhub/utils"
)

// CreateContainerParams 异步创建容器参数（task.Params JSON）。
type CreateContainerParams struct {
	Name            string                   `json:"name"`
	Template        string                   `json:"template"`
	OwnerUsername   string                   `json:"owner_username"`
	Remark          string                   `json:"remark"`
	GroupName       string                   `json:"group_name"`
	CPUShares       int                      `json:"cpu_shares"`
	MemoryMB        int                      `json:"memory_mb"`
	Autostart       bool                     `json:"autostart"`
	SwitchID        uint                     `json:"switch_id"`
	SecurityGroupID uint                     `json:"security_group_id"`
	Source          string                   `json:"source"`             // clone（默认/空）| download
	Distro          string                   `json:"distro"`             // download 模式：发行版
	Release         string                   `json:"release"`            // download 模式：版本
	Arch            string                   `json:"arch"`               // download 模式：架构
	DiskLimitGB     int                      `json:"disk_limit_gb"`      // zfs backing：容器 rootfs refquota（GB），0=不限
	ExtraNics       []AddLXCInterfaceRequest `json:"extra_nics"`
}

// ParseCreateContainerParams 反序列化任务参数 JSON。
func ParseCreateContainerParams(s string) (*CreateContainerParams, error) {
	var p CreateContainerParams
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

var containerNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)

func validateContainerName(name string) error {
	if name == "" {
		return errors.New("容器名称不能为空")
	}
	if isReservedName(name) {
		return errors.New("名称使用了保留前缀")
	}
	if !containerNameRE.MatchString(name) {
		return errors.New("名称只能含小写字母、数字、连字符，2-63 字符")
	}
	return nil
}

// ValidateName 导出容器名校验，供 handler 批量预检复用（避免正则重复）。
func ValidateName(name string) error { return validateContainerName(name) }

func isReservedName(name string) bool {
	prefix := config.GlobalConfig.LXCBasePrefix
	return len(name) > len(prefix) && name[:len(prefix)] == prefix
}

// CreateContainer 由模板克隆创建容器（异步任务调用）。progress 上报进度。
func CreateContainer(params *CreateContainerParams, progress func(int, string)) error {
	if params.Source == "download" {
		return createFromDownload(params, progress)
	}
	if progress == nil {
		progress = func(int, string) {}
	}
	if err := validateContainerName(params.Name); err != nil {
		return err
	}
	if params.OwnerUsername == "" {
		params.OwnerUsername = "admin"
	}
	tpl, err := template.GetTemplate(params.Template)
	if err != nil {
		return err
	}
	if tpl.Disabled {
		return errors.New("模板已禁用")
	}
	progress(10, "校验完成，开始克隆")

	// 克隆（按 backing 分支）：zfs 走手动 clone（CoW）+ 改 config；dir/overlay 走 lxc-copy。
	progress(20, "克隆容器（"+tpl.Backing+"）")
	if err := cloneContainer(params.Name, tpl, params.DiskLimitGB); err != nil {
		return err
	}
	progress(50, "克隆完成，写入配置")

	// 由容器名派生 per-container MAC，写入容器 config 的 lxc.net.0.hwaddr 并存 DB，保证两处一致。
	mac := genMacByName(params.Name)

	// 覆写克隆 config：per-clone cgroup/autostart/mac
	if err := applyCloneConfig(params, mac); err != nil {
		_ = DestroyContainer(params.Name)
		return err
	}

	// 写缓存行
	row := model.LXCCache{
		Name:          params.Name,
		OwnerUsername: params.OwnerUsername,
		Status:        "STOPPED",
		Template:      params.Template,
		CPUShares:     params.CPUShares,
		MemoryMB:      params.MemoryMB,
		Backing:       tpl.Backing,
		Autostart:     params.Autostart,
		Remark:        params.Remark,
		GroupName:     params.GroupName,
		MacAddress:    mac,
		Present:       true,
	}
	if err := model.DB.Create(&row).Error; err != nil {
		_ = DestroyContainer(params.Name)
		return fmt.Errorf("保存容器记录失败: %w", err)
	}

	// 改写 rootfs /etc/hostname 为本容器名（systemd/OpenRC 启动读它、覆盖 lxc.uts.name）
	setRootfsHostname(params.Name)
	progress(80, "启动容器")
	// 创建后默认启动，便于分配 IP。
	if err := StartContainer(params.Name); err != nil {
		logger.App.Warn("容器启动失败（已创建，保持停止态）", "name", params.Name, "error", err)
	}
	progress(90, "接入 VPC 网络")
	if err := AttachContainerToVPC(params.Name, params.SwitchID, params.SecurityGroupID); err != nil {
		logger.App.Warn("容器 VPC 接入失败", "name", params.Name, "error", err)
	}
	// VPC 接入后回填运行态 veth/ip 到缓存。
	_ = RefreshRuntimeFields(params.Name)

	// 可选 PostCreateCommand
	if tpl.PostCreateCommand != "" {
		_ = utils.ExecCommandQuiet("bash", "-c", "lxc-attach -n "+utils.ShellSingleQuote(params.Name)+" -- "+tpl.PostCreateCommand)
	}
	progress(100, "完成")
	return nil
}

// cloneContainer 按 backing 克隆基底。
// zfs：zfs clone <parent>/<base>@base → <parent>/<name>（mountpoint <lxcpath>/<name>），克隆继承基底
// config+rootfs（CoW），把 config 的 rootfs.path 改成 <lxcpath>/<name>/rootfs。
// dir/overlay：lxc-copy（overlay 在 LXC 5.0.2 克隆会失败，错误带 stdout）。
func cloneContainer(name string, tpl *model.LXCTemplate, diskLimitGB int) error {
	lxcpath := config.GlobalConfig.LXCLxcPath
	if tpl.Backing == "zfs" {
		parent, err := ZfsResolveParent(lxcpath)
		if err != nil {
			return fmt.Errorf("zfs 克隆失败: %w", err)
		}
		if err := zfsCloneContainer(parent, tpl.BaseContainerName, name); err != nil {
			return fmt.Errorf("zfs 克隆失败: %w", err)
		}
		// 磁盘上限（refquota）：仅 zfs backing、且填了值才设
		if diskLimitGB > 0 {
			if err := ZfsSetContainerRefquota(parent, name, int64(diskLimitGB)*1024*1024*1024); err != nil {
				return fmt.Errorf("设置磁盘上限失败: %w", err)
			}
		}
		// 克隆 dataset 已挂载在 <lxcpath>/<name>，config 继承自基底；改 rootfs.path 指向自己的 rootfs。
		cfgPath := filepath.Join(lxcpath, name, "config")
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			return fmt.Errorf("读克隆 config 失败: %w", err)
		}
		rewritten := rewriteRootfsPathForClone(string(data),
			filepath.Join(lxcpath, tpl.BaseContainerName, "rootfs"),
			filepath.Join(lxcpath, name, "rootfs"))
		if err := os.WriteFile(cfgPath, []byte(rewritten), 0644); err != nil {
			return fmt.Errorf("写克隆 config 失败: %w", err)
		}
		return nil
	}
	// lxc-copy 的真实错误常打到 stdout（stderr 多为空），错误必须带 stdout，否则排查无门。
	res := utils.ExecCommandLongRunning("lxc-copy", "-n", tpl.BaseContainerName, "-N", name, "-B", tpl.Backing)
	if res.Error != nil {
		return fmt.Errorf("克隆失败: %w (lxc-copy stdout: %q)", res.Error, res.Stdout)
	}
	return nil
}

func applyCloneConfig(p *CreateContainerParams, mac string) error {
	cfg := filepath.Join(config.GlobalConfig.LXCLxcPath, p.Name, "config")
	// per-clone 覆盖项：原地改写（已存在改值并去重，不存在再追加），避免与基底模板的值留下重复键
	pairs := []ConfigKV{
		{"lxc.uts.name", p.Name}, // 覆盖基底模板的 uts.name（=lxc__tmpl__<base>），改为本容器名
		{"lxc.cgroup2.cpu.weight", itoaDefault(p.CPUShares, 256)},
		{"lxc.cgroup2.memory.max", memMax(p.MemoryMB)},
		{"lxc.start.auto", autoVal(p.Autostart)},
		{"lxc.net.0.hwaddr", mac},
	}
	// 选定交换机时显式写 link（覆盖基底模板继承值）；未选则继承基底（保持现状）
	if link := resolveNIC0Link(p.SwitchID, ""); link != "" {
		pairs = append(pairs, ConfigKV{"lxc.net.0.link", link})
	}
	return SetConfigKeys(cfg, pairs)
}

// setRootfsHostname 把容器 rootfs 内的 /etc/hostname 改写为容器名。
// 动机：systemd/OpenRC 启动读 /etc/hostname 设置主机名，会覆盖 config 的 lxc.uts.name；
// 模板/快照克隆继承源 rootfs 的 /etc/hostname，导致克隆容器主机名仍是源名。
// 仅 dir: backing（含 zfs，rootfs 是 dataset 下子目录）处理；overlay 等只读 lower 不误写。best-effort。
func setRootfsHostname(name string) {
	cfgPath := filepath.Join(config.GlobalConfig.LXCLxcPath, name, "config")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return
	}
	rootfs := rootfsDirFromConfig(string(data))
	if rootfs == "" {
		return
	}
	_ = os.WriteFile(filepath.Join(rootfs, "etc", "hostname"), []byte(name+"\n"), 0644)
}

// rootfsDirFromConfig 取 lxc.rootfs.path 指向的可写 rootfs 目录（仅识别 dir:<path>）。
// overlay:<lower>:<upper> 等返回空，避免误写到只读 lower。
func rootfsDirFromConfig(cfg string) string {
	for _, line := range strings.Split(cfg, "\n") {
		key, sep := parseConfigKV(line)
		if sep == "" || key != "lxc.rootfs.path" {
			continue
		}
		eq := strings.Index(line, "=")
		val := strings.TrimSpace(line[eq+1:])
		if strings.HasPrefix(val, "dir:") {
			return strings.TrimSpace(val[len("dir:"):])
		}
		return ""
	}
	return ""
}

// 以下小工具仅 create 流程使用；共享工具（genMacByName/
// RefreshRuntimeFields）见 command.go。
func itoaDefault(v, def int) string {
	if v <= 0 {
		v = def
	}
	return fmt.Sprintf("%d", v)
}

func memMax(mb int) string {
	if mb <= 0 {
		return "512M"
	}
	return fmt.Sprintf("%dM", mb)
}

func autoVal(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// resolveNIC0LinkPure 是 resolveNIC0Link 的纯逻辑核心（便于单测，无 DB 副作用）。
// switchID==0 → fallback（clone 传 "" 表示继承基底、download 传 "br-ovs"）；
// 选定交换机：查到且 BridgeName 非空 → sw.BridgeName；否则回退 br-ovs。
func resolveNIC0LinkPure(switchID uint, sw model.VPCSwitch, found bool, fallback string) string {
	if switchID == 0 {
		return fallback
	}
	if !found || strings.TrimSpace(sw.BridgeName) == "" {
		return "br-ovs"
	}
	return strings.TrimSpace(sw.BridgeName)
}

// resolveNIC0Link 查交换机后决定主网卡 lxc.net.0.link 写入值。
func resolveNIC0Link(switchID uint, fallback string) string {
	var sw model.VPCSwitch
	err := model.DB.First(&sw, switchID).Error
	return resolveNIC0LinkPure(switchID, sw, err == nil, fallback)
}

// createFromDownload 用 lxc-create -t download 从官方镜像建容器（一次性，非模板克隆）。
// backing 跟随 LXCDefaultBacking：zfs 先建 dataset 再 lxc-create；dir 直接 lxc-create。
func createFromDownload(params *CreateContainerParams, progress func(int, string)) error {
	if progress == nil {
		progress = func(int, string) {}
	}
	if err := validateContainerName(params.Name); err != nil {
		return err
	}
	if params.OwnerUsername == "" {
		params.OwnerUsername = "admin"
	}
	lxcpath := config.GlobalConfig.LXCLxcPath
	backing := config.GlobalConfig.LXCDefaultBacking

	// zfs：先建容器 dataset（lxc-create -t download 会把 config+rootfs 填进去）
	zfsParent := ""
	if backing == "zfs" {
		p, err := ZfsResolveParent(lxcpath)
		if err != nil {
			return err
		}
		zfsParent = p
		if err := zfsCreateContainerDataset(zfsParent, params.Name); err != nil {
			return err
		}
		// 磁盘上限（refquota）：仅 zfs backing、且填了值才设
		if params.DiskLimitGB > 0 {
			if err := ZfsSetContainerRefquota(zfsParent, params.Name, int64(params.DiskLimitGB)*1024*1024*1024); err != nil {
				return fmt.Errorf("设置磁盘上限失败: %w", err)
			}
		}
	}

	progress(20, "下载镜像并创建容器…")
	cr := utils.ExecCommandLongRunning("lxc-create", "-t", "download", "-n", params.Name, "--",
		"-d", params.Distro, "-r", params.Release, "-a", params.Arch)
	if cr.Error != nil {
		if zfsParent != "" {
			_ = zfsDestroyContainer(zfsParent, params.Name) // 回滚 dataset
		}
		return fmt.Errorf("lxc-create -t download 失败: %w (stdout: %q)", cr.Error, cr.Stdout)
	}

	mac := genMacByName(params.Name)
	if err := applyDownloadConfig(params, mac); err != nil {
		_ = DestroyContainer(params.Name)
		return err
	}

	row := model.LXCCache{
		Name:          params.Name,
		OwnerUsername: params.OwnerUsername,
		Status:        "STOPPED",
		Template:      "download:" + params.Distro,
		Backing:       backing,
		CPUShares:     params.CPUShares,
		MemoryMB:      params.MemoryMB,
		Autostart:     params.Autostart,
		Remark:        params.Remark,
		GroupName:     params.GroupName,
		MacAddress:    mac,
		Present:       true,
	}
	if err := model.DB.Create(&row).Error; err != nil {
		_ = DestroyContainer(params.Name)
		return fmt.Errorf("保存容器记录失败: %w", err)
	}

	// 改写 rootfs /etc/hostname 为本容器名（systemd/OpenRC 启动读它、覆盖 lxc.uts.name）
	setRootfsHostname(params.Name)
	progress(80, "启动容器")
	if err := StartContainer(params.Name); err != nil {
		logger.App.Warn("容器启动失败（已创建，保持停止态）", "name", params.Name, "error", err)
	}
	progress(90, "接入 VPC 网络")
	if err := AttachContainerToVPC(params.Name, params.SwitchID, params.SecurityGroupID); err != nil {
		logger.App.Warn("容器 VPC 接入失败", "name", params.Name, "error", err)
	}
	_ = RefreshRuntimeFields(params.Name)
	progress(100, "完成")
	return nil
}

// applyDownloadConfig 给 lxc-create -t download 生成的容器 config 写回：net.0.link
// （覆盖模板默认 lxcbr0）+ per-container mac/cgroup/autostart。原地改写，避免与模板值重复。
func applyDownloadConfig(p *CreateContainerParams, mac string) error {
	cfg := filepath.Join(config.GlobalConfig.LXCLxcPath, p.Name, "config")
	pairs := []ConfigKV{
		{"lxc.net.0.link", resolveNIC0Link(p.SwitchID, "br-ovs")},
		{"lxc.net.0.hwaddr", mac},
		{"lxc.cgroup2.cpu.weight", itoaDefault(p.CPUShares, 256)},
		{"lxc.cgroup2.memory.max", memMax(p.MemoryMB)},
		{"lxc.start.auto", autoVal(p.Autostart)},
	}
	return SetConfigKeys(cfg, pairs)
}
