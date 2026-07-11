package pool

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"qvmhub/config"
	"qvmhub/model"
	"qvmhub/utils"
)

// ── ZFS 存储池类型定义 ──

// ZFSPoolRequest 创建 ZFS 存储池的请求参数。
type ZFSPoolRequest struct {
	DeviceIDs   []string `json:"device_ids"`   // 选中的磁盘设备 ID 列表
	PoolName    string   `json:"pool_name"`    // zpool 名称
	VdevType    string   `json:"vdev_type"`    // stripe / mirror / raidz1 / raidz2 / raidz3
	Ashift      string   `json:"ashift"`       // 扇区对齐，默认 12（4K）
	Compression string   `json:"compression"`  // lz4(默认) / zstd / off / gzip
	DatasetName string   `json:"dataset_name"` // VM 落盘数据集名，默认 vm-disks
	MountPath   string   `json:"mount_path"`   // 数据集挂载点，留空自动生成
	AtimeOff    bool     `json:"atime_off"`    // 是否关闭 atime（默认前端传 true）
}

// ZPoolInfo 扫描得到的 ZFS 存储池信息。
type ZPoolInfo struct {
	Name           string           `json:"name"`
	Size           int64            `json:"size"`
	Alloc          int64            `json:"alloc"`
	Free           int64            `json:"free"`
	Health         string           `json:"health"`
	VdevType       string           `json:"vdev_type"`
	ExpandVdevType string           `json:"expand_vdev_type"` // 扩容锁定类型（纯池=该类型、混合="mixed"）
	Members        []string         `json:"members"`
	Datasets       []ZFSDatasetInfo `json:"datasets"`
}

// ZFSDatasetInfo ZFS 数据集信息。
type ZFSDatasetInfo struct {
	Name       string `json:"name"`       // 如 tank/vm-disks
	Mountpoint string `json:"mountpoint"` // 挂载点（可能为 -/none/legacy）
	Mounted    bool   `json:"mounted"`
	Used       int64  `json:"used"`
	Avail      int64  `json:"avail"`
}

// ComponentName 返回去掉 pool/ 前缀的数据集组件名。
func (d ZFSDatasetInfo) ComponentName(poolName string) string {
	return strings.TrimPrefix(d.Name, poolName+"/")
}

// ── 纯逻辑（可单测） ──

var zfsPoolNameRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.\-]{0,62}$`)

var zfsReservedNames = map[string]bool{
	"mirror": true, "raidz": true, "raidz1": true, "raidz2": true, "raidz3": true,
	"spare": true, "log": true, "cache": true,
}

var zfsDatasetNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.\-]{0,62}$`)

// validateZFSPoolName 校验 zpool 名称合法性。
func validateZFSPoolName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("存储池名称不能为空")
	}
	if len(name) > 63 {
		return fmt.Errorf("存储池名称过长（最多 63 字符）")
	}
	if !zfsPoolNameRe.MatchString(name) {
		return fmt.Errorf("名称只能以字母开头，包含字母、数字、下划线、点或短横线")
	}
	// 禁止 c 加数字开头（与 SCSI 设备名 cXtXdX 冲突）
	if len(name) >= 2 && (name[0] == 'c' || name[0] == 'C') && name[1] >= '0' && name[1] <= '9' {
		return fmt.Errorf("名称不能以 c 加数字开头（与设备名冲突）")
	}
	if zfsReservedNames[strings.ToLower(name)] {
		return fmt.Errorf("名称不能使用保留字: %s", name)
	}
	return nil
}

// validateZFSDatasetName 校验单个数据集组件名。
func validateZFSDatasetName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("数据集名称不能为空")
	}
	if strings.Contains(name, "/") {
		return fmt.Errorf("数据集名称不能包含 /")
	}
	if !zfsDatasetNameRe.MatchString(name) {
		return fmt.Errorf("名称只能包含字母、数字、下划线、点或短横线")
	}
	return nil
}

// isValidZFSVdevType 判断是否为支持的 vdev 类型。
func isValidZFSVdevType(v string) bool {
	switch v {
	case "stripe", "mirror", "raidz1", "raidz2", "raidz3":
		return true
	}
	return false
}

// normalizeZFSVdevType 规范化 vdev 类型，空/非法值降级为 stripe。
func normalizeZFSVdevType(v string) string {
	v = strings.TrimSpace(v)
	if !isValidZFSVdevType(v) {
		return "stripe"
	}
	return v
}

// zfsVdevMinDisks 返回各 vdev 类型所需的最少磁盘数；未知类型返回 0。
func zfsVdevMinDisks(vdevType string) int {
	switch vdevType {
	case "stripe":
		return 1
	case "mirror":
		return 2
	case "raidz1":
		return 3
	case "raidz2":
		return 4
	case "raidz3":
		return 5
	}
	return 0
}

// zfsVdevKeyword 返回 zpool create 中使用的 vdev 关键字，stripe 返回空串。
func zfsVdevKeyword(vdevType string) string {
	switch vdevType {
	case "mirror":
		return "mirror"
	case "raidz1":
		return "raidz1"
	case "raidz2":
		return "raidz2"
	case "raidz3":
		return "raidz3"
	}
	return ""
}

// vdevTypeLabel 返回 vdev 类型的中文标签（用于提示）。
func vdevTypeLabel(v string) string {
	switch v {
	case "stripe":
		return "条带/单盘"
	case "mirror":
		return "镜像 (mirror)"
	case "raidz1":
		return "RAIDZ1"
	case "raidz2":
		return "RAIDZ2"
	case "raidz3":
		return "RAIDZ3"
	}
	return v
}

// buildZpoolCreateArgs 拼装 zpool create 参数（纯函数，假定输入已校验）。
func buildZpoolCreateArgs(poolName, ashift, vdevType string, devicePaths []string) []string {
	args := []string{"create", "-f", "-o", "ashift=" + ashift, poolName}
	if kw := zfsVdevKeyword(vdevType); kw != "" {
		args = append(args, kw)
	}
	args = append(args, devicePaths...)
	return args
}

// resolveStableDevicePaths 把 /dev/sdX 形式的设备路径优先替换为 aliases 中的稳定路径
// （如 /dev/disk/by-id/wwn-0x...）。aliases 为 nil 或某路径未命中、对应值为空时，
// 一律回退为原始 devicePaths 元素。纯函数，便于单测；aliases 由 readDeviceByIDAliases
// 在调用处扫描 /dev/disk/by-id 提供。
func resolveStableDevicePaths(devicePaths []string, aliases map[string]string) []string {
	out := make([]string, 0, len(devicePaths))
	for _, p := range devicePaths {
		if stable, ok := aliases[p]; ok && strings.TrimSpace(stable) != "" {
			out = append(out, stable)
			continue
		}
		out = append(out, p)
	}
	return out
}

// mergeStableAliases 按优先级合并多个「真实路径 → 稳定路径」来源：先出现者胜出，后续不覆盖；
// 空值视为缺失，让后续来源补位。调用方按优先级从高到低传入（ZFS 场景：by-id 优先于 by-path）。
// 纯函数，便于单测。
func mergeStableAliases(sources ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, src := range sources {
		for k, v := range src {
			if merged[k] == "" && strings.TrimSpace(v) != "" {
				merged[k] = v
			}
		}
	}
	return merged
}

// scanDevLinks 扫描指定 glob（如 /dev/disk/by-id/*）下的设备符号链接，建立
// 「真实路径(/dev/sda) → 稳定路径」的映射。分区级链接（如 by-path/...-part1）
// readlink 指向 /dev/sda1，不会与整盘 /dev/sda 混淆。
func scanDevLinks(pattern string) map[string]string {
	aliases := make(map[string]string)
	script := fmt.Sprintf(`for p in %s; do [ -e "$p" ] || continue; t=$(readlink -f "$p" 2>/dev/null || true); [ -n "$t" ] && echo "$t|$p"; done`, pattern)
	result := utils.ExecShell(script)
	if result.Error != nil {
		return aliases
	}
	for _, line := range strings.Split(result.Stdout, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "|", 2)
		if len(parts) != 2 {
			continue
		}
		if aliases[parts[0]] == "" {
			aliases[parts[0]] = parts[1]
		}
	}
	return aliases
}

// readStableDeviceAliases 按优先级收集 ZFS 成员盘的稳定设备路径别名，回退链：
//  1. /dev/disk/by-id  —— 基于硬件 WWN/序列号，盘换槽位也不变（最稳）；
//  2. /dev/disk/by-path —— 基于 PCI/SCSI 拓扑位置，虚拟盘/无序列号盘可由此锁定；
//  3. 两类都未命中时由 resolveStableDevicePaths 回退到 /dev/sdX。
//
// 注意：by-id 在无序列号的虚拟盘上往往缺失（udev 不生成），故必须叠加 by-path 兜底，
// 否则在虚拟化环境下会退化为 /dev/sdX，失去稳定路径的意义。
func readStableDeviceAliases() map[string]string {
	return mergeStableAliases(
		scanDevLinks("/dev/disk/by-id/*"),
		scanDevLinks("/dev/disk/by-path/*"),
	)
}

// normalizeZFSAshift 规范化 ashift，仅允许 0/9/12/13，否则默认 12。
func normalizeZFSAshift(a string) string {
	switch strings.TrimSpace(a) {
	case "0", "9", "12", "13":
		return strings.TrimSpace(a)
	}
	return "12"
}

// normalizeZFSCompression 规范化压缩算法，非法值降级为 lz4。
func normalizeZFSCompression(c string) string {
	c = strings.ToLower(strings.TrimSpace(c))
	switch c {
	case "lz4", "zstd", "off", "gzip", "lzjb":
		return c
	}
	if strings.HasPrefix(c, "gzip-") {
		return c
	}
	return "lz4"
}

// ── ZFS 可用性检测 ──

// ZFSAvailable 检测宿主机是否安装了 zfs/zpool 命令。
func ZFSAvailable() bool {
	_, err1 := exec.LookPath("zpool")
	_, err2 := exec.LookPath("zfs")
	return err1 == nil && err2 == nil
}

// ── 创建 ZFS 存储池 ──

// CreateZFSPool 创建 ZFS 存储池，按顺序执行：
// zpool create → 设置压缩/atime → 创建 vm-disks 数据集 → 设置挂载点 → 配置目录权限 → 存储配置。
func CreateZFSPool(ctx context.Context, req ZFSPoolRequest, progress func(int, string)) error {
	if err := validateZFSPoolName(req.PoolName); err != nil {
		return err
	}
	vdev := normalizeZFSVdevType(req.VdevType)
	if len(req.DeviceIDs) == 0 {
		return fmt.Errorf("至少需要选择一个物理磁盘")
	}
	minDisks := zfsVdevMinDisks(vdev)
	if len(req.DeviceIDs) < minDisks {
		return fmt.Errorf("%s 至少需要 %d 块磁盘，当前选择 %d 块", vdevTypeLabel(vdev), minDisks, len(req.DeviceIDs))
	}
	ashift := normalizeZFSAshift(req.Ashift)
	compression := normalizeZFSCompression(req.Compression)
	dataset := strings.TrimSpace(req.DatasetName)
	if dataset == "" {
		dataset = "vm-disks"
	}
	if err := validateZFSDatasetName(dataset); err != nil {
		return fmt.Errorf("数据集名称非法: %w", err)
	}
	mountPath := strings.TrimSpace(req.MountPath)
	if mountPath == "" {
		mountPath = defaultStorageMountPath("zfs-" + req.PoolName + "-" + dataset)
	}
	atimeVal := "on"
	if req.AtimeOff {
		atimeVal = "off"
	}

	// 1) 校验并收集设备路径（复用 LVM 的 PV 校验，已拒绝 zfs_member）
	progress(5, "正在校验选中的磁盘...")
	devicePaths, err := validateAndCollectPVTargets(req.DeviceIDs)
	if err != nil {
		return fmt.Errorf("校验磁盘失败: %w", err)
	}

	// 1.5) 优先用稳定路径（by-id → by-path）创建 zpool，避免盘符变动导致的隐患与误选；
	//      两类稳定标识都未命中时回退到 /dev/sdX。
	devicePaths = resolveStableDevicePaths(devicePaths, readStableDeviceAliases())

	// 2) 检查 pool 名冲突
	progress(10, "检查存储池名称冲突...")
	if zpoolExists(req.PoolName) {
		return fmt.Errorf("ZFS 存储池 %s 已存在", req.PoolName)
	}

	// 3) 创建 zpool
	progress(20, fmt.Sprintf("正在创建 ZFS 存储池 %s（%s）...", req.PoolName, vdevTypeLabel(vdev)))
	createArgs := buildZpoolCreateArgs(req.PoolName, ashift, vdev, devicePaths)
	if r := utils.ExecCommandContextWithTimeout(ctx, "zpool", 5*time.Minute, createArgs...); r.Error != nil {
		return fmt.Errorf("zpool create 失败: %s", r.Stderr)
	}

	// 4) 设置池级属性
	progress(50, "正在设置存储池属性（压缩/atime）...")
	if err := setZFSProp(ctx, req.PoolName, "compression", compression); err != nil {
		rollbackZpool(req.PoolName)
		return err
	}
	// atime 设置失败不致命
	_ = setZFSProp(ctx, req.PoolName, "atime", atimeVal)

	// 5) 创建 vm-disks 数据集
	progress(65, fmt.Sprintf("正在创建数据集 %s/%s ...", req.PoolName, dataset))
	fullDS := req.PoolName + "/" + dataset
	if r := utils.ExecCommandContextWithTimeout(ctx, "zfs", 2*time.Minute, "create", fullDS); r.Error != nil {
		rollbackZpool(req.PoolName)
		return fmt.Errorf("创建数据集失败: %s", r.Stderr)
	}

	// 6) 设置数据集挂载点
	progress(75, fmt.Sprintf("设置数据集挂载点 %s ...", mountPath))
	if r := utils.ExecCommandContextWithTimeout(ctx, "zfs", 2*time.Minute, "set", "mountpoint="+mountPath, fullDS); r.Error != nil {
		rollbackZpool(req.PoolName)
		return fmt.Errorf("设置数据集挂载点失败: %s", r.Stderr)
	}

	// 7) 配置虚拟机磁盘目录权限（挂载点即 VM 磁盘目录）
	progress(85, "正在配置虚拟机磁盘目录权限...")
	if err := ensureVMStorageDir(mountPath); err != nil {
		progress(85, fmt.Sprintf("警告: 配置目录权限失败: %v", err))
	}

	// 8) 保存存储池配置
	progress(95, "正在保存存储池配置...")
	deviceID := normalizeStorageDeviceID("zfs-" + req.PoolName)
	cfg := model.HostStoragePool{DeviceID: deviceID}
	if err := model.DB.Where("device_id = ?", deviceID).Assign(map[string]interface{}{
		"display_name": req.PoolName + "/" + dataset,
		"enabled":      true,
		"mount_path":   mountPath,
	}).FirstOrCreate(&cfg).Error; err != nil {
		return fmt.Errorf("保存存储池配置失败: %w", err)
	}

	progress(100, "ZFS 存储池创建完成")
	return nil
}

// ── 删除 ZFS 存储池 ──

// DeleteZFSPool 销毁指定 ZFS 存储池及其所有数据集。
func DeleteZFSPool(ctx context.Context, poolName string, progress func(int, string)) error {
	poolName = strings.TrimSpace(poolName)
	if poolName == "" {
		return fmt.Errorf("存储池名称不能为空")
	}
	if !zpoolExists(poolName) {
		return fmt.Errorf("ZFS 存储池 %s 不存在", poolName)
	}

	// 安全检查：拒绝挂载于系统关键路径的数据集
	progress(5, "正在检查挂载点安全...")
	datasets := scanZFSDatasets(poolName)
	for _, ds := range datasets {
		switch strings.TrimSpace(ds.Mountpoint) {
		case "/", "/boot", "/boot/efi", "/usr", "/var", "/home":
			return fmt.Errorf("数据集 %s 挂载于系统关键路径 %s，禁止删除", ds.Name, ds.Mountpoint)
		}
	}

	// 销毁存储池
	progress(30, fmt.Sprintf("正在销毁 ZFS 存储池 %s ...", poolName))
	if r := utils.ExecCommandContextWithTimeout(ctx, "zpool", 5*time.Minute, "destroy", "-f", poolName); r.Error != nil {
		return fmt.Errorf("zpool destroy 失败: %s", r.Stderr)
	}

	// 清理数据库配置
	progress(80, "正在清理存储池配置...")
	clearZFSPoolConfigs(poolName, datasets)

	progress(100, "ZFS 存储池已删除")
	return nil
}

// ── ZFS 操作封装 ──

// zpoolExists 检查 zpool 是否已存在。
func zpoolExists(name string) bool {
	r := utils.ExecCommandQuiet("zpool", "list", "-H", "-o", "name", name)
	if r.Error != nil {
		return false
	}
	return strings.TrimSpace(r.Stdout) != ""
}

// setZFSProp 设置 ZFS 属性。
func setZFSProp(ctx context.Context, target, prop, value string) error {
	r := utils.ExecCommandContextWithTimeout(ctx, "zfs", 1*time.Minute, "set", prop+"="+value, target)
	if r.Error != nil {
		return fmt.Errorf("zfs set %s=%s 失败: %s", prop, value, r.Stderr)
	}
	return nil
}

// rollbackZpool 销毁刚创建的 zpool（创建失败回滚）。
func rollbackZpool(name string) {
	utils.ExecCommandQuietWithTimeout("zpool", 1*time.Minute, "destroy", "-f", name)
}

// clearZFSPoolConfigs 清除数据库中与该 zpool 相关的存储池配置。
func clearZFSPoolConfigs(poolName string, datasets []ZFSDatasetInfo) {
	if model.DB == nil {
		return
	}
	ids := []string{normalizeStorageDeviceID("zfs-" + poolName)}
	for _, ds := range datasets {
		ids = append(ids, normalizeStorageDeviceID("zfs-"+poolName+"-"+ds.ComponentName(poolName)))
	}
	for _, id := range ids {
		model.DB.Where("device_id = ?", id).Delete(&model.HostStoragePool{})
	}
}

// CreateZFSDataset 在已有 ZFS 存储池下创建数据集（如 zp01/vm-storage）。
func CreateZFSDataset(pool, name string) error {
	pool = strings.TrimSpace(pool)
	name = strings.TrimSpace(name)
	if pool == "" || name == "" {
		return fmt.Errorf("存储池名称和数据集名称不能为空")
	}
	// 校验名称（支持嵌套如 vm-storage/sub，逐段校验）
	for _, seg := range strings.Split(name, "/") {
		if !zfsPoolNameRe.MatchString(seg) {
			return fmt.Errorf("数据集名称段 %q 无效（字母开头，含字母/数字/连字符/下划线/点，最长 63 字符）", seg)
		}
	}
	// 校验 pool 存在
	if r := utils.ExecCommandQuiet("zpool", "list", "-Ho", "name", pool); r.Error != nil {
		return fmt.Errorf("ZFS 存储池 %s 不存在", pool)
	}
	// 创建数据集（-p 自动建父数据集；数据集已存在则 zfs create 报错）
	fullDS := pool + "/" + name
	if r := utils.ExecCommand("zfs", "create", "-p", fullDS); r.Error != nil {
		return fmt.Errorf("创建数据集 %s 失败: %w", fullDS, r.Error)
	}
	return nil
}

// DeleteZFSDataset 删除已有 ZFS 数据集（如 zp01/vm-storage）。不删 pool 根。
func DeleteZFSDataset(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || !strings.Contains(name, "/") {
		return fmt.Errorf("无效的数据集路径（需 pool/dataset 形式）")
	}
	parts := strings.SplitN(name, "/", 2)
	if !zfsPoolNameRe.MatchString(parts[0]) {
		return fmt.Errorf("无效的存储池名称")
	}
	if r := utils.ExecCommand("zfs", "destroy", name); r.Error != nil {
		return fmt.Errorf("删除数据集 %s 失败: %w", name, r.Error)
	}
	return nil
}

// ── ZFS 信息扫描 ──

// ListZPools 扫描宿主机所有 ZFS 存储池。
func ListZPools() ([]ZPoolInfo, error) {
	if !ZFSAvailable() {
		return nil, nil
	}
	r := utils.ExecCommandQuiet("zpool", "list", "-H", "-p", "-o", "name,size,alloc,free,health")
	if r.Error != nil {
		return nil, fmt.Errorf("zpool list 失败: %s", r.Stderr)
	}

	var pools []ZPoolInfo
	for _, line := range strings.Split(r.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		name := fields[0]
		size, _ := strconv.ParseInt(fields[1], 10, 64)
		alloc, _ := strconv.ParseInt(fields[2], 10, 64)
		free, _ := strconv.ParseInt(fields[3], 10, 64)
		health := fields[4]

		vdev, members := scanZPoolTopology(name)
		pools = append(pools, ZPoolInfo{
			Name:           name,
			Size:           size,
			Alloc:          alloc,
			Free:           free,
			Health:         health,
			VdevType:       vdev,
			ExpandVdevType: expandVdevType(scanZPoolTopVdevTypes(name)),
			Members:        members,
			Datasets:       scanZFSDatasets(name),
		})
	}
	sort.Slice(pools, func(i, j int) bool { return pools[i].Name < pools[j].Name })
	return pools, nil
}

// scanZPoolTopology 解析 zpool status，返回顶层 vdev 类型与成员设备路径列表。
func scanZPoolTopology(pool string) (vdev string, members []string) {
	vdev = "stripe"
	r := utils.ExecCommandQuiet("zpool", "status", "-L", pool)
	if r.Error != nil {
		return
	}
	inConfig := false
	seen := make(map[string]bool)
	for _, line := range strings.Split(r.Stdout, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "config:") {
			inConfig = true
			continue
		}
		if strings.HasPrefix(t, "errors:") {
			inConfig = false
			continue
		}
		if !inConfig {
			continue
		}
		f := strings.Fields(t)
		if len(f) < 1 {
			continue
		}
		n := f[0]
		if strings.HasPrefix(n, "/dev/") {
			if !seen[n] {
				seen[n] = true
				members = append(members, n)
			}
			continue
		}
		switch {
		case strings.HasPrefix(n, "raidz3-"):
			vdev = "raidz3"
		case strings.HasPrefix(n, "raidz2-"):
			vdev = "raidz2"
		case strings.HasPrefix(n, "raidz1-"):
			vdev = "raidz1"
		case strings.HasPrefix(n, "mirror-"):
			if vdev == "stripe" {
				vdev = "mirror"
			}
		}
	}
	return
}

// scanZFSDatasets 扫描指定 zpool 下所有数据集。LXC 容器/模板数据集聚合为一个节点
// （不逐个展示——容器多时刷屏 + 容量重复计算）。
func scanZFSDatasets(pool string) []ZFSDatasetInfo {
	r := utils.ExecCommandQuiet("zfs", "list", "-H", "-p", "-o", "name,mountpoint,mounted,used,avail", "-r", pool)
	if r.Error != nil {
		return nil
	}

	// 解析 LXC 根 dataset（如 zp01/lxc）—— 只聚合属于当前 pool 的 LXC 子树
	lxcParent := ""
	if lxcpath := config.GlobalConfig.LXCLxcPath; lxcpath != "" {
		if lr := utils.ExecCommandQuiet("zfs", "list", "-Ho", "name", lxcpath); lr.Error == nil {
			lp := strings.TrimSpace(lr.Stdout)
			if lp != "" && strings.HasPrefix(lp, pool+"/") {
				lxcParent = lp
			}
		}
	}

	var ds []ZFSDatasetInfo
	var lxcUsed int64
	var lxcAvail int64
	lxcAggregated := false
	for _, line := range strings.Split(r.Stdout, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		f := strings.Fields(t)
		if len(f) < 5 {
			continue
		}
		used, _ := strconv.ParseInt(f[3], 10, 64)
		avail, _ := strconv.ParseInt(f[4], 10, 64)

		// LXC 根 + 子数据集 → 聚合到一个节点（容器多时不逐个展示）
		if lxcParent != "" && (f[0] == lxcParent || strings.HasPrefix(f[0], lxcParent+"/")) {
			lxcUsed += used
			if !lxcAggregated {
				lxcAvail = avail // zfs 共享池：所有 dataset 的 avail 相同（= 池剩余）
			}
			lxcAggregated = true
			continue
		}

		ds = append(ds, ZFSDatasetInfo{
			Name:       f[0],
			Mountpoint: f[1],
			Mounted:    f[2] == "yes",
			Used:       used,
			Avail:      avail,
		})
	}

	// 追加聚合后的 LXC 节点（一个，汇总 used/avail）
	if lxcAggregated {
		ds = append(ds, ZFSDatasetInfo{
			Name:       lxcParent,
			Mountpoint: config.GlobalConfig.LXCLxcPath,
			Mounted:    true,
			Used:       lxcUsed,
			Avail:      lxcAvail,
		})
	}

	return ds
}

// ── ZFS 层级注入 ──

// injectZFSTree 将 ZFS 存储池/数据集层级信息注入存储池树，
// 并把成员盘容量清零以避免与合成 zpool 节点重复计入。
func injectZFSTree(pools []HostStoragePoolInfo, zPools []ZPoolInfo, mounts map[string]findmntInfo,
	dfUsage map[string]mountUsage, configs map[string]model.HostStoragePool) []HostStoragePoolInfo {

	// 收集成员盘路径 → pool 名映射
	memberToPool := make(map[string]string)
	for _, zp := range zPools {
		for _, m := range zp.Members {
			memberToPool[m] = zp.Name
		}
	}
	markZFSMemberNodes(pools, memberToPool)

	// 为每个 zpool 创建合成节点
	for _, zp := range zPools {
		zpoolNode := HostStoragePoolInfo{
			ID:                normalizeStorageDeviceID("zfs-" + zp.Name),
			Name:              zp.Name,
			DisplayName:       "ZFS: " + zp.Name,
			DevicePath:        "",
			Type:              "zpool",
			Size:              zp.Size,
			Available:         zp.Free,
			Used:              zp.Alloc,
			IsZFSPool:         true,
			ZFSVdevType:       zp.VdevType,
			ZFSExpandVdevType: zp.ExpandVdevType,
			Readonly:          true,
			CanFormat:         false,
			CanUseForVM:       false,
			StatusReason:      fmt.Sprintf("ZFS 存储池（%s，%s）", vdevTypeLabel(zp.VdevType), zp.Health),
		}
		if zp.Size > 0 {
			zpoolNode.UsePercent = int(float64(zp.Alloc) / float64(zp.Size) * 100)
		}

		// 数据集子节点
		for _, ds := range zp.Datasets {
			zpoolNode.Children = append(zpoolNode.Children, buildZFSDatasetNode(zp.Name, ds, configs))
		}
		// 成员盘引用子节点
		for _, m := range zp.Members {
			if ref := buildZFSMemberRefNode(zp.Name, m); ref != nil {
				zpoolNode.Children = append(zpoolNode.Children, *ref)
			}
		}

		pools = append(pools, zpoolNode)
	}

	return pools
}

// markZFSMemberNodes 标记属于 zpool 的成员盘节点：只读、容量清零、提示已加入 ZFS 存储池。
func markZFSMemberNodes(pools []HostStoragePoolInfo, memberToPool map[string]string) {
	for i := range pools {
		if poolName, ok := memberToPool[pools[i].DevicePath]; ok {
			pools[i].Readonly = true
			pools[i].CanFormat = false
			pools[i].CanUseForVM = false
			pools[i].StatusReason = "已加入 ZFS 存储池 " + poolName
			pools[i].Size = 0
			pools[i].Used = 0
			pools[i].Available = 0
			pools[i].UsePercent = 0
		}
		markZFSMemberNodes(pools[i].Children, memberToPool)
	}
}

// buildZFSDatasetNode 构建 ZFS 数据集节点（可作为 VM 落盘位置）。
func buildZFSDatasetNode(poolName string, ds ZFSDatasetInfo, configs map[string]model.HostStoragePool) HostStoragePoolInfo {
	comp := ds.ComponentName(poolName)
	id := normalizeStorageDeviceID("zfs-" + poolName + "-" + comp)
	cfg, configured := configs[id]

	mp := ds.Mountpoint
	if mp == "-" || mp == "none" || mp == "legacy" || mp == "" {
		mp = ""
	}

	node := HostStoragePoolInfo{
		ID:          id,
		Name:        comp,
		DisplayName: ds.Name,
		Type:        "zdataset",
		Size:        ds.Used + ds.Avail,
		Used:        ds.Used,
		Available:   ds.Avail,
		FSType:      "zfs",
		IsZFSPool:   true,
		Configured:  configured,
	}
	if configured {
		node.DisplayName = cfg.DisplayName
		node.Enabled = cfg.Enabled
		node.IsDefault = cfg.IsDefault
		if cfg.MountPath != "" {
			mp = cfg.MountPath
		}
	}
	if mp != "" {
		node.Mountpoints = []string{mp}
		node.MountPath = mp
		node.VMDir = mp // ZFS 数据集挂载点即 VM 磁盘目录
		node.CanUseForVM = true
	}
	if node.Size > 0 && node.Used > 0 {
		node.UsePercent = int(float64(node.Used) / float64(node.Size) * 100)
	}
	return node
}

// buildZFSMemberRefNode 构建 ZFS 成员盘引用节点（仅展示层级，不可操作）。
func buildZFSMemberRefNode(poolName, devPath string) *HostStoragePoolInfo {
	base := filepath.Base(devPath)
	return &HostStoragePoolInfo{
		ID:           normalizeStorageDeviceID("zfsmember-" + poolName + "-" + base),
		Name:         base,
		DisplayName:  devPath,
		DevicePath:   devPath,
		Type:         "zmember",
		Size:         0,
		Readonly:     true,
		StatusReason: "ZFS 存储池 " + poolName + " 成员盘",
	}
}

// ── ZFS 扩容：顶层 vdev 类型解析（按 tab 缩进）──

// parseZPoolTopVdevTypes 解析 `zpool status -L <pool>` 的 stdout，按缩进层级
// 取顶层 vdev（pool 行=1、顶层 vdev=2、成员盘=3，详见 statusIndentLevel），返回各顶层 vdev 类型。
// 纯函数（命令由 scanZPoolTopVdevTypes 执行），便于用样例输出单测。
func parseZPoolTopVdevTypes(statusOutput string) []string {
	inConfig := false
	var types []string
	for _, line := range strings.Split(statusOutput, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "config:") {
			inConfig = true
			continue
		}
		if strings.HasPrefix(t, "errors:") {
			break
		}
		if !inConfig {
			continue
		}
		if statusIndentLevel(line) != 2 { // 仅顶层 vdev（pool=1、顶层 vdev=2、成员盘=3）
			continue
		}
		f := strings.Fields(t)
		if len(f) == 0 {
			continue
		}
		types = append(types, classifyVdevName(f[0]))
	}
	return types
}

// statusIndentLevel 返回 zpool status 配置块某行的缩进层级。
// 实测 `zpool status -L` 用 1 个前导 tab + 每 2 空格一级表示层级：
// pool 行=1（\t）、顶层 vdev=2（\t+2 空格）、成员盘=3（\t+4 空格）。
// 为兼容个别纯 tab 缩进的实现，按 tab=1 级、每 2 空格=1 级折算。
func statusIndentLevel(line string) int {
	tabs, spaces := 0, 0
	for _, r := range line {
		switch r {
		case '\t':
			tabs++
		case ' ':
			spaces++
		default:
			return tabs + spaces/2
		}
	}
	return tabs + spaces/2
}

// classifyVdevName 按顶层 vdev 行首字段判定类型（裸设备/无前缀 → stripe）。
func classifyVdevName(name string) string {
	switch {
	case strings.HasPrefix(name, "mirror-"):
		return "mirror"
	case strings.HasPrefix(name, "raidz3-"):
		return "raidz3"
	case strings.HasPrefix(name, "raidz2-"):
		return "raidz2"
	case strings.HasPrefix(name, "raidz1-"):
		return "raidz1"
	default:
		return "stripe"
	}
}

// expandVdevType 由顶层 vdev 类型列表派生扩容锁定类型：全相同→该类型；空/混合→"mixed"。
func expandVdevType(types []string) string {
	if len(types) == 0 {
		return "mixed"
	}
	first := types[0]
	for _, t := range types[1:] {
		if t != first {
			return "mixed"
		}
	}
	return first
}

// scanZPoolTopVdevTypes 执行 zpool status -L 并解析顶层 vdev 类型（exec 包装，不单测）。
func scanZPoolTopVdevTypes(pool string) []string {
	r := utils.ExecCommandQuiet("zpool", "status", "-L", pool)
	if r.Error != nil {
		return nil
	}
	return parseZPoolTopVdevTypes(r.Stdout)
}

// buildZpoolAddArgs 拼装 zpool add 参数（纯函数，假定已校验）。
// add -f -o ashift=12 <pool> [vdevKeyword] <disks...>
func buildZpoolAddArgs(pool, vdevType string, devicePaths []string) []string {
	args := []string{"add", "-f", "-o", "ashift=12", pool}
	if kw := zfsVdevKeyword(vdevType); kw != "" { // stripe→""（裸盘直列）
		args = append(args, kw)
	}
	args = append(args, devicePaths...)
	return args
}

// AddZFSVdevs 给已存在的 zpool 加同类型顶层 vdev（扩容）。
// 纯类型池仅允许加同类；混合池放行任意。复用建池的磁盘校验与稳定路径。
func AddZFSVdevs(pool, vdevType string, deviceIDs []string) error {
	if !zpoolExists(pool) {
		return fmt.Errorf("ZFS 存储池 %s 不存在", pool)
	}
	vdev := normalizeZFSVdevType(vdevType)
	if len(deviceIDs) == 0 {
		return fmt.Errorf("至少需要选择一个物理磁盘")
	}
	minDisks := zfsVdevMinDisks(vdev)
	if len(deviceIDs) < minDisks {
		return fmt.Errorf("%s 至少需要 %d 块磁盘，当前选择 %d 块", vdevTypeLabel(vdev), minDisks, len(deviceIDs))
	}
	// 同类型强校验：纯类型池仅允许加同类
	cur := expandVdevType(scanZPoolTopVdevTypes(pool))
	if cur != "mixed" && cur != vdev {
		return fmt.Errorf("扩容须与现有 vdev 类型一致（%s）", vdevTypeLabel(cur))
	}
	// 复用建池的磁盘校验 + 稳定路径
	devicePaths, err := validateAndCollectPVTargets(deviceIDs)
	if err != nil {
		return fmt.Errorf("校验磁盘失败: %w", err)
	}
	devicePaths = resolveStableDevicePaths(devicePaths, readStableDeviceAliases())
	args := buildZpoolAddArgs(pool, vdev, devicePaths)
	ctx := context.Background()
	if r := utils.ExecCommandContextWithTimeout(ctx, "zpool", 2*time.Minute, args...); r.Error != nil {
		return fmt.Errorf("zpool add 失败: %s", strings.TrimSpace(r.Stderr))
	}
	return nil
}
