package bandwidth

import (
	"fmt"
	"strings"

	"github.com/digitalocean/go-libvirt"

	"qvmhub/config"
	"qvmhub/model"
	"qvmhub/service/ip_resolver"
	"qvmhub/service/libvirt_rpc"
)

// ==================== VM 带宽管理 ====================

// ReapplyConfiguredVMBandwidth 按持久化配置重刷运行态带宽规则。
// 这一步会在 VM 重新获得新的 vnet/ofport 后清理旧流表，再按当前端口重新下发。
func ReapplyConfiguredVMBandwidth(vmName string) error {
	// VM 无网口则无需刷新带宽
	mac := ip_resolver.GetFirstVMMAC(vmName)
	if mac == "" {
		return nil
	}
	config, err := getVMBandwidthConfigRaw(vmName)
	if err != nil {
		return err
	}
	return ApplyVMBandwidth(vmName,
		config.InboundAvg, config.InboundPeak, config.InboundBurst,
		config.OutboundAvg, config.OutboundPeak, config.OutboundBurst,
	)
}

// SetVMCustomAverage 用户自定义某台 VM 的平峰速率
// 校验：修改后该 VM 的 average 不超过用户配额，且所有 VM 的 average 总和不超配额
func SetVMCustomAverage(username, vmName string, inAvgMbps, outAvgMbps int) error {
	if HookIsLightweightCloudVM(vmName) {
		return fmt.Errorf("轻量云服务器的带宽由管理员在单机配额中设置，不能在 VM 详情页修改")
	}
	if IsVPCBoundVM(vmName) {
		if inAvgMbps < 0 || outAvgMbps < 0 {
			return fmt.Errorf("VM 级带宽不能小于 0")
		}
		if inAvgMbps == 0 && outAvgMbps == 0 {
			return ClearVMBandwidth(vmName)
		}
		inAvgKB := MbpsToKBps(inAvgMbps)
		outAvgKB := MbpsToKBps(outAvgMbps)
		return ApplyVMBandwidth(vmName, inAvgKB, inAvgKB, inAvgKB*30, outAvgKB, outAvgKB, outAvgKB*30)
	}

	// 获取用户信息
	var user model.User
	if err := model.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}

	// 获取用户所有 VM
	vms := HookGetUserVMList(username)
	if len(vms) == 0 {
		return fmt.Errorf("用户没有虚拟机")
	}

	// 校验单台 VM 不超过用户配额
	if user.MaxBandwidthDown > 0 && inAvgMbps <= 0 {
		return fmt.Errorf("下行平峰速率必须大于 0Mbps，当前用户下行配额为 %.2fMbps", user.MaxBandwidthDown)
	}
	if user.MaxBandwidthUp > 0 && outAvgMbps <= 0 {
		return fmt.Errorf("上行平峰速率必须大于 0Mbps，当前用户上行配额为 %.2fMbps", user.MaxBandwidthUp)
	}
	if user.MaxBandwidthDown > 0 && float64(inAvgMbps) > user.MaxBandwidthDown {
		return fmt.Errorf("下行平峰速率 %dMbps 超过配额上限 %.2fMbps", inAvgMbps, user.MaxBandwidthDown)
	}
	if user.MaxBandwidthUp > 0 && float64(outAvgMbps) > user.MaxBandwidthUp {
		return fmt.Errorf("上行平峰速率 %dMbps 超过配额上限 %.2fMbps", outAvgMbps, user.MaxBandwidthUp)
	}

	// 统计其他 VM 的 average 总和
	var otherInTotal, otherOutTotal int
	for _, vm := range vms {
		if vm == vmName {
			continue
		}
		bw, err := GetVMBandwidth(vm)
		if err != nil {
			continue
		}
		otherInTotal += bw.InboundAvg
		otherOutTotal += bw.OutboundAvg
	}

	// 校验总和不超配额
	if user.MaxBandwidthDown > 0 && float64(otherInTotal+inAvgMbps) > user.MaxBandwidthDown {
		return fmt.Errorf("下行总带宽超出配额（其他VM已用 %dMbps + 本机 %dMbps > 上限 %.2fMbps）",
			otherInTotal, inAvgMbps, user.MaxBandwidthDown)
	}
	if user.MaxBandwidthUp > 0 && float64(otherOutTotal+outAvgMbps) > user.MaxBandwidthUp {
		return fmt.Errorf("上行总带宽超出配额（其他VM已用 %dMbps + 本机 %dMbps > 上限 %.2fMbps）",
			otherOutTotal, outAvgMbps, user.MaxBandwidthUp)
	}

	cfg := config.GlobalConfig

	// 计算 peak 和 burst（固定，用户不可修改）
	downPeak := int(user.MaxBandwidthDown * 125)
	upPeak := int(user.MaxBandwidthUp * 125)

	var downBurst, upBurst int
	if user.MaxBandwidthDown > 0 {
		if cfg.MaxBurstInbound > 0 {
			downBurst = MbpsToKBps(cfg.MaxBurstInbound) * 30
		} else {
			downBurst = downPeak * 30
		}
	}
	if user.MaxBandwidthUp > 0 {
		if cfg.MaxBurstOutbound > 0 {
			upBurst = MbpsToKBps(cfg.MaxBurstOutbound) * 30
		} else {
			upBurst = upPeak * 30
		}
	}

	downTrafficLimited, upTrafficLimited := HookIsUserTrafficLimited(username)
	if downTrafficLimited {
		penaltyDown := MbpsToKBps(10)
		inAvgMbps = 10
		downPeak = penaltyDown
		downBurst = penaltyDown * 30
	}
	if upTrafficLimited {
		penaltyUp := MbpsToKBps(1)
		outAvgMbps = 1
		upPeak = penaltyUp
		upBurst = penaltyUp * 30
	}

	// 参数顺序：(downAvg, downPeak, downBurst, upAvg, upPeak, upBurst)
	if err := ApplyVMBandwidth(vmName, MbpsToKBps(inAvgMbps), downPeak, downBurst, MbpsToKBps(outAvgMbps), upPeak, upBurst); err != nil {
		return err
	}
	HookRefreshVMCacheByNameAsync(vmName)
	return nil
}

// GetVMBandwidthMbps 获取 VM 带宽的简要信息（Mbps）
func GetVMBandwidthMbps(vmName string) (inAvg, outAvg int) {
	mac := ip_resolver.GetFirstVMMAC(vmName)
	if mac == "" {
		return 0, 0
	}

	params, err := libvirt_rpc.GetInterfaceParametersRPC(vmName, mac, uint32(libvirt.DomainAffectConfig))
	if err != nil {
		return 0, 0
	}
	for _, p := range params {
		switch p.Field {
		case libvirt.DomainBandwidthInAverage:
			if v, ok := p.Value.I.(int64); ok {
				inAvg = KBpsToMbps(int(v)) // inbound = VM 下行
			} else if v, ok := p.Value.I.(uint64); ok {
				inAvg = KBpsToMbps(int(v))
			}
		case libvirt.DomainBandwidthOutAverage:
			if v, ok := p.Value.I.(int64); ok {
				outAvg = KBpsToMbps(int(v)) // outbound = VM 上行
			} else if v, ok := p.Value.I.(uint64); ok {
				outAvg = KBpsToMbps(int(v))
			}
		}
	}
	return inAvg, outAvg
}

// IsVPCBoundVM 判断 VM 是否绑定到 VPC 交换机
func IsVPCBoundVM(vmName string) bool {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" || model.DB == nil {
		return false
	}
	var count int64
	model.DB.Model(&model.VPCVMBinding{}).Where("vm_name = ?", vmName).Count(&count)
	return count > 0
}
