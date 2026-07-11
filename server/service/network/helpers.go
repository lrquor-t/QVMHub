package network

import (
	"fmt"
	"strings"

	"qvmhub/config"
	"qvmhub/utils"
)

// GetUFWStatus 获取 UFW 状态
func GetUFWStatus() (string, error) {
	result := utils.ExecCommand("ufw", "status", "numbered")
	if result.Error != nil {
		return "", fmt.Errorf("获取 UFW 状态失败: %s", result.Stderr)
	}
	return result.Stdout, nil
}

// ManageUFWRule 管理 UFW 规则
func ManageUFWRule(action, rule string) error {
	var cmd string
	switch action {
	case "allow":
		cmd = fmt.Sprintf("ufw allow %s", rule)
	case "deny":
		cmd = fmt.Sprintf("ufw deny %s", rule)
	case "delete":
		cmd = fmt.Sprintf("ufw delete %s", rule)
	default:
		return fmt.Errorf("不支持的操作: %s", action)
	}

	result := utils.ExecShell(cmd)
	if result.Error != nil {
		return fmt.Errorf("UFW 操作失败: %s", result.Stderr)
	}
	return nil
}

// getHostIP 获取宿主机外网 IP
func getHostIP() string {
	// 优先使用配置的固定 IP
	if config.GlobalConfig.HostIP != "" {
		return config.GlobalConfig.HostIP
	}

	// 使用配置的外网网卡名称
	nic := config.GlobalConfig.ExternalNIC
	if nic != "" {
		result := utils.ExecShell(fmt.Sprintf(
			"ip -4 addr show %s 2>/dev/null | grep -oP '(?<=inet\\s)\\d+\\.\\d+\\.\\d+\\.\\d+'", utils.ShellSingleQuote(nic)))
		if result.Error == nil && result.Stdout != "" {
			return strings.TrimSpace(result.Stdout)
		}
	}

	// 自动检测：通过默认路由获取外网网卡 IP
	result := utils.ExecShell(
		"ip -4 route get 8.8.8.8 2>/dev/null | grep -oP 'src \\K\\S+'")
	if result.Error == nil && result.Stdout != "" {
		return strings.TrimSpace(result.Stdout)
	}

	return "0.0.0.0"
}
