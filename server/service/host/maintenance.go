package host

import (
	"context"
	"fmt"
	"strings"
	"time"

	"qvmhub/config"
	"qvmhub/utils"
)

// EnterMaintenanceMode 执行维护模式收敛流程。
func EnterMaintenanceMode(ctx context.Context, params *MaintenanceModeTaskParams, progressFn func(int, string)) (*MaintenanceModeTaskResult, error) {
	if progressFn == nil {
		progressFn = func(int, string) {}
	}

	result := &MaintenanceModeTaskResult{}
	progressFn(5, "正在收集维护模式执行信息...")

	shutdownTimeout := 40
	if config.GlobalConfig != nil && config.GlobalConfig.MaintenanceVMShutdownTimeoutSeconds > 0 {
		shutdownTimeout = config.GlobalConfig.MaintenanceVMShutdownTimeoutSeconds
	}

	progressFn(20, "正在关闭运行中的虚拟机...")
	stoppedVMs, warnings, err := stopAllRunningVMsForMaintenance(ctx, time.Duration(shutdownTimeout)*time.Second)
	if err != nil {
		return result, err
	}
	result.StoppedVMs = stoppedVMs
	result.Warnings = append(result.Warnings, warnings...)

	progressFn(70, "正在停用宿主机相关服务...")
	disabledServices, warnings, err := disableMaintenanceServiceUnits(ctx, resolveMaintenanceServiceUnits(params))
	if err != nil {
		return result, err
	}
	result.DisabledServices = disabledServices
	result.Warnings = append(result.Warnings, warnings...)

	ClearRuntimeCachesForMaintenance()
	progressFn(100, "维护模式已启用，启动类操作已被阻止")
	return result, nil
}

// ExitMaintenanceMode 恢复维护模式期间停用的服务。
func ExitMaintenanceMode(ctx context.Context, params *MaintenanceModeTaskParams, progressFn func(int, string)) (*MaintenanceModeTaskResult, error) {
	if progressFn == nil {
		progressFn = func(int, string) {}
	}

	result := &MaintenanceModeTaskResult{}
	progressFn(15, "正在恢复宿主机相关服务...")

	enabledServices, warnings, err := enableMaintenanceServiceUnits(ctx, resolveMaintenanceServiceUnits(params))
	if err != nil {
		return result, err
	}
	result.EnabledServices = enabledServices
	result.Warnings = append(result.Warnings, warnings...)

	progressFn(100, "维护模式已关闭，宿主机相关服务已恢复")
	return result, nil
}

func stopAllRunningVMsForMaintenance(ctx context.Context, timeout time.Duration) ([]string, []string, error) {
	result := utils.ExecShellQuiet("virsh list --name --state-running 2>/dev/null | grep -v '^$'")
	if result.Error != nil || strings.TrimSpace(result.Stdout) == "" {
		return nil, nil, nil
	}

	var stopped []string
	var warnings []string
	for _, vmName := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		if err := checkMaintenanceCanceled(ctx); err != nil {
			return stopped, warnings, err
		}

		vmName = strings.TrimSpace(vmName)
		if vmName == "" {
			continue
		}

		state := strings.ToLower(strings.TrimSpace(utils.ExecCommand("virsh", "domstate", vmName).Stdout))
		needForceOff := strings.Contains(state, "paused")
		if !needForceOff {
			if err := HookShutdownVM(vmName); err != nil {
				needForceOff = true
			} else if !HookWaitVMShutdownForDisable(vmName, timeout) {
				needForceOff = true
			}
		}
		if needForceOff {
			if err := HookDestroyVM(vmName); err != nil {
				warnings = append(warnings, fmt.Sprintf("关闭虚拟机 %s 失败: %s", vmName, err.Error()))
				continue
			}
		}
		stopped = append(stopped, vmName)
	}
	return stopped, warnings, nil
}

func disableMaintenanceServiceUnits(ctx context.Context, units []string) ([]string, []string, error) {
	var applied []string
	var warnings []string

	panelUnit := ""
	if config.GlobalConfig != nil {
		panelUnit = strings.TrimSpace(config.GlobalConfig.ServiceUnitName)
	}

	for _, unit := range units {
		if err := checkMaintenanceCanceled(ctx); err != nil {
			return applied, warnings, err
		}
		if unit == "" {
			continue
		}

		if panelUnit != "" && unit == panelUnit {
			enableResult := utils.ExecCommand("systemctl", "enable", unit)
			if enableResult.Error != nil {
				warnings = append(warnings, fmt.Sprintf("确保面板服务 %s 开机自启失败: %s", unit, FirstNonEmpty(enableResult.Stderr, enableResult.Error.Error())))
			} else {
				warnings = append(warnings, fmt.Sprintf("已跳过面板服务 %s，面板始终保持开机自启", unit))
			}
			continue
		}

		disableResult := utils.ExecCommand("systemctl", "disable", unit)
		if disableResult.Error != nil {
			warnings = append(warnings, fmt.Sprintf("禁用服务 %s 失败: %s", unit, FirstNonEmpty(disableResult.Stderr, disableResult.Error.Error())))
			continue
		}

		stopResult := utils.ExecCommand("systemctl", "stop", unit)
		if stopResult.Error != nil {
			warnings = append(warnings, fmt.Sprintf("停止服务 %s 失败: %s", unit, FirstNonEmpty(stopResult.Stderr, stopResult.Error.Error())))
			continue
		}
		applied = append(applied, unit)
	}

	return applied, warnings, nil
}

func enableMaintenanceServiceUnits(ctx context.Context, units []string) ([]string, []string, error) {
	var applied []string
	var warnings []string
	panelUnit := ""
	if config.GlobalConfig != nil {
		panelUnit = strings.TrimSpace(config.GlobalConfig.ServiceUnitName)
	}

	for _, unit := range units {
		if err := checkMaintenanceCanceled(ctx); err != nil {
			return applied, warnings, err
		}
		if unit == "" {
			continue
		}

		if panelUnit != "" && unit == panelUnit {
			enableResult := utils.ExecCommand("systemctl", "enable", unit)
			if enableResult.Error != nil {
				warnings = append(warnings, fmt.Sprintf("确保面板服务 %s 开机自启失败: %s", unit, FirstNonEmpty(enableResult.Stderr, enableResult.Error.Error())))
			}
			continue
		}

		enableResult := utils.ExecCommand("systemctl", "enable", unit)
		if enableResult.Error != nil {
			warnings = append(warnings, fmt.Sprintf("启用服务 %s 失败: %s", unit, FirstNonEmpty(enableResult.Stderr, enableResult.Error.Error())))
			continue
		}

		startResult := utils.ExecCommand("systemctl", "start", unit)
		if startResult.Error != nil {
			warnings = append(warnings, fmt.Sprintf("启动服务 %s 失败: %s", unit, FirstNonEmpty(startResult.Stderr, startResult.Error.Error())))
			continue
		}

		applied = append(applied, unit)
	}

	return applied, warnings, nil
}

func resolveMaintenanceServiceUnits(params *MaintenanceModeTaskParams) []string {
	if params != nil && len(params.ServiceUnits) > 0 {
		return ParseMaintenanceServiceUnits(strings.Join(params.ServiceUnits, ","))
	}
	if config.GlobalConfig == nil {
		return nil
	}
	return ParseMaintenanceServiceUnits(config.GlobalConfig.MaintenanceServiceUnits)
}
