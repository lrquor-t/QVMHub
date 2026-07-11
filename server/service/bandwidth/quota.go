package bandwidth

import (
	"fmt"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/service/ip_resolver"
)

// ==================== 配额重平衡 ====================

func listBandwidthConfigurableVMs(vms []string) []string {
	var configurable []string
	for _, vmName := range vms {
		if IsVPCBoundVM(vmName) {
			logger.App.Info("跳过VM的用户带宽重分配", "vm", vmName, "reason", "VPC交换机负责聚合限速")
			continue
		}
		if ip_resolver.GetFirstVMMAC(vmName) == "" {
			logger.App.Warn("跳过VM的速率重分配，无法获取网卡MAC地址", "vm", vmName)
			continue
		}
		configurable = append(configurable, vmName)
	}
	return configurable
}

// RebalanceUserBandwidth 重新分配用户所有 VM 的带宽
// 规则：average = 用户配额 / VM数量（均分），peak = 用户配额，burst = 系统最大速率 × 30秒
func RebalanceUserBandwidth(username string) error {
	// 获取用户信息
	var user model.User
	if err := model.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}

	vms := HookGetUserVMList(username)
	if len(vms) == 0 {
		return nil
	}
	configurableVMs := listBandwidthConfigurableVMs(vms)
	if len(configurableVMs) == 0 {
		logger.App.Warn("用户没有可配置速率限制的VM，跳过带宽重分配", "user", username)
		return nil
	}

	cfg := config.GlobalConfig

	// 如果带宽配额为 0（不限制），清除所有 VM 限制
	if user.MaxBandwidthDown <= 0 && user.MaxBandwidthUp <= 0 {
		for _, vmName := range configurableVMs {
			if err := ClearVMBandwidth(vmName); err != nil {
				fmt.Printf("[警告] 清除 VM %s 速率限制失败: %v\n", vmName, err)
			}
		}
		return nil
	}

	// 计算下行参数（对应 virsh domiftune 的 inbound）
	var downAvg, downPeak, downBurst int
	if user.MaxBandwidthDown > 0 {
		downPeak = int(user.MaxBandwidthDown * 125)                     // peak = 用户配额
		downAvg = int(user.MaxBandwidthDown*125) / len(configurableVMs) // average = 配额均分
		if downAvg < 1 {
			downAvg = 1
		}
		if cfg.MaxBurstInbound > 0 {
			downBurst = MbpsToKBps(cfg.MaxBurstInbound) * 30
		} else {
			downBurst = downPeak * 30
		}
	}

	// 计算上行参数（对应 virsh domiftune 的 outbound）
	var upAvg, upPeak, upBurst int
	if user.MaxBandwidthUp > 0 {
		upPeak = int(user.MaxBandwidthUp * 125)
		upAvg = int(user.MaxBandwidthUp*125) / len(configurableVMs)
		if upAvg < 1 {
			upAvg = 1
		}
		if cfg.MaxBurstOutbound > 0 {
			upBurst = MbpsToKBps(cfg.MaxBurstOutbound) * 30
		} else {
			upBurst = upPeak * 30
		}
	}

	// 应用到所有 VM：参数顺序 (downAvg, downPeak, downBurst, upAvg, upPeak, upBurst)
	// 检查用户是否处于流量超限状态，如果是则覆盖对应方向为惩罚速率
	downTrafficLimited, upTrafficLimited := HookIsUserTrafficLimited(username)

	var lastErr error
	for _, vmName := range configurableVMs {
		finalDownAvg, finalDownPeak, finalDownBurst := downAvg, downPeak, downBurst
		finalUpAvg, finalUpPeak, finalUpBurst := upAvg, upPeak, upBurst

		// 流量超限时覆盖为惩罚速率
		if downTrafficLimited {
			penaltyDown := MbpsToKBps(10) // 下行惩罚 10Mbps
			finalDownAvg = penaltyDown
			finalDownPeak = penaltyDown
			finalDownBurst = penaltyDown * 30
		}
		if upTrafficLimited {
			penaltyUp := MbpsToKBps(1) // 上行惩罚 1Mbps
			finalUpAvg = penaltyUp
			finalUpPeak = penaltyUp
			finalUpBurst = penaltyUp * 30
		}

		if err := ApplyVMBandwidth(vmName, finalDownAvg, finalDownPeak, finalDownBurst, finalUpAvg, finalUpPeak, finalUpBurst); err != nil {
			fmt.Printf("[警告] 为 VM %s 设置速率限制失败: %v\n", vmName, err)
			lastErr = err
		}
	}

	return lastErr
}
