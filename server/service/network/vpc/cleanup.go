package vpc

import (
	"fmt"
	"strings"

	"qvmhub/logger"
	"qvmhub/model"
)

func CleanupUserNetworkResources(username string, vmNames []string) error {
	username = strings.TrimSpace(username)
	if username == "" || model.DB == nil {
		return nil
	}
	HookCleanupOVSStaticHostsForVMs(vmNames)

	var switches []model.VPCSwitch
	if err := model.DB.Where("username = ?", username).Find(&switches).Error; err != nil {
		return fmt.Errorf("查询用户 VPC 交换机失败: %w", err)
	}
	for _, sw := range switches {
		HookRemovePortForwardsForCIDR(sw.CIDR)
		if err := removeVPCSwitchRuntime(sw); err != nil {
			logger.App.Warn("清理用户 VPC 交换机运行态失败", "user", username, "switch", sw.Name, "id", sw.ID, "error", err)
		}
	}

	var groups []model.VPCSecurityGroup
	if err := model.DB.Where("username = ?", username).Find(&groups).Error; err != nil {
		return fmt.Errorf("查询用户安全组失败: %w", err)
	}
	groupIDs := make([]uint, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
	}
	if len(groupIDs) > 0 {
		if err := model.DB.Where("security_group_id IN ?", groupIDs).Delete(&model.VPCSecurityGroupRule{}).Error; err != nil {
			return fmt.Errorf("删除用户安全组规则失败: %w", err)
		}
	}
	if err := model.DB.Where("username = ?", username).Delete(&model.VPCVMBinding{}).Error; err != nil {
		return fmt.Errorf("删除用户 VPC VM 绑定失败: %w", err)
	}
	if err := model.DB.Where("username = ?", username).Delete(&model.VPCSwitchTrafficMonthly{}).Error; err != nil {
		return fmt.Errorf("删除用户 VPC 交换机流量记录失败: %w", err)
	}
	if err := model.DB.Where("username = ?", username).Delete(&model.VPCSecurityGroup{}).Error; err != nil {
		return fmt.Errorf("删除用户安全组失败: %w", err)
	}
	if err := model.DB.Where("username = ?", username).Delete(&model.VPCSwitch{}).Error; err != nil {
		return fmt.Errorf("删除用户 VPC 交换机失败: %w", err)
	}

	if len(switches) > 0 || len(groups) > 0 {
		if err := ApplyVPCACLRules(); err != nil {
			logger.App.Warn("清理用户后重建 VPC ACL 失败", "user", username, "error", err)
		}
		if err := HookSavePortForwardRules(); err != nil {
			logger.App.Warn("清理用户后保存端口转发规则失败", "user", username, "error", err)
		}
	}
	return nil
}

func CleanupVMVPCBinding(vmName string) {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" || model.DB == nil {
		return
	}
	var binding model.VPCVMBinding
	if err := model.DB.Where("vm_name = ?", vmName).First(&binding).Error; err != nil {
		return
	}
	switchID := binding.SwitchID
	if err := model.DB.Delete(&binding).Error; err != nil {
		logger.App.Warn("清理 VM VPC 绑定失败", "vm", vmName, "error", err)
		return
	}
	var sw model.VPCSwitch
	if err := model.DB.First(&sw, switchID).Error; err == nil {
		if err := ApplyVPCSwitchBandwidth(sw); err != nil {
			logger.App.Warn("清理 VM 后刷新交换机带宽失败", "vm", vmName, "switch", sw.Name, "id", sw.ID, "error", err)
		}
	}
	if err := ApplyVPCACLRules(); err != nil {
		logger.App.Warn("清理 VM 后重建 VPC ACL 失败", "vm", vmName, "error", err)
	}
}

func checkSwitchResourceQuota(username string, excludeID uint, req VPCSwitchRequest) error {
	var user model.User
	if err := model.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return fmt.Errorf("用户不存在")
	}
	if err := validateSwitchDirectionTrafficQuota("下行", user.MaxTrafficDown, req.TrafficDownGB); err != nil {
		return err
	}
	if err := validateSwitchDirectionTrafficQuota("上行", user.MaxTrafficUp, req.TrafficUpGB); err != nil {
		return err
	}
	if err := validateSwitchDirectionBandwidthQuota("下行", user.MaxBandwidthDown, req.BandwidthDownMbps); err != nil {
		return err
	}
	if err := validateSwitchDirectionBandwidthQuota("上行", user.MaxBandwidthUp, req.BandwidthUpMbps); err != nil {
		return err
	}
	var switches []model.VPCSwitch
	model.DB.Where("username = ?", username).Find(&switches)
	totalDown := req.TrafficDownGB
	totalUp := req.TrafficUpGB
	totalBandwidthDown := float64(req.BandwidthDownMbps)
	totalBandwidthUp := float64(req.BandwidthUpMbps)
	for _, sw := range switches {
		if sw.ID == excludeID {
			continue
		}
		if user.MaxBandwidthDown > 0 && sw.BandwidthDownMbps <= 0 {
			return fmt.Errorf("已有交换机 %s 的下行总带宽为不限，请先调整后再继续", sw.Name)
		}
		if user.MaxBandwidthUp > 0 && sw.BandwidthUpMbps <= 0 {
			return fmt.Errorf("已有交换机 %s 的上行总带宽为不限，请先调整后再继续", sw.Name)
		}
		totalDown += sw.TrafficDownGB
		totalUp += sw.TrafficUpGB
		totalBandwidthDown += float64(sw.BandwidthDownMbps)
		totalBandwidthUp += float64(sw.BandwidthUpMbps)
	}
	if user.MaxTrafficDown > 0 && totalDown > user.MaxTrafficDown {
		return fmt.Errorf("下行月流量配额不足：交换机合计 %.2fGB / 用户上限 %.2fGB", totalDown, user.MaxTrafficDown)
	}
	if user.MaxTrafficUp > 0 && totalUp > user.MaxTrafficUp {
		return fmt.Errorf("上行月流量配额不足：交换机合计 %.2fGB / 用户上限 %.2fGB", totalUp, user.MaxTrafficUp)
	}
	if user.MaxBandwidthDown > 0 && totalBandwidthDown > user.MaxBandwidthDown {
		return fmt.Errorf("下行总带宽配额不足：交换机合计 %.2fMbps / 用户上限 %.2fMbps", totalBandwidthDown, user.MaxBandwidthDown)
	}
	if user.MaxBandwidthUp > 0 && totalBandwidthUp > user.MaxBandwidthUp {
		return fmt.Errorf("上行总带宽配额不足：交换机合计 %.2fMbps / 用户上限 %.2fMbps", totalBandwidthUp, user.MaxBandwidthUp)
	}
	return nil
}

func validateSwitchDirectionTrafficQuota(label string, userMax, switchValue float64) error {
	if switchValue < 0 {
		return fmt.Errorf("交换机%s月流量配额不能小于 0", label)
	}
	if userMax > 0 && switchValue <= 0 {
		return fmt.Errorf("用户%s月流量配额有限，交换机%s月流量配额必须大于 0；只有用户该方向配额不限时才能设置为 0", label, label)
	}
	return nil
}

func validateSwitchDirectionBandwidthQuota(label string, userMax float64, switchValue int) error {
	if switchValue < 0 {
		return fmt.Errorf("交换机%s总带宽不能小于 0", label)
	}
	if userMax > 0 && switchValue <= 0 {
		return fmt.Errorf("用户%s带宽配额有限，交换机%s总带宽必须大于 0；只有用户该方向带宽不限时才能设置为 0", label, label)
	}
	return nil
}
