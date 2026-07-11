package host

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"qvmhub/logger"
	"qvmhub/utils"
)

const (
	hostZRAMManagedHeader = "# 由 kvm_console 管理：宿主机 zRAM 配置\n"
	hostZRAMUnitName      = "kvm-console-zram.service"
	hostZRAMLabel         = "kvm-zram"
)

var hostZRAMManagedLabels = []string{
	hostZRAMLabel,
	"kvm-console-zram",
	"kvm-console-zra",
}

var (
	hostZRAMConfigPath          = "/etc/kvm-console/zram.env"
	hostZRAMLegacyScript        = "/etc/kvm-console/host-zram.sh"
	hostZRAMRunDevicePath       = "/run/kvm-console/zram.device"
	hostZRAMUnitPath            = "/etc/systemd/system/kvm-console-zram.service"
	hostZRAMPanelUnitPaths      = []string{"/etc/systemd/system/kvm-console.service", "/lib/systemd/system/kvm-console.service", "/usr/lib/systemd/system/kvm-console.service"}
	hostZRAMExecutableFallbacks = []string{"/opt/kvm-console/kvm-console", "/usr/local/bin/kvm-console"}
)

var hostZRAMProfiles = []HostZRAMProfile{
	{
		Key:         "off",
		Name:        "关闭",
		Description: "关闭面板管理的 zRAM swap，适合排障或宿主机内存压力很低的场景。",
		SizePercent: 0,
		MaxSizeMB:   0,
		Algorithm:   "lz4",
		Priority:    0,
	},
	{
		Key:         "conservative",
		Name:        "保守",
		Description: "zRAM 逻辑容量为宿主机内存 10%，最高 16 GiB，优先降低压缩和换页开销。",
		SizePercent: 10,
		MaxSizeMB:   16 * 1024,
		Algorithm:   "lz4",
		Priority:    60,
	},
	{
		Key:         "balanced",
		Name:        "均衡",
		Description: "zRAM 逻辑容量为宿主机内存 20%，最高 32 GiB，适合作为纯虚拟化宿主机默认挡位。",
		SizePercent: 20,
		MaxSizeMB:   32 * 1024,
		Algorithm:   "lz4",
		Priority:    80,
	},
	{
		Key:         "aggressive",
		Name:        "积极",
		Description: "zRAM 逻辑容量为宿主机内存 35%，最高 64 GiB，适合 VM 密度高且希望优先压缩内存的宿主机。",
		SizePercent: 35,
		MaxSizeMB:   64 * 1024,
		Algorithm:   "lz4",
		Priority:    100,
	},
	{
		Key:         "extreme",
		Name:        "极致",
		Description: "zRAM 逻辑容量为宿主机内存 50%，最高 128 GiB，适合内存非常紧张且能接受更多 CPU 开销的宿主机。",
		SizePercent: 50,
		MaxSizeMB:   128 * 1024,
		Algorithm:   "lz4",
		Priority:    120,
	},
}

func GetHostZRAMProfiles() []HostZRAMProfile {
	profiles := make([]HostZRAMProfile, len(hostZRAMProfiles))
	copy(profiles, hostZRAMProfiles)
	return profiles
}

func findHostZRAMProfile(key string) (HostZRAMProfile, bool) {
	normalized := strings.TrimSpace(strings.ToLower(key))
	for _, profile := range hostZRAMProfiles {
		if profile.Key == normalized {
			return profile, true
		}
	}
	return HostZRAMProfile{}, false
}

func commandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func hostZRAMSupported() bool {
	for _, name := range []string{"zramctl", "mkswap", "swapon", "swapoff", "blkid"} {
		if !commandAvailable(name) {
			return false
		}
	}
	if _, err := os.Stat("/sys/class/zram-control"); err == nil {
		return true
	}
	if _, err := os.Stat("/sys/module/zram"); err == nil {
		return true
	}
	if commandAvailable("modprobe") {
		result := utils.ExecCommandWithTimeout("modprobe", time.Second*5, "-n", "zram")
		return result.Error == nil
	}
	return false
}

func readHostZRAMInt64(path string) *int64 {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	value, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		return nil
	}
	return &value
}

func bytesToMB(value *int64) *int64 {
	if value == nil {
		return nil
	}
	mb := *value / 1024 / 1024
	return &mb
}

func readHostZRAMAlgorithm(name string) string {
	content, err := os.ReadFile(filepath.Join("/sys/block", name, "comp_algorithm"))
	if err != nil {
		return ""
	}
	for _, item := range strings.Fields(string(content)) {
		if strings.HasPrefix(item, "[") && strings.HasSuffix(item, "]") {
			return strings.Trim(item, "[]")
		}
	}
	return ""
}

func readHostZRAMMMStat(name string) (origDataSize, comprDataSize, memUsedTotal *int64) {
	content, err := os.ReadFile(filepath.Join("/sys/block", name, "mm_stat"))
	if err != nil {
		return nil, nil, nil
	}
	fields := strings.Fields(string(content))
	if len(fields) < 3 {
		return nil, nil, nil
	}
	parse := func(index int) *int64 {
		value, err := strconv.ParseInt(fields[index], 10, 64)
		if err != nil {
			return nil
		}
		return &value
	}
	return parse(0), parse(1), parse(2)
}

func readHostZRAMSwapPriority(device string) *int {
	content, err := os.ReadFile("/proc/swaps")
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] != device {
			continue
		}
		value, err := strconv.Atoi(fields[4])
		if err != nil {
			return nil
		}
		return &value
	}
	return nil
}

func hostZRAMDeviceHasManagedLabel(device string) bool {
	if !commandAvailable("blkid") {
		return false
	}
	result := utils.ExecCommandWithTimeout("blkid", time.Second*5, "-o", "value", "-s", "LABEL", device)
	if result.Error != nil {
		return false
	}
	label := strings.TrimSpace(result.Stdout)
	for _, managedLabel := range hostZRAMManagedLabels {
		if label == managedLabel {
			return true
		}
	}
	return false
}

func readHostZRAMRunDevice() string {
	content, err := os.ReadFile(hostZRAMRunDevicePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

func writeHostZRAMRunDevice(device string) error {
	if err := os.MkdirAll(filepath.Dir(hostZRAMRunDevicePath), 0755); err != nil {
		return fmt.Errorf("创建 zRAM 运行目录失败: %w", err)
	}
	if err := os.WriteFile(hostZRAMRunDevicePath, []byte(strings.TrimSpace(device)+"\n"), 0644); err != nil {
		return fmt.Errorf("写入 zRAM 运行设备记录失败: %w", err)
	}
	return nil
}

func removeHostZRAMRunDevice() {
	_ = os.Remove(hostZRAMRunDevicePath)
}

func hostZRAMDeviceIsManaged(device string) bool {
	if device == "" || !strings.HasPrefix(filepath.Base(device), "zram") {
		return false
	}
	if _, err := os.Stat(filepath.Join("/sys/block", filepath.Base(device))); err != nil {
		return false
	}
	return hostZRAMDeviceHasManagedLabel(device) || readHostZRAMRunDevice() == device
}

func hostZRAMSwapActive(device string) bool {
	content, err := os.ReadFile("/proc/swaps")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 1 && fields[0] == device {
			return true
		}
	}
	return false
}

func listHostZRAMSwapDevices() []string {
	content, err := os.ReadFile("/proc/swaps")
	if err != nil {
		return nil
	}
	devices := []string{}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || !strings.HasPrefix(filepath.Base(fields[0]), "zram") {
			continue
		}
		if hostZRAMDeviceIsManaged(fields[0]) {
			devices = append(devices, fields[0])
		}
	}
	return devices
}

func readHostZRAMRuntimeConfig() HostZRAMRuntimeConfig {
	devices := listHostZRAMSwapDevices()
	if len(devices) == 0 {
		return HostZRAMRuntimeConfig{}
	}
	device := devices[0]
	name := filepath.Base(device)
	sizeBytes := readHostZRAMInt64(filepath.Join("/sys/block", name, "disksize"))
	origDataSize, comprDataSize, memUsedTotal := readHostZRAMMMStat(name)
	return HostZRAMRuntimeConfig{
		Device:          device,
		SizeBytes:       sizeBytes,
		SizeMB:          bytesToMB(sizeBytes),
		UsedBytes:       memUsedTotal,
		UsedMB:          bytesToMB(memUsedTotal),
		OriginalBytes:   origDataSize,
		CompressedBytes: comprDataSize,
		Algorithm:       readHostZRAMAlgorithm(name),
		Priority:        readHostZRAMSwapPriority(device),
	}
}

func readHostMemTotalBytes() int64 {
	content, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			value, err := strconv.ParseInt(fields[1], 10, 64)
			if err == nil {
				return value * 1024
			}
		}
	}
	return 0
}

func expectedHostZRAMSizeBytes(profile HostZRAMProfile) int64 {
	if profile.SizePercent <= 0 {
		return 0
	}
	size := readHostMemTotalBytes() * int64(profile.SizePercent) / 100
	if profile.MaxSizeMB > 0 {
		capBytes := int64(profile.MaxSizeMB) * 1024 * 1024
		if size > capBytes {
			size = capBytes
		}
	}
	return size
}

func hostZRAMConfigMatchesProfile(config HostZRAMRuntimeConfig, profile HostZRAMProfile) bool {
	if profile.Key == "off" {
		return config.Device == ""
	}
	if config.Device == "" || config.SizeBytes == nil {
		return false
	}
	expectedSize := expectedHostZRAMSizeBytes(profile)
	sizeDelta := *config.SizeBytes - expectedSize
	if sizeDelta < 0 {
		sizeDelta = -sizeDelta
	}
	if sizeDelta > 8*1024*1024 {
		return false
	}
	if config.Algorithm != "" && config.Algorithm != profile.Algorithm {
		return false
	}
	if config.Priority != nil && *config.Priority != profile.Priority {
		return false
	}
	return true
}

func detectHostZRAMProfile(config HostZRAMRuntimeConfig) string {
	if config.Device == "" {
		return "off"
	}
	for _, profile := range hostZRAMProfiles {
		if hostZRAMConfigMatchesProfile(config, profile) {
			return profile.Key
		}
	}
	return "custom"
}

func readHostZRAMPersistentConfig() (string, *HostZRAMPersistentConfig, bool) {
	content, err := os.ReadFile(hostZRAMConfigPath)
	if err != nil {
		return "", nil, false
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	profileKey := strings.TrimSpace(values["ZRAM_PROFILE"])
	if profileKey == "" {
		return "", nil, false
	}
	config := &HostZRAMPersistentConfig{
		Profile:     profileKey,
		SizePercent: atoiDefault(values["ZRAM_SIZE_PERCENT"], 0),
		MaxSizeMB:   atoiDefault(values["ZRAM_MAX_SIZE_MB"], 0),
		Algorithm:   strings.TrimSpace(values["ZRAM_ALGORITHM"]),
		Priority:    atoiDefault(values["ZRAM_PRIORITY"], 0),
	}
	return profileKey, config, true
}

func atoiDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func buildHostZRAMEnv(profile HostZRAMProfile) string {
	return fmt.Sprintf(`%sZRAM_PROFILE=%s
ZRAM_SIZE_PERCENT=%d
ZRAM_MAX_SIZE_MB=%d
ZRAM_ALGORITHM=%s
ZRAM_PRIORITY=%d
ZRAM_LABEL=%s
`,
		hostZRAMManagedHeader,
		profile.Key,
		profile.SizePercent,
		profile.MaxSizeMB,
		profile.Algorithm,
		profile.Priority,
		hostZRAMLabel,
	)
}

func buildHostZRAMUnit() string {
	executablePath := resolveHostZRAMExecutablePath()
	return fmt.Sprintf(`[Unit]
Description=Apply kvm_console zRAM settings
After=sysinit.target local-fs.target
ConditionPathExists=/etc/kvm-console/zram.env

[Service]
Type=oneshot
EnvironmentFile=/etc/kvm-console/zram.env
ExecStart=%s host-zram-apply
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
`, executablePath)
}

func resolveHostZRAMExecutablePath() string {
	if configured := strings.TrimSpace(os.Getenv("KVM_CONSOLE_BINARY")); configured != "" && fileExecutable(configured) {
		return configured
	}
	for _, unitPath := range hostZRAMPanelUnitPaths {
		content, err := os.ReadFile(unitPath)
		if err != nil {
			continue
		}
		if execPath := parseSystemdExecStartPath(string(content)); execPath != "" && fileExecutable(execPath) {
			return execPath
		}
	}
	if executablePath, err := os.Executable(); err == nil && strings.TrimSpace(executablePath) != "" && fileExecutable(executablePath) && !looksTemporaryExecutablePath(executablePath) {
		return executablePath
	}
	for _, fallback := range hostZRAMExecutableFallbacks {
		if fileExecutable(fallback) {
			return fallback
		}
	}
	if len(hostZRAMExecutableFallbacks) > 0 {
		return hostZRAMExecutableFallbacks[0]
	}
	return "/opt/kvm-console/kvm-console"
}

func parseSystemdExecStartPath(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.HasPrefix(line, "ExecStart=") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "ExecStart="))
		value = strings.TrimPrefix(value, "-")
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "\"") {
			if end := strings.Index(value[1:], "\""); end >= 0 {
				return value[1 : end+1]
			}
		}
		return strings.Fields(value)[0]
	}
	return ""
}

func looksTemporaryExecutablePath(path string) bool {
	normalized := filepath.ToSlash(strings.TrimSpace(path))
	return strings.Contains(normalized, "/tmp/") || strings.Contains(normalized, "/temp/") || strings.Contains(normalized, "/server/tmp/")
}

func fileExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0111 != 0
}

func persistHostZRAMProfile(profile HostZRAMProfile) error {
	if err := os.MkdirAll(filepath.Dir(hostZRAMConfigPath), 0755); err != nil {
		return fmt.Errorf("创建 zRAM 配置目录失败: %w", err)
	}
	if err := os.WriteFile(hostZRAMConfigPath, []byte(buildHostZRAMEnv(profile)), 0644); err != nil {
		return fmt.Errorf("写入 zRAM 配置失败: %w", err)
	}
	_ = os.Remove(hostZRAMLegacyScript)
	if err := os.MkdirAll(filepath.Dir(hostZRAMUnitPath), 0755); err != nil {
		return fmt.Errorf("创建 systemd 配置目录失败: %w", err)
	}
	if err := os.WriteFile(hostZRAMUnitPath, []byte(buildHostZRAMUnit()), 0644); err != nil {
		return fmt.Errorf("写入 zRAM systemd 单元失败: %w", err)
	}
	if result := utils.ExecCommand("systemctl", "daemon-reload"); result.Error != nil {
		return fmt.Errorf("刷新 systemd 配置失败: %s", strings.TrimSpace(result.Stderr))
	}
	if profile.Key == "off" {
		if result := utils.ExecCommand("systemctl", "disable", "--now", hostZRAMUnitName); result.Error != nil {
			return fmt.Errorf("禁用 zRAM 持久化服务失败: %s", strings.TrimSpace(result.Stderr))
		}
		utils.ExecCommand("systemctl", "reset-failed", hostZRAMUnitName)
		return nil
	}
	if result := utils.ExecCommand("systemctl", "enable", hostZRAMUnitName); result.Error != nil {
		return fmt.Errorf("启用 zRAM 持久化服务失败: %s", strings.TrimSpace(result.Stderr))
	}
	return nil
}

func listHostZRAMBlockDevices() []string {
	matches, err := filepath.Glob("/sys/block/zram*")
	if err != nil {
		return nil
	}
	devices := make([]string, 0, len(matches)+1)
	seen := map[string]bool{}
	add := func(device string) {
		if device == "" || seen[device] {
			return
		}
		seen[device] = true
		devices = append(devices, device)
	}
	for _, match := range matches {
		add("/dev/" + filepath.Base(match))
	}
	add(readHostZRAMRunDevice())
	return devices
}

func ensureHostZRAMModule() error {
	if _, err := os.Stat("/sys/class/zram-control"); err == nil {
		return nil
	}
	if _, err := os.Stat("/sys/module/zram"); err == nil {
		return nil
	}
	if !commandAvailable("modprobe") {
		return fmt.Errorf("zRAM 模块未加载，且系统缺少 modprobe")
	}
	result := utils.ExecCommandWithTimeout("modprobe", time.Second*10, "zram")
	if result.Error != nil {
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = result.Error.Error()
		}
		return fmt.Errorf("加载 zRAM 模块失败: %s", detail)
	}
	return nil
}

func writeHostZRAMSysValue(name, fileName, value string) error {
	path := filepath.Join("/sys/block", name, fileName)
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		return fmt.Errorf("写入 zRAM 参数 %s 失败: %w", fileName, err)
	}
	return nil
}

func resetHostZRAMDevice(device string) error {
	name := filepath.Base(device)
	for attempt := 0; attempt < 10; attempt++ {
		if result := utils.ExecCommandWithTimeout("zramctl", time.Second*5, "--reset", device); result.Error == nil {
			return nil
		}
		if err := os.WriteFile(filepath.Join("/sys/block", name, "reset"), []byte("1"), 0644); err == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	if hostZRAMSwapActive(device) {
		return fmt.Errorf("zRAM 设备 %s 仍在 swap 中，无法重置", device)
	}
	logger.App.Warn("zRAM swap 已关闭，但设备暂时 busy，稍后会继续清理", "device", device)
	return nil
}

func stopHostZRAMManagedDevices() error {
	for _, device := range listHostZRAMSwapDevices() {
		result := utils.ExecCommandWithTimeout("swapoff", time.Minute, device)
		if result.Error != nil {
			detail := strings.TrimSpace(result.Stderr)
			if detail == "" {
				detail = result.Error.Error()
			}
			return fmt.Errorf("关闭 zRAM swap %s 失败: %s", device, detail)
		}
	}

	for _, device := range listHostZRAMBlockDevices() {
		if !hostZRAMDeviceIsManaged(device) {
			continue
		}
		if err := resetHostZRAMDevice(device); err != nil {
			return err
		}
	}
	removeHostZRAMRunDevice()
	return nil
}

func findFreeHostZRAMDevice() (string, error) {
	result := utils.ExecCommandWithTimeout("zramctl", time.Second*5, "--find")
	device := strings.TrimSpace(result.Stdout)
	if result.Error == nil && device != "" {
		if _, err := os.Stat(filepath.Join("/sys/block", filepath.Base(device))); err == nil {
			return device, nil
		}
	}
	if content, err := os.ReadFile("/sys/class/zram-control/hot_add"); err == nil {
		index := strings.TrimSpace(string(content))
		if index != "" {
			device = "/dev/zram" + index
			if _, err := os.Stat(filepath.Join("/sys/block", filepath.Base(device))); err == nil {
				return device, nil
			}
		}
	}
	return "", fmt.Errorf("未找到可用 zRAM 设备")
}

func hostZRAMAlgorithmSupported(name, algorithm string) bool {
	content, err := os.ReadFile(filepath.Join("/sys/block", name, "comp_algorithm"))
	if err != nil {
		return false
	}
	for _, item := range strings.Fields(string(content)) {
		if strings.Trim(item, "[]") == algorithm {
			return true
		}
	}
	return false
}

func chooseHostZRAMAlgorithm(name, preferred string) string {
	for _, algorithm := range []string{preferred, "lz4", "lzo-rle", "lzo"} {
		algorithm = strings.TrimSpace(algorithm)
		if algorithm != "" && hostZRAMAlgorithmSupported(name, algorithm) {
			return algorithm
		}
	}
	return ""
}

func applyHostZRAMRuntime(profile HostZRAMProfile) error {
	if err := ensureHostZRAMModule(); err != nil {
		return err
	}
	if err := stopHostZRAMManagedDevices(); err != nil {
		return err
	}
	if profile.Key == "off" {
		return nil
	}

	sizeBytes := expectedHostZRAMSizeBytes(profile)
	if sizeBytes <= 0 {
		return fmt.Errorf("zRAM 容量计算结果无效")
	}
	device, err := findFreeHostZRAMDevice()
	if err != nil {
		return err
	}
	name := filepath.Base(device)
	if algorithm := chooseHostZRAMAlgorithm(name, profile.Algorithm); algorithm != "" {
		if err := writeHostZRAMSysValue(name, "comp_algorithm", algorithm); err != nil {
			return err
		}
	}
	if err := writeHostZRAMSysValue(name, "disksize", strconv.FormatInt(sizeBytes, 10)); err != nil {
		return err
	}

	if result := utils.ExecCommandWithTimeout("mkswap", time.Second*30, "-f", "-L", hostZRAMLabel, device); result.Error != nil {
		return fmt.Errorf("初始化 zRAM swap 失败: %s", strings.TrimSpace(result.Stderr))
	}
	if result := utils.ExecCommandWithTimeout("swapon", time.Second*30, "--priority", strconv.Itoa(profile.Priority), device); result.Error != nil {
		return fmt.Errorf("启用 zRAM swap 失败: %s", strings.TrimSpace(result.Stderr))
	}
	return writeHostZRAMRunDevice(device)
}

func ApplyHostZRAMPersistentProfile() error {
	profileKey, _, configured := readHostZRAMPersistentConfig()
	if !configured {
		return fmt.Errorf("未找到 zRAM 持久化配置")
	}
	profile, ok := findHostZRAMProfile(profileKey)
	if !ok {
		return fmt.Errorf("未知的 zRAM 持久化挡位: %s", profileKey)
	}
	if !hostZRAMSupported() {
		return fmt.Errorf("当前宿主机缺少 zRAM 内核能力或 util-linux 相关工具")
	}
	return applyHostZRAMRuntime(profile)
}

func GetHostZRAMStatus() *HostZRAMStatus {
	runtimeConfig := readHostZRAMRuntimeConfig()
	persistentProfile, persistentConfig, persistentConfigured := readHostZRAMPersistentConfig()
	supported := hostZRAMSupported()
	status := &HostZRAMStatus{
		Supported:            supported,
		Enabled:              runtimeConfig.Device != "",
		CurrentProfile:       detectHostZRAMProfile(runtimeConfig),
		PersistentConfigured: persistentConfigured,
		PersistentProfile:    persistentProfile,
		RuntimeConfig:        runtimeConfig,
		PersistentConfig:     persistentConfig,
		Profiles:             GetHostZRAMProfiles(),
	}
	if !supported {
		status.Message = "当前宿主机缺少 zRAM 内核能力或 util-linux 相关工具"
	}
	return status
}

func SetHostZRAMProfile(profileKey string) (*HostZRAMStatus, error) {
	profile, ok := findHostZRAMProfile(profileKey)
	if !ok {
		return nil, fmt.Errorf("未知的 zRAM 挡位: %s", strings.TrimSpace(profileKey))
	}
	if !hostZRAMSupported() {
		return nil, fmt.Errorf("当前宿主机缺少 zRAM 内核能力或 util-linux 相关工具")
	}
	if err := persistHostZRAMProfile(profile); err != nil {
		return nil, err
	}
	if err := applyHostZRAMRuntime(profile); err != nil {
		return nil, err
	}
	status := GetHostZRAMStatus()
	status.Message = fmt.Sprintf("zRAM 已切换到%s挡位", profile.Name)
	return status, nil
}
