package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"qvmhub/utils"
)

const (
	kvmIntelModuleName                   = "kvm_intel"
	kvmIntelUnrestrictedGuestParamPath   = "/sys/module/kvm_intel/parameters/unrestricted_guest"
	kvmIntelNestedParamPath              = "/sys/module/kvm_intel/parameters/nested"
	kvmIntelUnrestrictedGuestConfigPath  = "/etc/modprobe.d/kvm-intel-unrestricted-guest.conf"
	kvmIntelUnrestrictedGuestConfigKey   = "unrestricted_guest"
	kvmIntelUnrestrictedGuestConfigValue = "# 由 kvm_console 管理：控制 Intel KVM unrestricted_guest 兼容性\n"
)

type HostKVMIntelUnrestrictedGuestStatus struct {
	Supported            bool     `json:"supported"`
	Module               string   `json:"module"`
	RuntimeAvailable     bool     `json:"runtime_available"`
	RuntimeEnabled       bool     `json:"runtime_enabled"`
	PersistentConfigured bool     `json:"persistent_configured"`
	PersistentEnabled    *bool    `json:"persistent_enabled"`
	ActiveVMCount        int      `json:"active_vm_count"`
	ActiveVMNames        []string `json:"active_vm_names"`
	CanApplyRuntime      bool     `json:"can_apply_runtime"`
	RuntimeApplied       bool     `json:"runtime_applied"`
	RequiresReload       bool     `json:"requires_reload"`
	Message              string   `json:"message"`
}

func parseKernelModuleBool(raw string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "y", "yes", "true", "on":
		return true, true
	case "0", "n", "no", "false", "off":
		return false, true
	default:
		return false, false
	}
}

func readKernelModuleBool(path string) (bool, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, false
	}
	return parseKernelModuleBool(string(content))
}

func parseKVMIntelPersistentUnrestrictedGuest() (*bool, bool) {
	files, err := filepath.Glob("/etc/modprobe.d/*.conf")
	if err != nil {
		return nil, false
	}

	var found *bool
	configured := false
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if idx := strings.Index(line, "#"); idx >= 0 {
				line = line[:idx]
			}
			fields := strings.Fields(line)
			if len(fields) < 3 || fields[0] != "options" || fields[1] != kvmIntelModuleName {
				continue
			}
			for _, field := range fields[2:] {
				key, value, ok := strings.Cut(field, "=")
				if !ok || key != kvmIntelUnrestrictedGuestConfigKey {
					continue
				}
				if parsed, valid := parseKernelModuleBool(value); valid {
					valueCopy := parsed
					found = &valueCopy
					configured = true
				}
			}
		}
	}

	return found, configured
}

func getActiveKVMVMNames() []string {
	result := utils.ExecCommand("virsh", "list", "--name", "--state-running", "--state-paused")
	if result.Error != nil {
		return nil
	}
	names := make([]string, 0)
	for _, line := range strings.Split(result.Stdout, "\n") {
		name := strings.TrimSpace(line)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func buildHostKVMIntelUnrestrictedGuestStatus(message string) *HostKVMIntelUnrestrictedGuestStatus {
	runtimeEnabled, runtimeAvailable := readKernelModuleBool(kvmIntelUnrestrictedGuestParamPath)
	persistentEnabled, persistentConfigured := parseKVMIntelPersistentUnrestrictedGuest()
	activeVMNames := getActiveKVMVMNames()
	status := &HostKVMIntelUnrestrictedGuestStatus{
		Supported:            runtimeAvailable,
		Module:               kvmIntelModuleName,
		RuntimeAvailable:     runtimeAvailable,
		RuntimeEnabled:       runtimeEnabled,
		PersistentConfigured: persistentConfigured,
		PersistentEnabled:    persistentEnabled,
		ActiveVMCount:        len(activeVMNames),
		ActiveVMNames:        activeVMNames,
		CanApplyRuntime:      runtimeAvailable && len(activeVMNames) == 0,
		Message:              message,
	}
	status.RequiresReload = runtimeAvailable && persistentConfigured && persistentEnabled != nil && *persistentEnabled != runtimeEnabled
	if !runtimeAvailable && message == "" {
		status.Message = "当前宿主机未加载 kvm_intel，或不是 Intel KVM 环境"
	}
	return status
}

func GetHostKVMIntelUnrestrictedGuestStatus() *HostKVMIntelUnrestrictedGuestStatus {
	return buildHostKVMIntelUnrestrictedGuestStatus("")
}

func writeKVMIntelUnrestrictedGuestConfig(enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}
	content := fmt.Sprintf("%soptions %s %s=%s\n",
		kvmIntelUnrestrictedGuestConfigValue,
		kvmIntelModuleName,
		kvmIntelUnrestrictedGuestConfigKey,
		value,
	)
	if err := os.MkdirAll(filepath.Dir(kvmIntelUnrestrictedGuestConfigPath), 0755); err != nil {
		return fmt.Errorf("创建 modprobe 配置目录失败: %w", err)
	}
	if err := os.WriteFile(kvmIntelUnrestrictedGuestConfigPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入 modprobe 配置失败: %w", err)
	}
	return nil
}

func readModuleRefcnt(moduleName string) (int, bool) {
	refcntPath := fmt.Sprintf("/sys/module/%s/refcnt", moduleName)
	content, err := os.ReadFile(refcntPath)
	if err != nil {
		return 0, false
	}
	var refcnt int
	if _, scanErr := fmt.Sscanf(strings.TrimSpace(string(content)), "%d", &refcnt); scanErr != nil {
		return 0, false
	}
	return refcnt, true
}

func reloadKVMIntelModule(enabled bool) error {
	nestedEnabled, nestedAvailable := readKernelModuleBool(kvmIntelNestedParamPath)

	if refcnt, ok := readModuleRefcnt(kvmIntelModuleName); ok && refcnt > 0 {
		return fmt.Errorf("kvm_intel 模块引用计数为 %d，正在被内核使用，无法热卸载。可能原因：系统运行在 KVM 嵌套虚拟化环境中、模块被其他进程或内核组件持有。请重启宿主机以应用更改", refcnt)
	}

	if result := utils.ExecCommand("modprobe", "-r", kvmIntelModuleName); result.Error != nil {
		errMsg := strings.TrimSpace(result.Stderr)
		if strings.Contains(errMsg, "is in use") || strings.Contains(errMsg, "in use") {
			return fmt.Errorf("kvm_intel 模块正在被使用，无法热卸载。请关闭所有虚拟机后重试，或重启宿主机以应用更改")
		}
		return fmt.Errorf("卸载 kvm_intel 模块失败: %s", errMsg)
	}

	value := "0"
	if enabled {
		value = "1"
	}
	args := []string{kvmIntelModuleName}
	if nestedAvailable {
		if nestedEnabled {
			args = append(args, "nested=1")
		} else {
			args = append(args, "nested=0")
		}
	}
	args = append(args, fmt.Sprintf("%s=%s", kvmIntelUnrestrictedGuestConfigKey, value))
	if result := utils.ExecCommand("modprobe", args...); result.Error != nil {
		restoreResult := utils.ExecCommand("modprobe", kvmIntelModuleName)
		if restoreResult.Error != nil {
			return fmt.Errorf("加载 kvm_intel 模块失败: %s；恢复模块也失败: %s", strings.TrimSpace(result.Stderr), strings.TrimSpace(restoreResult.Stderr))
		}
		return fmt.Errorf("加载 kvm_intel 模块失败: %s", strings.TrimSpace(result.Stderr))
	}
	return nil
}

func SetHostKVMIntelUnrestrictedGuest(enabled bool) (*HostKVMIntelUnrestrictedGuestStatus, error) {
	if err := writeKVMIntelUnrestrictedGuestConfig(enabled); err != nil {
		return nil, err
	}

	status := buildHostKVMIntelUnrestrictedGuestStatus("配置已保存")
	if !status.RuntimeAvailable {
		status.RequiresReload = true
		status.Message = "配置已保存；当前未加载 kvm_intel，需在 Intel KVM 宿主机加载模块或重启后生效"
		return status, nil
	}
	if status.RuntimeEnabled == enabled {
		status.RuntimeApplied = true
		status.RequiresReload = false
		status.Message = "配置已保存，当前运行时状态已经一致"
		return status, nil
	}
	if len(status.ActiveVMNames) > 0 {
		status.RequiresReload = true
		status.Message = "配置已保存；当前有虚拟机正在运行或暂停，需要关停所有虚拟机后重载 KVM 模块，或重启宿主机后生效"
		return status, nil
	}

	if err := reloadKVMIntelModule(enabled); err != nil {
		status.RequiresReload = true
		status.Message = "配置已保存，但运行时应用失败：" + err.Error()
		return status, nil
	}

	status = buildHostKVMIntelUnrestrictedGuestStatus("配置已保存并已应用到当前运行时")
	status.RuntimeApplied = true
	status.RequiresReload = false
	return status, nil
}
