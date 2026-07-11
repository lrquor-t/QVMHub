package host

import (
	"fmt"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/utils"
)

const (
	networkWaitOnlineUnit = "systemd-networkd-wait-online.service"
)

// GetNetworkWaitOnlineStatus 获取 systemd-networkd-wait-online.service 当前状态
// 返回是否已禁用（disabled+masked）
func GetNetworkWaitOnlineStatus() (disabled bool, summary string) {
	// 优先返回数据库持久化值，其次检测实际系统状态
	disabled = config.GlobalConfig.NetworkWaitOnlineDisabled

	// 检测实际系统状态
	maskedResult := utils.ExecCommandQuiet("systemctl", "is-enabled", networkWaitOnlineUnit)
	isEnabled := maskedResult.ExitCode == 0

	if isEnabled {
		summary = fmt.Sprintf("当前状态：已启用（systemd-networkd-wait-online.service 正常启用，OVS 桥接后开机可能卡在此服务上）")
	} else {
		// 可能是 masked 或 disabled
		isMasked := utils.ExecCommandQuiet("systemctl", "is-enabled", networkWaitOnlineUnit)
		if isMasked.ExitCode != 0 {
			summary = fmt.Sprintf("当前状态：已禁用/已 mask（系统开机不再等待网络就绪，适合 OVS 桥接环境）")
		} else {
			summary = fmt.Sprintf("当前状态：已启用")
		}
	}

	return disabled, summary
}

// SetNetworkWaitOnlineDisabled 设置 systemd-networkd-wait-online.service 禁用状态
// disabled=true: systemctl disable + mask
// disabled=false: systemctl unmask + enable
func SetNetworkWaitOnlineDisabled(disabled bool) error {
	if disabled {
		// 禁用并 mask
		disableResult := utils.ExecCommand("systemctl", "disable", networkWaitOnlineUnit)
		if disableResult.Error != nil {
			logger.App.Warn("systemctl disable 失败，尝试继续 mask", "unit", networkWaitOnlineUnit, "error", disableResult.Error)
		}

		maskResult := utils.ExecCommand("systemctl", "mask", networkWaitOnlineUnit)
		if maskResult.Error != nil {
			return fmt.Errorf("systemctl mask %s 失败: %w", networkWaitOnlineUnit, maskResult.Error)
		}

		logger.App.Info("已禁用网络等待就绪检测", "unit", networkWaitOnlineUnit)

		// 如果服务当前正在运行，也停掉它（避免当前启动卡住）
		stopResult := utils.ExecCommandQuiet("systemctl", "stop", networkWaitOnlineUnit)
		if stopResult.Error != nil {
			logger.App.Debug("systemctl stop 失败（可能服务未运行）", "unit", networkWaitOnlineUnit, "error", stopResult.Error)
		}
	} else {
		// 恢复：unmask + enable
		unmaskResult := utils.ExecCommand("systemctl", "unmask", networkWaitOnlineUnit)
		if unmaskResult.Error != nil {
			return fmt.Errorf("systemctl unmask %s 失败: %w", networkWaitOnlineUnit, unmaskResult.Error)
		}

		enableResult := utils.ExecCommand("systemctl", "enable", networkWaitOnlineUnit)
		if enableResult.Error != nil {
			return fmt.Errorf("systemctl enable %s 失败: %w", networkWaitOnlineUnit, enableResult.Error)
		}

		logger.App.Info("已恢复网络等待就绪检测", "unit", networkWaitOnlineUnit)
	}

	// 更新运行时配置
	config.GlobalConfig.NetworkWaitOnlineDisabled = disabled
	return nil
}
