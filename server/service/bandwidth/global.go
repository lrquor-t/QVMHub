package bandwidth

import (
	"sort"
	"strings"

	"github.com/digitalocean/go-libvirt"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/service/ip_resolver"
	"qvmhub/service/libvirt_rpc"
)

// ==================== 全局带宽限制（管理员系统设置） ====================

// getRunningVMNames 获取宿主机所有运行中的VM名称列表
func getRunningVMNames() []string {
	domains, err := libvirt_rpc.ListAllDomainsRPC()
	if err != nil {
		return nil
	}
	var names []string
	for _, dom := range domains {
		l, libErr := libvirt_rpc.GetLibvirt()
		if libErr != nil {
			continue
		}
		state, _, _, _, _, infoErr := l.DomainGetInfo(dom)
		if infoErr != nil {
			continue
		}
		if libvirt.DomainState(state) == libvirt.DomainRunning {
			name := dom.Name
			if name != "" {
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names
}

// GetGlobalEffectiveBandwidth 根据系统设置计算全局有效带宽
// 规则：有效带宽 = max(1, 配置值 - 5) Mbps，未配置(0)则返回 0
// 返回 (下行Mbps, 上行Mbps)
func GetGlobalEffectiveBandwidth() (downMbps, upMbps int) {
	cfg := config.GlobalConfig
	if cfg.MaxBurstInbound > 0 {
		downMbps = cfg.MaxBurstInbound - 5
		if downMbps < 1 {
			downMbps = 1
		}
	}
	if cfg.MaxBurstOutbound > 0 {
		upMbps = cfg.MaxBurstOutbound - 5
		if upMbps < 1 {
			upMbps = 1
		}
	}
	return downMbps, upMbps
}

// listNonLightweightRunningVMs 获取所有运行中的非轻量云VM名称（用于全局带宽分配）
func listNonLightweightRunningVMs() []string {
	var names []string
	for _, name := range getRunningVMNames() {
		if strings.TrimSpace(name) == "" {
			continue
		}
		// 跳过轻量云VM（其带宽由单机配额管理）
		if HookIsLightweightCloudVM(name) {
			continue
		}
		names = append(names, name)
	}
	return names
}

// ApplyGlobalBandwidthLimit 根据系统设置中的全局带宽配置，为所有运行中的VM和VPC交换机设置外网带宽上限
//
// 规则：
//   - 当 max_burst_inbound / max_burst_outbound > 0 时启用全局限制
//   - 有效带宽 = 配置值 - 5Mbps（最少 1Mbps）
//   - 每台 VM 设置全量有效带宽作为上限（不除以 VM 数量），多台 VM 同时跑满时由 TCP 拥塞控制自然分享
//   - 非 VPC VM 直接在 VM 级应用限速
//   - VPC VM 由交换机层面聚合限速
//   - 轻量云 VM 不受全局带宽限制（由单机配额管理）
func ApplyGlobalBandwidthLimit() error {
	cfg := config.GlobalConfig
	// 未配置全局带宽限制
	if cfg.MaxBurstInbound <= 0 && cfg.MaxBurstOutbound <= 0 {
		return nil
	}

	runningVMs := listNonLightweightRunningVMs()
	vmCount := len(runningVMs)

	if vmCount == 0 {
		logger.App.Info("没有运行中的非轻量云虚拟机，跳过VM级带宽分配")
		return nil
	}

	totalDown, totalUp := GetGlobalEffectiveBandwidth()
	logger.App.Info("全局带宽有效带宽信息", "downMbps", totalDown, "upMbps", totalUp, "vmCount", vmCount)

	// 找出哪些VM属于VPC交换机，避免重复限速
	vpcSwitches := make(map[uint]bool)

	var lastErr error
	for _, vmName := range runningVMs {
		// 跳过静态获取MAC失败的VM（可能刚关闭）
		mac := ip_resolver.GetFirstVMMAC(vmName)
		if mac == "" {
			logger.App.Info("全局带宽跳过VM，无法获取MAC地址", "vm", vmName)
			continue
		}

		if sw, ok := GetVPCSwitchForVM(vmName); ok && sw != nil {
			// VPC VM：标记交换机需要重新应用带宽，不在VM级单独限制
			vpcSwitches[sw.ID] = true
			logger.App.Info("全局带宽VM属于VPC交换机，由交换机聚合限速", "vm", vmName, "switchName", sw.Name, "switchID", sw.ID)
			continue
		}

		// 非VPC VM：应用全量有效带宽限制（TCP拥塞控制在多VM之间自然分享）
		downAvgKB := MbpsToKBps(totalDown)
		upAvgKB := MbpsToKBps(totalUp)
		downBurstKB := downAvgKB * 30
		upBurstKB := upAvgKB * 30

		if err := ApplyVMBandwidth(vmName, downAvgKB, downAvgKB, downBurstKB, upAvgKB, upAvgKB, upBurstKB); err != nil {
			logger.App.Warn("全局带宽应用VM带宽限制失败", "vm", vmName, "error", err)
			lastErr = err
		}
	}

	// 为标记的VPC交换机重新应用带宽限制（effectiveVPCSwitchBandwidth 会自动考虑全局带宽上限）
	for switchID := range vpcSwitches {
		var sw model.VPCSwitch
		if err := model.DB.First(&sw, switchID).Error; err != nil {
			logger.App.Warn("全局带宽查找VPC交换机失败", "switchID", switchID, "error", err)
			continue
		}
		if err := HookApplyVPCSwitchBandwidth(sw); err != nil {
			logger.App.Warn("全局带宽应用VPC交换机带宽限制失败", "switchName", sw.Name, "switchID", sw.ID, "error", err)
			lastErr = err
		}
	}

	return lastErr
}

// ClearGlobalBandwidthLimit 清除全局带宽限制（当管理员将两个方向都设为0时）
// 清除所有VM和VPC交换机上的全局带宽限制，恢复由用户配额决定的带宽分配
func ClearGlobalBandwidthLimit() error {
	runningVMs := listNonLightweightRunningVMs()
	vpcSwitches := make(map[uint]bool)

	for _, vmName := range runningVMs {
		if mac := ip_resolver.GetFirstVMMAC(vmName); mac == "" {
			continue
		}
		if sw, ok := GetVPCSwitchForVM(vmName); ok && sw != nil {
			vpcSwitches[sw.ID] = true
			continue
		}
		if err := ClearVMBandwidth(vmName); err != nil {
			logger.App.Warn("全局带宽清除VM限速失败", "vm", vmName, "error", err)
		}
	}

	for switchID := range vpcSwitches {
		var sw model.VPCSwitch
		if err := model.DB.First(&sw, switchID).Error; err != nil {
			logger.App.Warn("全局带宽查找VPC交换机失败", "switchID", switchID, "error", err)
			continue
		}
		if err := HookApplyVPCSwitchBandwidth(sw); err != nil {
			logger.App.Warn("全局带宽恢复VPC交换机原始带宽失败", "switchName", sw.Name, "switchID", sw.ID, "error", err)
		}
	}

	return nil
}
