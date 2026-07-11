package vm

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"qvmhub/service/libvirt_rpc"
)

var monitorInfoCommandPattern = regexp.MustCompile(`^info\s+[a-zA-Z0-9_.:-]+(?:\s+[a-zA-Z0-9_.:-]+)*$`)
var monitorSendKeyCommandPattern = regexp.MustCompile(`^sendkey\s+([a-z0-9_]+(?:-[a-z0-9_]+)*)(?:\s+(\d{1,4}))?$`)

var monitorExactCommandSet = map[string]struct{}{
	"c":                       {},
	"stop":                    {},
	"help":                    {},
	"nmi":                     {},
	"system_reset":            {},
	"system_powerdown":        {},
	"system_wakeup":           {},
	"sendkey ctrl-alt-delete": {},
}

// VMMonitorStatus 虚拟机监视器状态
type VMMonitorStatus struct {
	DomainState      string `json:"domain_state"`
	MonitorAvailable bool   `json:"monitor_available"`
	MonitorStatus    string `json:"monitor_status"`
	RawOutput        string `json:"raw_output"`
}

// VMMonitorCommandResult 监视器命令执行结果
type VMMonitorCommandResult struct {
	Command string           `json:"command"`
	Output  string           `json:"output"`
	Status  *VMMonitorStatus `json:"status"`
}

// GetVMMonitorStatus 获取虚拟机当前监视器状态
func GetVMMonitorStatus(name string) (*VMMonitorStatus, error) {
	stateStr, err := libvirt_rpc.GetDomainStateRPC(name)
	if err != nil {
		return nil, fmt.Errorf("获取虚拟机状态失败: %w", err)
	}

	status := &VMMonitorStatus{
		DomainState: stateStr,
	}

	domainStateLower := strings.ToLower(status.DomainState)
	status.MonitorAvailable = strings.HasPrefix(domainStateLower, "running") || strings.HasPrefix(domainStateLower, "paused")
	if !status.MonitorAvailable {
		return status, nil
	}

	monitorOutput, err := libvirt_rpc.QemuMonitorCommandRPC(name, "info status", libvirt_rpc.DomainQemuMonitorCommandHmp)
	if err != nil {
		return nil, fmt.Errorf("获取 QEMU Monitor 状态失败: %w", err)
	}

	status.RawOutput = strings.TrimSpace(monitorOutput)
	status.MonitorStatus = parseMonitorStatus(status.RawOutput)
	return status, nil
}

// ExecuteVMMonitorCommand 执行虚拟机 QEMU Monitor 命令
func ExecuteVMMonitorCommand(name, command string) (*VMMonitorCommandResult, error) {
	normalized, err := normalizeMonitorCommand(command)
	if err != nil {
		return nil, err
	}

	status, err := GetVMMonitorStatus(name)
	if err != nil {
		return nil, err
	}
	if !status.MonitorAvailable {
		return nil, fmt.Errorf("虚拟机当前状态为 %s，QEMU Monitor 不可用", status.DomainState)
	}

	output, err := libvirt_rpc.QemuMonitorCommandRPC(name, normalized, libvirt_rpc.DomainQemuMonitorCommandHmp)
	if err != nil {
		return nil, fmt.Errorf("执行监视器命令失败: %w", err)
	}

	output = strings.TrimSpace(output)
	if output == "" {
		output = fmt.Sprintf("命令 %s 已发送", normalized)
	}

	latestStatus, statusErr := GetVMMonitorStatus(name)
	if statusErr != nil {
		latestStatus = status
	}

	return &VMMonitorCommandResult{
		Command: normalized,
		Output:  output,
		Status:  latestStatus,
	}, nil
}

func normalizeMonitorCommand(command string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(command))
	if normalized == "" {
		return "", fmt.Errorf("监视器命令不能为空")
	}

	switch normalized {
	case "cont", "continue":
		normalized = "c"
	case "c", "stop", "help", "nmi", "system_reset", "system_powerdown", "system_wakeup":
		// 直接使用 normalized
	}

	if _, ok := monitorExactCommandSet[normalized]; ok {
		return normalized, nil
	}

	if monitorInfoCommandPattern.MatchString(normalized) {
		return normalized, nil
	}

	if matches := monitorSendKeyCommandPattern.FindStringSubmatch(normalized); len(matches) > 0 {
		if matches[2] != "" {
			holdMS, _ := strconv.Atoi(matches[2])
			if holdMS <= 0 || holdMS > 5000 {
				return "", fmt.Errorf("sendkey 的按键持续时间必须在 1 到 5000 毫秒之间")
			}
		}
		return normalized, nil
	}

	return "", fmt.Errorf("当前仅支持当前虚拟机安全命令：c、stop、help、nmi、system_*、sendkey 与 info 子命令")
}

func parseMonitorStatus(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "VM status:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "VM status:"))
		}
	}
	return ""
}
