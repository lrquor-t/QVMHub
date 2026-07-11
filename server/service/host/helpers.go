package host

import (
	"context"
	"fmt"
	"strings"

	"qvmhub/config"
	"qvmhub/taskqueue"
)

// IsMaintenanceModeError 判断错误是否由维护模式拦截导致。
func IsMaintenanceModeError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "系统当前处于维护模式")
}

// IsLibvirtUnavailableError 判断是否为 libvirt 服务当前不可用导致的错误。
func IsLibvirtUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	return IsLibvirtUnavailableText(err.Error())
}

// IsLibvirtUnavailableText 判断文本是否包含 libvirt 不可用的特征。
func IsLibvirtUnavailableText(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(text, "failed to connect to the hypervisor") ||
		strings.Contains(text, "failed to connect socket") ||
		strings.Contains(text, "libvirt-sock") ||
		strings.Contains(text, "libvirt socket") ||
		strings.Contains(text, "no such file or directory")
}

// ParseMaintenanceServiceUnits 解析维护模式 service units 配置。
func ParseMaintenanceServiceUnits(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == ';' || r == '\t'
	})
	seen := make(map[string]bool)
	units := make([]string, 0, len(parts))
	for _, part := range parts {
		unit := strings.TrimSpace(part)
		if unit == "" || seen[unit] {
			continue
		}
		seen[unit] = true
		units = append(units, unit)
	}
	return units
}

// FirstNonEmpty 返回第一个非空字符串。
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func checkMaintenanceCanceled(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return taskqueue.ErrTaskCanceled
	default:
		return nil
	}
}

// IsMaintenanceModeEnabled 判断当前是否已开启维护模式。
func IsMaintenanceModeEnabled() bool {
	return config.GlobalConfig != nil && config.GlobalConfig.MaintenanceMode
}

// EnsureMaintenanceModeDisabled 校验维护模式是否允许当前操作。
func EnsureMaintenanceModeDisabled(action string) error {
	if !IsMaintenanceModeEnabled() {
		return nil
	}
	action = strings.TrimSpace(action)
	if action == "" {
		action = "执行该操作"
	}
	return fmt.Errorf("系统当前处于维护模式，暂不允许%s，请先关闭维护模式", action)
}
