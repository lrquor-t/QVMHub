package host

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"qvmhub/utils"
)

const (
	hostKSMManagedHeader = "# 由 kvm_console 管理：宿主机 KSM 配置\n"
	hostKSMUnitName      = "kvm-console-ksm.service"
)

var (
	hostKSMBasePath   = "/sys/kernel/mm/ksm"
	hostKSMConfigPath = "/etc/kvm-console/ksm.env"
	hostKSMUnitPath   = "/etc/systemd/system/kvm-console-ksm.service"
)

var hostKSMProfiles = []HostKSMProfile{
	{
		Key:              "off",
		Name:             "关闭",
		Description:      "不扫描内存页，适合临时排障或 CPU 压力优先的宿主机。",
		Run:              0,
		PagesToScan:      100,
		SleepMillisecs:   200,
		MergeAcrossNodes: true,
		UseZeroPages:     false,
		SmartScan:        true,
	},
	{
		Key:              "conservative",
		Name:             "保守",
		Description:      "低频扫描，优先降低 CPU 开销，适合内存压力不高的虚拟化宿主机。",
		Run:              1,
		PagesToScan:      100,
		SleepMillisecs:   200,
		MergeAcrossNodes: false,
		UseZeroPages:     false,
		SmartScan:        true,
	},
	{
		Key:              "balanced",
		Name:             "均衡",
		Description:      "推荐挡位，启用零页合并，在节省内存和控制扫描开销之间取平衡。",
		Run:              1,
		PagesToScan:      500,
		SleepMillisecs:   50,
		MergeAcrossNodes: true,
		UseZeroPages:     true,
		SmartScan:        true,
	},
	{
		Key:              "aggressive",
		Name:             "积极",
		Description:      "提高扫描速度，适合 VM 密度较高且希望更快释放重复内存的宿主机。",
		Run:              1,
		PagesToScan:      2000,
		SleepMillisecs:   20,
		MergeAcrossNodes: true,
		UseZeroPages:     true,
		SmartScan:        true,
	},
	{
		Key:              "extreme",
		Name:             "极致",
		Description:      "最大化去重速度，适合内存非常紧张的纯虚拟化宿主机，CPU 开销会更明显。",
		Run:              1,
		PagesToScan:      10000,
		SleepMillisecs:   10,
		MergeAcrossNodes: true,
		UseZeroPages:     true,
		SmartScan:        true,
	},
}

func GetHostKSMProfiles() []HostKSMProfile {
	profiles := make([]HostKSMProfile, len(hostKSMProfiles))
	copy(profiles, hostKSMProfiles)
	return profiles
}

func findHostKSMProfile(key string) (HostKSMProfile, bool) {
	normalized := strings.TrimSpace(strings.ToLower(key))
	for _, profile := range hostKSMProfiles {
		if profile.Key == normalized {
			return profile, true
		}
	}
	return HostKSMProfile{}, false
}

func boolToKernelInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func kernelIntToBool(value *int) *bool {
	if value == nil {
		return nil
	}
	boolValue := *value != 0
	return &boolValue
}

func readHostKSMInt(name string) (*int, bool) {
	content, err := os.ReadFile(filepath.Join(hostKSMBasePath, name))
	if err != nil {
		return nil, false
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		return nil, false
	}
	return &value, true
}

func readHostKSMInt64(name string) *int64 {
	content, err := os.ReadFile(filepath.Join(hostKSMBasePath, name))
	if err != nil {
		return nil
	}
	value, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil {
		return nil
	}
	return &value
}

func writeHostKSMInt(name string, value int) error {
	path := filepath.Join(hostKSMBasePath, name)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取 KSM 参数 %s 失败: %w", name, err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return fmt.Errorf("打开 KSM 参数 %s 失败: %w", name, err)
	}
	defer file.Close()
	if _, err := file.WriteString(strconv.Itoa(value)); err != nil {
		return fmt.Errorf("写入 KSM 参数 %s 失败: %w", name, err)
	}
	return nil
}

func readHostKSMRuntimeConfig() (HostKSMRuntimeConfig, bool) {
	run, supported := readHostKSMInt("run")
	pagesToScan, _ := readHostKSMInt("pages_to_scan")
	sleepMillisecs, _ := readHostKSMInt("sleep_millisecs")
	mergeAcrossNodesRaw, _ := readHostKSMInt("merge_across_nodes")
	useZeroPagesRaw, _ := readHostKSMInt("use_zero_pages")
	smartScanRaw, _ := readHostKSMInt("smart_scan")

	return HostKSMRuntimeConfig{
		Run:              run,
		PagesToScan:      pagesToScan,
		SleepMillisecs:   sleepMillisecs,
		MergeAcrossNodes: kernelIntToBool(mergeAcrossNodesRaw),
		UseZeroPages:     kernelIntToBool(useZeroPagesRaw),
		SmartScan:        kernelIntToBool(smartScanRaw),
	}, supported
}

func readHostKSMMetrics() HostKSMMetrics {
	return HostKSMMetrics{
		PagesShared:   readHostKSMInt64("pages_shared"),
		PagesSharing:  readHostKSMInt64("pages_sharing"),
		PagesUnshared: readHostKSMInt64("pages_unshared"),
		PagesVolatile: readHostKSMInt64("pages_volatile"),
		PagesScanned:  readHostKSMInt64("pages_scanned"),
		FullScans:     readHostKSMInt64("full_scans"),
		GeneralProfit: readHostKSMInt64("general_profit"),
	}
}

func hostKSMConfigMatchesProfile(config HostKSMRuntimeConfig, profile HostKSMProfile) bool {
	if config.Run == nil || *config.Run != profile.Run {
		return false
	}
	if profile.Run == 0 {
		return true
	}
	if config.PagesToScan == nil || *config.PagesToScan != profile.PagesToScan {
		return false
	}
	if config.SleepMillisecs == nil || *config.SleepMillisecs != profile.SleepMillisecs {
		return false
	}
	if config.MergeAcrossNodes != nil && *config.MergeAcrossNodes != profile.MergeAcrossNodes {
		return false
	}
	if config.UseZeroPages != nil && *config.UseZeroPages != profile.UseZeroPages {
		return false
	}
	if config.SmartScan != nil && *config.SmartScan != profile.SmartScan {
		return false
	}
	return true
}

func detectHostKSMProfile(config HostKSMRuntimeConfig) string {
	for _, profile := range hostKSMProfiles {
		if hostKSMConfigMatchesProfile(config, profile) {
			return profile.Key
		}
	}
	if config.Run != nil && *config.Run == 0 {
		return "off"
	}
	return "custom"
}

func hostKSMProfileToConfig(profile HostKSMProfile) HostKSMRuntimeConfig {
	run := profile.Run
	pagesToScan := profile.PagesToScan
	sleepMillisecs := profile.SleepMillisecs
	mergeAcrossNodes := profile.MergeAcrossNodes
	useZeroPages := profile.UseZeroPages
	smartScan := profile.SmartScan
	return HostKSMRuntimeConfig{
		Run:              &run,
		PagesToScan:      &pagesToScan,
		SleepMillisecs:   &sleepMillisecs,
		MergeAcrossNodes: &mergeAcrossNodes,
		UseZeroPages:     &useZeroPages,
		SmartScan:        &smartScan,
	}
}

func readHostKSMPersistentConfig() (string, *HostKSMRuntimeConfig, bool) {
	content, err := os.ReadFile(hostKSMConfigPath)
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
	profileKey := strings.TrimSpace(values["KSM_PROFILE"])
	if profile, ok := findHostKSMProfile(profileKey); ok {
		config := hostKSMProfileToConfig(profile)
		return profile.Key, &config, true
	}
	return profileKey, nil, profileKey != ""
}

func buildHostKSMEnv(profile HostKSMProfile) string {
	return fmt.Sprintf(`%sKSM_PROFILE=%s
KSM_RUN=%d
KSM_PAGES_TO_SCAN=%d
KSM_SLEEP_MILLISECS=%d
KSM_MERGE_ACROSS_NODES=%d
KSM_USE_ZERO_PAGES=%d
KSM_SMART_SCAN=%d
`,
		hostKSMManagedHeader,
		profile.Key,
		profile.Run,
		profile.PagesToScan,
		profile.SleepMillisecs,
		boolToKernelInt(profile.MergeAcrossNodes),
		boolToKernelInt(profile.UseZeroPages),
		boolToKernelInt(profile.SmartScan),
	)
}

func buildHostKSMUnit() string {
	return `[Unit]
Description=Apply kvm_console KSM settings
After=sysinit.target local-fs.target
ConditionPathExists=/sys/kernel/mm/ksm/run

[Service]
Type=oneshot
EnvironmentFile=/etc/kvm-console/ksm.env
ExecStart=/bin/sh -c 'base=/sys/kernel/mm/ksm; write(){ [ -e "$base/$1" ] && printf "%%s" "$2" > "$base/$1"; }; if [ "${KSM_RUN:-0}" = "1" ]; then write pages_to_scan "${KSM_PAGES_TO_SCAN:-500}"; write sleep_millisecs "${KSM_SLEEP_MILLISECS:-50}"; write merge_across_nodes "${KSM_MERGE_ACROSS_NODES:-1}"; write use_zero_pages "${KSM_USE_ZERO_PAGES:-0}"; write smart_scan "${KSM_SMART_SCAN:-1}"; write run 1; else write run 0; fi'
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
`
}

func persistHostKSMProfile(profile HostKSMProfile) error {
	if err := os.MkdirAll(filepath.Dir(hostKSMConfigPath), 0755); err != nil {
		return fmt.Errorf("创建 KSM 配置目录失败: %w", err)
	}
	if err := os.WriteFile(hostKSMConfigPath, []byte(buildHostKSMEnv(profile)), 0644); err != nil {
		return fmt.Errorf("写入 KSM 配置失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(hostKSMUnitPath), 0755); err != nil {
		return fmt.Errorf("创建 systemd 配置目录失败: %w", err)
	}
	if err := os.WriteFile(hostKSMUnitPath, []byte(buildHostKSMUnit()), 0644); err != nil {
		return fmt.Errorf("写入 KSM systemd 单元失败: %w", err)
	}
	if result := utils.ExecCommand("systemctl", "daemon-reload"); result.Error != nil {
		return fmt.Errorf("刷新 systemd 配置失败: %s", strings.TrimSpace(result.Stderr))
	}
	if result := utils.ExecCommand("systemctl", "enable", hostKSMUnitName); result.Error != nil {
		return fmt.Errorf("启用 KSM 持久化服务失败: %s", strings.TrimSpace(result.Stderr))
	}
	return nil
}

func applyHostKSMRuntime(profile HostKSMProfile) error {
	if profile.Run == 0 {
		return writeHostKSMInt("run", 0)
	}

	writes := map[string]int{
		"pages_to_scan":      profile.PagesToScan,
		"sleep_millisecs":    profile.SleepMillisecs,
		"merge_across_nodes": boolToKernelInt(profile.MergeAcrossNodes),
		"use_zero_pages":     boolToKernelInt(profile.UseZeroPages),
		"smart_scan":         boolToKernelInt(profile.SmartScan),
	}
	names := make([]string, 0, len(writes))
	for name := range writes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := writeHostKSMInt(name, writes[name]); err != nil {
			return err
		}
	}
	return writeHostKSMInt("run", 1)
}

func GetHostKSMStatus() *HostKSMStatus {
	runtimeConfig, supported := readHostKSMRuntimeConfig()
	persistentProfile, persistentConfig, persistentConfigured := readHostKSMPersistentConfig()
	status := &HostKSMStatus{
		Supported:            supported,
		Enabled:              runtimeConfig.Run != nil && *runtimeConfig.Run == 1,
		CurrentProfile:       detectHostKSMProfile(runtimeConfig),
		PersistentConfigured: persistentConfigured,
		PersistentProfile:    persistentProfile,
		RuntimeConfig:        runtimeConfig,
		PersistentConfig:     persistentConfig,
		Metrics:              readHostKSMMetrics(),
		Profiles:             GetHostKSMProfiles(),
	}
	if !supported {
		status.Message = "当前宿主机未提供 KSM sysfs 接口"
	}
	return status
}

func SetHostKSMProfile(profileKey string) (*HostKSMStatus, error) {
	profile, ok := findHostKSMProfile(profileKey)
	if !ok {
		return nil, fmt.Errorf("未知的 KSM 挡位: %s", strings.TrimSpace(profileKey))
	}
	if _, supported := readHostKSMRuntimeConfig(); !supported {
		return nil, fmt.Errorf("当前宿主机未提供 KSM sysfs 接口")
	}
	if err := applyHostKSMRuntime(profile); err != nil {
		return nil, err
	}
	if err := persistHostKSMProfile(profile); err != nil {
		return nil, err
	}
	status := GetHostKSMStatus()
	status.Message = fmt.Sprintf("KSM 已切换到%s挡位", profile.Name)
	return status, nil
}
