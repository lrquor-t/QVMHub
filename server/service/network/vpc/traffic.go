package vpc

import (
	"time"

	"qvmhub/logger"
	"qvmhub/model"
)

func aggregateSwitchMonthlyTrafficRaw(switchID uint) (downBytes, upBytes int64) {
	var sw model.VPCSwitch
	if err := model.DB.First(&sw, switchID).Error; err != nil {
		return 0, 0
	}
	vmNames := listVPCSwitchVMNames(sw)
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, 0)
	for _, vmName := range vmNames {
		var records []model.VmStatsRecord
		model.DB.Where("vm_name = ? AND recorded_at >= ? AND recorded_at < ?", vmName, monthStart, monthEnd).
			Order("recorded_at ASC").Find(&records)
		for i := 1; i < len(records); i++ {
			if delta := records[i].NetRxBytes - records[i-1].NetRxBytes; delta > 0 {
				downBytes += delta
			}
			if delta := records[i].NetTxBytes - records[i-1].NetTxBytes; delta > 0 {
				upBytes += delta
			}
		}
	}
	return downBytes, upBytes
}

func AggregateSwitchMonthlyTraffic(switchID uint) (downBytes, upBytes int64) {
	rawDown, rawUp := aggregateSwitchMonthlyTrafficRaw(switchID)
	var record model.VPCSwitchTrafficMonthly
	if err := model.DB.Where("switch_id = ? AND month = ?", switchID, CurrentTrafficMonth()).First(&record).Error; err != nil {
		return rawDown, rawUp
	}
	return ClampTrafficBytes(rawDown - record.OffsetDown), ClampTrafficBytes(rawUp - record.OffsetUp)
}

func ClampTrafficBytes(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func TrafficQuotaBytes(gb float64) float64 {
	return gb * 1024 * 1024 * 1024
}

func getOrCreateVPCSwitchTrafficMonthly(sw model.VPCSwitch, month string) model.VPCSwitchTrafficMonthly {
	var record model.VPCSwitchTrafficMonthly
	if err := model.DB.Where("switch_id = ? AND month = ?", sw.ID, month).First(&record).Error; err == nil {
		return record
	}
	return model.VPCSwitchTrafficMonthly{
		SwitchID: sw.ID,
		Username: sw.Username,
		Month:    month,
	}
}

func saveVPCSwitchTrafficMonthly(record model.VPCSwitchTrafficMonthly) error {
	if record.ID == 0 {
		return model.DB.Create(&record).Error
	}
	return model.DB.Save(&record).Error
}

func rebaseVPCSwitchTrafficMonthly(switchID uint, keepDown, keepUp int64) {
	if model.DB == nil || switchID == 0 {
		return
	}
	var sw model.VPCSwitch
	if err := model.DB.First(&sw, switchID).Error; err != nil {
		return
	}
	rawDown, rawUp := aggregateSwitchMonthlyTrafficRaw(switchID)
	record := getOrCreateVPCSwitchTrafficMonthly(sw, CurrentTrafficMonth())
	record.Username = sw.Username
	record.OffsetDown = rawDown - ClampTrafficBytes(keepDown)
	record.OffsetUp = rawUp - ClampTrafficBytes(keepUp)
	record.TrafficDown = ClampTrafficBytes(keepDown)
	record.TrafficUp = ClampTrafficBytes(keepUp)
	record.IsLimitedDown = sw.TrafficDownGB > 0 && float64(record.TrafficDown) >= TrafficQuotaBytes(sw.TrafficDownGB)
	record.IsLimitedUp = sw.TrafficUpGB > 0 && float64(record.TrafficUp) >= TrafficQuotaBytes(sw.TrafficUpGB)
	if err := saveVPCSwitchTrafficMonthly(record); err != nil {
		logger.App.Warn("重算交换机月流量偏移失败", "switch", sw.Name, "id", sw.ID, "error", err)
	}
}

func effectiveVPCSwitchBandwidth(sw model.VPCSwitch) (downMbps, upMbps int) {
	normalizeVPCSwitchBandwidthForResponse(&sw)
	downMbps = sw.BandwidthDownMbps
	upMbps = sw.BandwidthUpMbps

	globalDown, globalUp := HookGetGlobalEffectiveBandwidth()
	if globalDown > 0 && (downMbps <= 0 || downMbps > globalDown) {
		downMbps = globalDown
	}
	if globalUp > 0 && (upMbps <= 0 || upMbps > globalUp) {
		upMbps = globalUp
	}

	if downLimited, upLimited := IsVPCSwitchTrafficLimited(sw.ID); downLimited || upLimited {
		if downLimited {
			downMbps = VPCSwitchTrafficPenaltyMbps
		}
		if upLimited {
			upMbps = VPCSwitchTrafficPenaltyMbps
		}
	}
	return downMbps, upMbps
}

func IsVPCSwitchTrafficLimited(switchID uint) (downLimited, upLimited bool) {
	if model.DB == nil || switchID == 0 {
		return false, false
	}
	var record model.VPCSwitchTrafficMonthly
	if err := model.DB.Where("switch_id = ? AND month = ?", switchID, CurrentTrafficMonth()).First(&record).Error; err != nil {
		return false, false
	}
	return record.IsLimitedDown, record.IsLimitedUp
}

func CheckAndApplyVPCSwitchTrafficLimit(sw model.VPCSwitch) {
	if sw.ID == 0 || model.DB == nil {
		return
	}
	rawDown, rawUp := aggregateSwitchMonthlyTrafficRaw(sw.ID)
	record := getOrCreateVPCSwitchTrafficMonthly(sw, CurrentTrafficMonth())
	effectiveDown := ClampTrafficBytes(rawDown - record.OffsetDown)
	effectiveUp := ClampTrafficBytes(rawUp - record.OffsetUp)
	record.Username = sw.Username
	record.TrafficDown = effectiveDown
	record.TrafficUp = effectiveUp

	downLimited := sw.TrafficDownGB > 0 && float64(effectiveDown) >= TrafficQuotaBytes(sw.TrafficDownGB)
	upLimited := sw.TrafficUpGB > 0 && float64(effectiveUp) >= TrafficQuotaBytes(sw.TrafficUpGB)
	wasLimited := record.IsLimitedDown || record.IsLimitedUp
	changed := record.IsLimitedDown != downLimited || record.IsLimitedUp != upLimited
	record.IsLimitedDown = downLimited
	record.IsLimitedUp = upLimited

	if err := saveVPCSwitchTrafficMonthly(record); err != nil {
		logger.App.Warn("保存交换机月流量失败", "switch", sw.Name, "id", sw.ID, "error", err)
		return
	}
	if changed || wasLimited != (downLimited || upLimited) {
		if err := ApplyVPCSwitchBandwidth(sw); err != nil {
			logger.App.Warn("应用交换机限速状态失败", "switch", sw.Name, "id", sw.ID, "error", err)
		}
	}
	if (downLimited || upLimited) && changed {
		logger.App.Warn("交换机流量超限，已强制限速", "switch", sw.Name, "id", sw.ID, "penaltyMbps", VPCSwitchTrafficPenaltyMbps, "down", HookFormatTrafficBytes(effectiveDown), "quotaDownGB", sw.TrafficDownGB, "up", HookFormatTrafficBytes(effectiveUp), "quotaUpGB", sw.TrafficUpGB)
	}
}

func CheckAllVPCSwitchTrafficQuota() {
	if model.DB == nil {
		return
	}
	var switches []model.VPCSwitch
	model.DB.Find(&switches)
	for _, sw := range switches {
		CheckAndApplyVPCSwitchTrafficLimit(sw)
	}
}

func CheckVPCSwitchTrafficAfterQuotaUpdate(switchID uint) {
	if switchID == 0 || model.DB == nil {
		return
	}
	var sw model.VPCSwitch
	if err := model.DB.First(&sw, switchID).Error; err != nil {
		return
	}
	rawDown, rawUp := aggregateSwitchMonthlyTrafficRaw(switchID)
	record := getOrCreateVPCSwitchTrafficMonthly(sw, CurrentTrafficMonth())
	effectiveDown := ClampTrafficBytes(rawDown - record.OffsetDown)
	effectiveUp := ClampTrafficBytes(rawUp - record.OffsetUp)
	record.TrafficDown = effectiveDown
	record.TrafficUp = effectiveUp
	if !record.IsLimitedDown && !record.IsLimitedUp {
		if err := saveVPCSwitchTrafficMonthly(record); err != nil {
			logger.App.Warn("VPC交换机月度流量记录失败", "switch", sw.Name, "error", err)
		}
		return
	}
	downLimited := sw.TrafficDownGB > 0 && float64(effectiveDown) >= TrafficQuotaBytes(sw.TrafficDownGB)
	upLimited := sw.TrafficUpGB > 0 && float64(effectiveUp) >= TrafficQuotaBytes(sw.TrafficUpGB)
	if record.IsLimitedDown == downLimited && record.IsLimitedUp == upLimited {
		if err := saveVPCSwitchTrafficMonthly(record); err != nil {
			logger.App.Warn("VPC交换机月度流量记录失败", "switch", sw.Name, "error", err)
		}
		return
	}
	record.IsLimitedDown = downLimited
	record.IsLimitedUp = upLimited
	if err := saveVPCSwitchTrafficMonthly(record); err != nil {
		logger.App.Warn("保存交换机解限状态失败", "switch", sw.Name, "id", sw.ID, "error", err)
		return
	}
	if !downLimited && !upLimited {
		logger.App.Info("配额调整后已低于用量，解除强制限速", "switch", sw.Name, "id", sw.ID)
	}
	if err := ApplyVPCSwitchBandwidth(sw); err != nil {
		logger.App.Warn("配额调整后应用交换机带宽失败", "switch", sw.Name, "id", sw.ID, "error", err)
	}
}

func ResetAllVPCSwitchMonthlyTraffic() {
	if model.DB == nil {
		return
	}
	lastMonth := time.Now().AddDate(0, -1, 0).Format("2006-01")
	var records []model.VPCSwitchTrafficMonthly
	model.DB.Where("month = ? AND (is_limited_down = ? OR is_limited_up = ?)", lastMonth, true, true).Find(&records)
	for _, record := range records {
		var sw model.VPCSwitch
		if err := model.DB.First(&sw, record.SwitchID).Error; err != nil {
			continue
		}
		if err := ApplyVPCSwitchBandwidth(sw); err != nil {
			logger.App.Warn("月重置后恢复交换机带宽失败", "switch", sw.Name, "id", sw.ID, "error", err)
		}
	}
	cleanupMonth := time.Now().AddDate(0, -12, 0).Format("2006-01")
	model.DB.Where("month < ?", cleanupMonth).Delete(&model.VPCSwitchTrafficMonthly{})
}
