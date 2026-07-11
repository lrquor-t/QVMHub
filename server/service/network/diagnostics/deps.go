package diagnostics

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"qvmhub/config"
)

// deps.go — diagnostics 子包通过 hook 变量调用 service 根包函数，避免循环 import。
// service 根包在 network_diagnostics_register.go 的 init() 中为这些变量赋值。

// ── Hooks ──

var (
	// HookGetVMNetworkRuntimeStatus 获取 VM 网络运行时状态（由 service 根包注入）
	HookGetVMNetworkRuntimeStatus func(vmName string) (*VMNetworkRuntimeStatus, error)
	// HookListLivePortForwardsFromIPTables 获取 iptables 中的端口转发规则（由 service 根包注入）
	HookListLivePortForwardsFromIPTables func() ([]PortForwardRule, error)
	// HookExecCommand 执行系统命令（由 service 根包注入，封装 utils.ExecCommand）
	HookExecCommand func(name string, args ...string) ExecResult
)

// ExecResult 等价于 utils.CmdResult（仅含 diagnostics 所需字段）
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Error    error
}

// ── Config helpers ──

func networkCaptureDir() string {
	if config.GlobalConfig == nil || strings.TrimSpace(config.GlobalConfig.NetworkCaptureDir) == "" {
		return "/var/lib/kvm-console/captures"
	}
	return config.GlobalConfig.NetworkCaptureDir
}

func captureDefaultSeconds() int {
	if config.GlobalConfig == nil || config.GlobalConfig.NetworkCaptureDefaultSeconds <= 0 {
		return 30
	}
	return config.GlobalConfig.NetworkCaptureDefaultSeconds
}

func captureMaxSeconds() int {
	if config.GlobalConfig == nil || config.GlobalConfig.NetworkCaptureMaxSeconds <= 0 {
		return 120
	}
	return config.GlobalConfig.NetworkCaptureMaxSeconds
}

func captureMaxMB() int {
	if config.GlobalConfig == nil || config.GlobalConfig.NetworkCaptureMaxMB <= 0 {
		return 64
	}
	return config.GlobalConfig.NetworkCaptureMaxMB
}

func captureMaxPackets() int {
	if config.GlobalConfig == nil || config.GlobalConfig.NetworkCaptureMaxPackets <= 0 {
		return 5000
	}
	return config.GlobalConfig.NetworkCaptureMaxPackets
}

// ── Utility helpers ──

func clampInt(value, defaultValue, maxValue int) int {
	if value <= 0 {
		value = defaultValue
	}
	if maxValue > 0 && value > maxValue {
		value = maxValue
	}
	return value
}

func validPort(port int) bool {
	return port >= 1 && port <= 65535
}

func captureFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func sanitizeFilePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "vm"
	}
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('_')
		}
	}
	result := builder.String()
	if result == "" {
		return "vm"
	}
	return result
}

func isUsableCaptureInterface(iface VMNetworkInterface) bool {
	target := strings.TrimSpace(iface.Target)
	if target == "" || target == "-" {
		return false
	}
	if !(strings.HasPrefix(target, "vnet") || strings.HasPrefix(target, "tap")) {
		return false
	}
	return iface.OFPort != "" && iface.OFPort != "-1"
}

func readNetworkNeighbors(iface string) []string {
	if HookExecCommand == nil {
		return []string{}
	}
	result := HookExecCommand("ip", "neigh", "show", "dev", iface)
	if result.Error != nil || strings.TrimSpace(result.Stdout) == "" {
		return []string{}
	}
	var lines []string
	for _, line := range strings.Split(result.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func portForwardsForVMInterfaces(interfaces []VMNetworkInterface) []PortForwardRule {
	if HookListLivePortForwardsFromIPTables == nil {
		return []PortForwardRule{}
	}
	ips := make(map[string]bool)
	for _, iface := range interfaces {
		if iface.IP != "" {
			ips[iface.IP] = true
		}
	}
	rules, err := HookListLivePortForwardsFromIPTables()
	if err != nil {
		return []PortForwardRule{}
	}
	var result []PortForwardRule
	for _, rule := range rules {
		if ips[rule.DestIP] {
			result = append(result, rule)
		}
	}
	return result
}

// CaptureFilePathAbs validates and returns the absolute capture file path.
// Exported for service root delegate.
func CaptureFilePathAbs(taskID uint) (string, string, error) {
	session, ok := GetNetworkCaptureSession(taskID)
	if !ok || session.FileName == "" {
		return "", "", fmt.Errorf("抓包文件不存在或任务尚未完成")
	}
	fileName := filepath.Base(session.FileName)
	filePath := filepath.Join(networkCaptureDir(), fileName)
	absDir, _ := filepath.Abs(networkCaptureDir())
	absFile, _ := filepath.Abs(filePath)
	if absDir != "" && !strings.HasPrefix(absFile, absDir+string(os.PathSeparator)) && absFile != absDir {
		return "", "", fmt.Errorf("抓包文件路径异常")
	}
	if _, err := os.Stat(filePath); err != nil {
		return "", "", fmt.Errorf("抓包文件不存在或已过期")
	}
	return filePath, fileName, nil
}
