package template

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/model"
	archpkg "qvmhub/service/arch"
	"qvmhub/service/lxc/zfsbacking"
	"qvmhub/utils"
)

func sha256OfFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// FinalizeImport 由已落地的 tarball 创建金基底容器 + DB 行，并删除临时 tarball。
// progress 用于异步任务上报阶段进度（可为 nil，同步调用时传 nil）。
func FinalizeImport(params *ImportParams, progress func(int, string)) error {
	report := func(pct int, msg string) {
		if progress != nil {
			progress(pct, msg)
		}
	}
	if err := validateImportParams(params); err != nil {
		return err
	}
	if params.OwnerUsername == "" {
		params.OwnerUsername = "admin"
	}
	// 架构强制跟随宿主机：rootfs tarball 内容里没有可靠 arch，LXC 容器又跑在宿主机内核上，
	// 由宿主机决定（忽略前端传入值）。最终 writeBaseConfig 与 DB 行均用此值。
	hostArch, err := HostArchLXC()
	if err != nil {
		return err
	}
	params.Arch = hostArch
	backing := strings.TrimSpace(params.Backing)
	if backing == "" {
		backing = config.GlobalConfig.LXCDefaultBacking
	}
	// backing=zfs 时先验证 lxcpath 在 zfs 上：失败比慢速 tarball inspect 后才崩更友好。
	if backing == "zfs" {
		if _, err := zfsbacking.ResolveParent(config.GlobalConfig.LXCLxcPath); err != nil {
			return fmt.Errorf("backing=zfs 校验失败（lxc 目录不在 zfs 上？改用 dir，或把 lxc 目录迁到 zfs）: %w", err)
		}
	}
	base := baseContainerName(params.Name)

	// 校验 tarball（结构 + os-release；sha/size 由其返回）
	report(10, "校验 tar 包")
	info, err := InspectRootfsTarball(params.SourcePath)
	if err != nil {
		return err
	}

	// 名称占用检查
	var cnt int64
	model.DB.Model(&model.LXCTemplate{}).Where("name = ?", params.Name).Count(&cnt)
	if cnt > 0 {
		return fmt.Errorf("模板 %s 已存在", params.Name)
	}
	// 基底容器是否已存在（lxc-create 会失败）
	if existsContainer(base) {
		return fmt.Errorf("基底容器 %s 已存在", base)
	}

	// 创建基底 + 解包 + 写 config（按 backing 分支）。zfs：zfs create 一个 dataset；dir/overlay：lxc-create -t none。
	// 两者 rootfs.path 都是普通目录 <lxcpath>/<base>/rootfs，故 config 写入共用 writeBaseConfig。
	rootfs := filepath.Join(config.GlobalConfig.LXCLxcPath, base, "rootfs")
	report(30, "创建基底容器（"+backing+"）")
	if backing == "zfs" {
		parent, err := zfsbacking.ResolveParent(config.GlobalConfig.LXCLxcPath)
		if err != nil {
			return err
		}
		if err := zfsbacking.CreateBase(parent, base); err != nil {
			return err
		}
		if err := extractRootfsInto(rootfs, params.SourcePath, info.RootfsMember, report); err != nil {
			_ = zfsbacking.DestroyBase(parent, base)
			return err
		}
		if err := writeBaseConfig(base, params.Arch); err != nil {
			_ = zfsbacking.DestroyBase(parent, base)
			return err
		}
		if err := zfsbacking.SnapshotBase(parent, base); err != nil {
			_ = zfsbacking.DestroyBase(parent, base)
			return err
		}
	} else {
		cre := utils.ExecCommandLongRunning("lxc-create", "-n", base, "-t", "none", "-B", backing)
		if cre.Error != nil {
			return fmt.Errorf("创建基底容器失败: %w", cre.Error)
		}
		if err := extractRootfsInto(rootfs, params.SourcePath, info.RootfsMember, report); err != nil {
			_ = destroyContainerQuiet(base)
			return err
		}
		if err := writeBaseConfig(base, params.Arch); err != nil {
			_ = destroyContainerQuiet(base)
			return err
		}
	}

	// 写 DB 行
	report(85, "写入模板记录")
	tpl := model.LXCTemplate{
		Name:              params.Name,
		DisplayName:       orDefault(params.DisplayName, params.Name),
		Distro:            params.Distro,
		Release:           params.Release,
		Arch:              orDefault(params.Arch, "amd64"),
		Description:       params.Description,
		BaseContainerName: base,
		Backing:           backing,
		RootfsSizeBytes:   rootfsSizeBytes(base), // 解包后算 rootfs 表观大小，与「从容器制作」同口径（du -sb）
		CloneVisible:      true,
		OwnerUsername:     params.OwnerUsername,
		PostCreateCommand: params.PostCreateCommand,
		SHA256:            info.SHA256,
	}
	if err := model.DB.Create(&tpl).Error; err != nil {
		_ = destroyBase(base, backing)
		return fmt.Errorf("保存模板记录失败: %w", err)
	}

	// 删除临时 tarball（基底 rootfs 即唯一源）
	if params.SourcePath != "" && isInDir(params.SourcePath, config.GlobalConfig.LXCTemplateImportDir) {
		if err := os.Remove(params.SourcePath); err != nil {
			logger.App.Warn("删除临时 tarball 失败", "path", params.SourcePath, "error", err)
		}
	}
	logger.App.Info("LXC 模板导入完成", "name", params.Name, "base", base)
	report(100, "导入完成")
	return nil
}

func writeBaseConfig(base, arch string) error {
	cfg := filepath.Join(config.GlobalConfig.LXCLxcPath, base, "config")
	// 读 lxc-create 生成的 config，去掉默认 net 块与我们将覆盖的标量键（避免重复），
	// 保留 lxc.uts.name / lxc.include / apparmor 等安全关键键，再写入我们的覆盖项。
	data, err := os.ReadFile(cfg)
	if err != nil {
		// zfs backing 无 lxc-create 生成的 config（zfs create 不写 config）→ 用最小基底前缀从头写；
		// dir/overlay 由 lxc-create 生成 config，正常读取后去重。
		if !os.IsNotExist(err) {
			return fmt.Errorf("读取基底 config 失败: %w", err)
		}
		data = []byte("lxc.uts.name = " + base + "\nlxc.apparmor.profile = generated\nlxc.apparmor.allow_nesting = 1\n")
	}
	// 权威写入 lxc.rootfs.path：lxc-create -t none 不设置 rootfs（"none" 即无模板/无 rootfs），
	// 不补这行的话后续 lxc-copy 会「No rootfs specified」失败。import 把 rootfs 解到
	// <lxcpath>/<base>/rootfs，这里以 last-wins 追加正确路径（覆盖 lxc-create 可能写入的错误值）。
	rootfsLine := "lxc.rootfs.path = " + filepath.Join(config.GlobalConfig.LXCLxcPath, base, "rootfs") + "\n"
	return os.WriteFile(cfg, []byte(composeBaseConfig(string(data), arch, base)+rootfsLine), 0644)
}

// extractRootfsInto 把 tarball 解到 rootfs（dir 普通目录 / zfs dataset 内子目录都一样）。
func extractRootfsInto(rootfs, src, member string, report func(int, string)) error {
	if err := os.MkdirAll(rootfs, 0755); err != nil {
		return fmt.Errorf("创建 rootfs 目录失败: %w", err)
	}
	strip := strings.Count(member, "/") + 1
	report(50, "解包 rootfs（大包较慢，请稍候）")
	ex := utils.ExecCommandLongRunning("tar", "-xf", src, "-C", rootfs, "--strip-components", strconv.Itoa(strip), member)
	if ex.Error != nil {
		return fmt.Errorf("解包 rootfs 失败: %w", ex.Error)
	}
	return nil
}

// destroyBase 按 backing 销毁基底：zfs → DestroyBase（含 @base 快照）；dir/overlay → destroyContainerQuiet。
func destroyBase(base, backing string) error {
	if backing == "zfs" {
		parent, err := zfsbacking.ResolveParent(config.GlobalConfig.LXCLxcPath)
		if err != nil {
			return err
		}
		return zfsbacking.DestroyBase(parent, base)
	}
	return destroyContainerQuiet(base)
}

// composeBaseConfig 纯函数：把已有的 existing 内容去重后追加我们的覆盖项。
// 去重规则：丢弃所有 lxc.net.* 与将被覆盖的标量键（arch/cgroup2.cpu.weight/
// cgroup2.memory.max/start.auto/uts.name）及空行/注释；保留其余键（rootfs.path/include/apparmor…）。
// utsName 作为权威 lxc.uts.name 写入：对 import（lxc-create 已写 base）是 no-op，
// 对「从容器制作模板」则把源主机名重置为基底名。
func composeBaseConfig(existing, arch, utsName string) string {
	if arch == "" {
		arch = "amd64"
	}
	lxcArch := "x86_64"
	if arch == "arm64" {
		lxcArch = "aarch64"
	}
	if strings.TrimSpace(utsName) == "" {
		utsName = "lxc-base"
	}
	overriddenExact := map[string]bool{
		"lxc.arch":               true,
		"lxc.cgroup2.cpu.weight": true,
		"lxc.cgroup2.memory.max": true,
		"lxc.start.auto":         true,
		"lxc.uts.name":           true,
	}
	var b strings.Builder
	for _, line := range strings.Split(existing, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		key := trim
		if eq := strings.IndexByte(trim, '='); eq > 0 {
			key = strings.TrimSpace(trim[:eq])
		}
		// 去掉默认 net 块（lxc.net.*）与我们将覆盖的标量键（含 uts.name），避免重复键
		if overriddenExact[key] || strings.HasPrefix(key, "lxc.net.") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	for _, l := range []string{
		"lxc.uts.name = " + utsName,
		"lxc.arch = " + lxcArch,
		"lxc.cgroup2.cpu.weight = 256",
		"lxc.cgroup2.memory.max = 512M",
		"lxc.start.auto = 0",
		"lxc.net.0.type = veth",
		"lxc.net.0.flags = up",
		"lxc.net.0.link = br-ovs",
	} {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}

// ---- helpers ----

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// mapArchToLXC 把 arch.DetectHostArch()（uname -m 规范化值）映射为 LXC 模板用的 amd64/arm64。
// riscv64 等当前 LXC 模板不支持，返回错误（与 validateImportParams 的 amd64/arm64 白名单一致）。
func mapArchToLXC(a string) (string, error) {
	switch a {
	case archpkg.ArchX8664:
		return "amd64", nil
	case archpkg.ArchAarch64:
		return "arm64", nil
	default:
		return "", fmt.Errorf("宿主机架构 %s 暂不支持 LXC 模板（仅 amd64/arm64）", a)
	}
}

// HostArchLXC 返回宿主机架构对应的 LXC arch（amd64/arm64），供前端展示与 finalize 落库。
func HostArchLXC() (string, error) {
	return mapArchToLXC(archpkg.DetectHostArch())
}

func existsContainer(name string) bool {
	res := utils.ExecCommandQuiet("lxc-info", "-n", name)
	return res.ExitCode == 0
}

func destroyContainerQuiet(name string) error {
	res := utils.ExecCommandQuiet("lxc-destroy", "-n", name)
	return res.Error
}

func isInDir(path, dir string) bool {
	abs, _ := filepath.Abs(path)
	d, _ := filepath.Abs(dir)
	return strings.HasPrefix(abs+string(filepath.Separator), d+string(filepath.Separator))
}

// RootfsInfo 是对 rootfs tarball 校验/探测的结果。
type RootfsInfo struct {
	SHA256       string
	SizeBytes    int64
	Distro       string // 来自 os-release 的 ID（best-effort）
	Release      string // 来自 os-release 的 VERSION_ID（best-effort）
	RootfsMember string // 顶层 rootfs 目录在归档里的【原始】成员名（rootfs 或 ./rootfs），FinalizeImport 据此解包
}

// InspectRootfsTarball 校验 tarball（按内容 auto-detect 格式）顶层含 rootfs/ 目录、
// 含 rootfs/etc/os-release，并解析 os-release 的 ID/VERSION_ID，返回 sha256 与大小。
func InspectRootfsTarball(path string) (*RootfsInfo, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("读取 tarball 失败: %w", err)
	}
	if st.IsDir() {
		return nil, fmt.Errorf("模板源必须是文件而非目录")
	}
	// 列条目（-t 不带压缩标志，GNU tar 按内容自动识别 gzip/xz/bzip2/zstd）
	listRes := utils.ExecCommand("tar", "-tf", abs)
	if listRes.Error != nil {
		return nil, fmt.Errorf("非有效 tar 包或格式不支持: %w", listRes.Error)
	}
	rawListing := listRes.Stdout
	if !listingHasTopLevelRootfs(rawListing) {
		return nil, fmt.Errorf("压缩包顶层未找到 rootfs 目录")
	}
	// 顶层 rootfs 校验已通过，findMember 必然命中；保留原始成员名供 finalize 解包用。
	// 去掉目录条目的尾随 '/'（tar -t 常把目录列为 "rootfs/"），使后续 strip 推导与解包选择器稳定。
	rootfsMember, ok := findMember(rawListing, "rootfs")
	if !ok {
		return nil, fmt.Errorf("压缩包顶层未找到 rootfs 目录")
	}
	rootfsMember = strings.TrimSuffix(rootfsMember, "/")
	if _, ok := findMember(rawListing, "rootfs/etc/os-release"); !ok {
		return nil, fmt.Errorf("rootfs 下缺少 etc/os-release，无法判定为合法 rootfs")
	}
	// 读 os-release 内容（处理符号链接回退，见 readOSRelease）。
	distro, release := readOSRelease(abs, rawListing)
	sha, err := sha256OfFile(abs)
	if err != nil {
		return nil, err
	}
	return &RootfsInfo{SHA256: sha, SizeBytes: st.Size(), Distro: distro, Release: release, RootfsMember: rootfsMember}, nil
}

// readOSRelease 从归档读取 os-release 文本并解析。优先 rootfs/etc/os-release；
// 不少发行版（Debian 等）的 /etc/os-release 是指向 /usr/lib/os-release 的符号链接，
// 而 tar -O 对符号链接成员输出为空（实测），故空则回退到 rootfs/usr/lib/os-release。
// 两者都取不到（或无 ID/VERSION_ID）时返回空串——属 best-effort，不影响校验通过。
func readOSRelease(abs, listing string) (distro, release string) {
	for _, target := range []string{"rootfs/etc/os-release", "rootfs/usr/lib/os-release"} {
		member, ok := findMember(listing, target)
		if !ok {
			continue
		}
		res := utils.ExecCommand("tar", "-xf", abs, "-O", member)
		if res.Error != nil || strings.TrimSpace(res.Stdout) == "" {
			continue // 符号链接成员无 stdout 内容，或读取出错——试下一个候选
		}
		if d, r := parseOSRelease(res.Stdout); d != "" || r != "" {
			return d, r
		}
	}
	return "", ""
}

// listingHasTopLevelRootfs 判断 tar -t 输出里是否存在顶层 rootfs 目录
// （原始条目为 rootfs、rootfs/、./rootfs、./rootfs/ ，或以其为前缀的子路径）。
func listingHasTopLevelRootfs(listing string) bool {
	for _, line := range strings.Split(listing, "\n") {
		e := strings.TrimSpace(line)
		if e == "" {
			continue
		}
		e = strings.TrimPrefix(e, "./")
		e = strings.TrimSuffix(e, "/")
		if e == "rootfs" || strings.HasPrefix(e, "rootfs/") {
			return true
		}
	}
	return false
}

// findMember 在 tar -t 原始输出里找规范化后等于 target 的成员，返回其原始行。
// 用原始行（而非规范化值）传给 tar -O，以兼容 ./rootfs/... 形式的存储名。
func findMember(listing, target string) (string, bool) {
	for _, line := range strings.Split(listing, "\n") {
		raw := strings.TrimSpace(line)
		if raw == "" {
			continue
		}
		norm := strings.TrimSuffix(strings.TrimPrefix(raw, "./"), "/")
		if norm == target {
			return raw, true
		}
	}
	return "", false
}

// parseOSRelease 解析 os-release 文本，取 ID 与 VERSION_ID（去引号与首尾空白）。缺失返回空串。
func parseOSRelease(content string) (distro, release string) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.Trim(strings.TrimSpace(line[eq+1:]), `"'`)
		switch key {
		case "ID":
			distro = val
		case "VERSION_ID":
			release = val
		}
	}
	return distro, release
}

// extractMemberOnce 用 tar -xf -O --occurrence=1 取首个匹配成员的内容（命中即停，
// 不遍历整包，对大 rootfs 友好）。成员不存在时 ok=false。
// 兼容 rootfs 与 ./rootfs 形态由调用方分别尝试两个成员名实现。
func extractMemberOnce(abs, member string) (content string, ok bool) {
	res := utils.ExecCommand("tar", "-xf", abs, "-O", "--occurrence=1", member)
	if res.Error != nil {
		return "", false
	}
	return res.Stdout, true
}

// ProbeRootfsTarball 快速探测 rootfs tarball：仅确认 rootfs/etc/os-release 存在并读其内容，
// 解析 distro/release，返回文件大小。不算 sha256、不做全量 tar -tf（那些由 finalize 在
// 异步任务里做），保证大包也秒级返回。/etc/os-release 为符号链接时回退 rootfs/usr/lib/os-release。
func ProbeRootfsTarball(path string) (distro, release string, size int64, err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", 0, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", "", 0, fmt.Errorf("读取 tarball 失败: %w", err)
	}
	if st.IsDir() {
		return "", "", 0, fmt.Errorf("模板源必须是文件而非目录")
	}
	// 探测 rootfs/etc/os-release（兼容 ./rootfs 存储形态）
	content, ok := extractMemberOnce(abs, "rootfs/etc/os-release")
	if !ok {
		content, ok = extractMemberOnce(abs, "./rootfs/etc/os-release")
	}
	if !ok {
		return "", "", 0, fmt.Errorf("rootfs 下缺少 etc/os-release，无法判定为合法 rootfs")
	}
	// 符号链接（/etc/os-release → /usr/lib/os-release）时 tar -O 输出空 → 回退 usr/lib
	if strings.TrimSpace(content) == "" {
		for _, m := range []string{"rootfs/usr/lib/os-release", "./rootfs/usr/lib/os-release"} {
			if c, ok2 := extractMemberOnce(abs, m); ok2 && strings.TrimSpace(c) != "" {
				content = c
				break
			}
		}
	}
	d, r := parseOSRelease(content)
	return d, r, st.Size(), nil
}
