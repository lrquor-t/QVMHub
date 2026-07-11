package lightweight

import (
	"fmt"
	"strings"
	"time"

	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/service/snapshot"
)

func NormalizeLightweightVMQuotaRequest(req LightweightVMQuotaRequest) LightweightVMQuotaRequest {
	req.VMName = strings.TrimSpace(req.VMName)
	if req.TrafficDownGB < 0 {
		req.TrafficDownGB = 0
	}
	if req.TrafficUpGB < 0 {
		req.TrafficUpGB = 0
	}
	if req.BandwidthDownMbps < 0 {
		req.BandwidthDownMbps = 0
	}
	if req.BandwidthUpMbps < 0 {
		req.BandwidthUpMbps = 0
	}
	if req.MaxPortForwards < 0 {
		req.MaxPortForwards = 0
	}
	if req.MaxSnapshots < 0 {
		req.MaxSnapshots = 0
	}
	if req.MaxRuntimeHours < 0 {
		req.MaxRuntimeHours = 0
	}
	return req
}

func DefaultLightweightVMQuota(vmName string) LightweightVMQuotaRequest {
	return LightweightVMQuotaRequest{
		VMName:          vmName,
		MaxPortForwards: 10,
		MaxSnapshots:    2,
	}
}

func UpsertLightweightVMQuota(username string, req LightweightVMQuotaRequest) (*model.LightweightVMQuota, error) {
	username = strings.TrimSpace(username)
	req = NormalizeLightweightVMQuotaRequest(req)
	if username == "" {
		return nil, fmt.Errorf("用户不能为空")
	}
	if req.VMName == "" {
		return nil, fmt.Errorf("虚拟机名称不能为空")
	}
	var quota model.LightweightVMQuota
	err := model.DB.Where("vm_name = ?", req.VMName).First(&quota).Error
	if err == nil {
		quota.Username = username
		quota.TrafficDownGB = req.TrafficDownGB
		quota.TrafficUpGB = req.TrafficUpGB
		quota.BandwidthDownMbps = req.BandwidthDownMbps
		quota.BandwidthUpMbps = req.BandwidthUpMbps
		quota.MaxPortForwards = req.MaxPortForwards
		quota.MaxSnapshots = req.MaxSnapshots
		quota.MaxRuntimeHours = req.MaxRuntimeHours
		if err := model.DB.Save(&quota).Error; err != nil {
			return nil, err
		}
		CheckLightweightVMTrafficAfterQuotaUpdate(req.VMName)
		SyncLightweightVMRuntimeQuotaState(req.VMName, time.Now())
		HookRefreshVMCacheByNameAsync(req.VMName)
		return FillLightweightVMQuotaRuntime(&quota), nil
	}
	quota = model.LightweightVMQuota{
		Username:          username,
		VMName:            req.VMName,
		TrafficDownGB:     req.TrafficDownGB,
		TrafficUpGB:       req.TrafficUpGB,
		BandwidthDownMbps: req.BandwidthDownMbps,
		BandwidthUpMbps:   req.BandwidthUpMbps,
		MaxPortForwards:   req.MaxPortForwards,
		MaxSnapshots:      req.MaxSnapshots,
		MaxRuntimeHours:   req.MaxRuntimeHours,
	}
	if err := model.DB.Create(&quota).Error; err != nil {
		return nil, err
	}
	if err := ApplyLightweightVMBandwidth(req.VMName); err != nil {
		logger.App.Warn("应用 VM 带宽失败", "component", "轻量云", "vm", req.VMName, "error", err)
	}
	SyncLightweightVMRuntimeQuotaState(req.VMName, time.Now())
	HookRefreshVMCacheByNameAsync(req.VMName)
	return FillLightweightVMQuotaRuntime(&quota), nil
}

func GetLightweightVMQuota(vmName string) (*model.LightweightVMQuota, error) {
	var quota model.LightweightVMQuota
	if err := model.DB.Where("vm_name = ?", strings.TrimSpace(vmName)).First(&quota).Error; err != nil {
		return nil, err
	}
	return FillLightweightVMQuotaRuntime(&quota), nil
}

func FillLightweightVMQuotaRuntime(quota *model.LightweightVMQuota) *model.LightweightVMQuota {
	if quota == nil {
		return nil
	}
	down, up := AggregateLightweightVMMonthlyTraffic(quota.VMName)
	quota.UsedTrafficDown = down
	quota.UsedTrafficUp = up
	quota.UsedTrafficDownGB = HookFormatTrafficBytes(down)
	quota.UsedTrafficUpGB = HookFormatTrafficBytes(up)
	quota.IsLimitedDown, quota.IsLimitedUp = IsLightweightVMTrafficLimited(quota.VMName)
	quota.UsedPortForwards = GetLightweightVMPortForwardUsage(quota.VMName)
	quota.UsedSnapshots = snapshot.CountVMSnapshots(quota.VMName)
	fillLightweightVMRuntimeSnapshot(quota, time.Now())
	fillLightweightVMNICRuntime(quota)
	return quota
}

func fillLightweightVMNICRuntime(quota *model.LightweightVMQuota) {
	if quota == nil || model.DB == nil || strings.TrimSpace(quota.VMName) == "" {
		return
	}
	var records []model.VmStatsRecord
	model.DB.Where("vm_name = ?", strings.TrimSpace(quota.VMName)).
		Order("recorded_at DESC").
		Limit(2).
		Find(&records)
	if len(records) == 0 {
		quota.CurrentNetRxRate = "0 B/s"
		quota.CurrentNetTxRate = "0 B/s"
		return
	}
	latest := records[0]
	quota.CurrentNetRxBytes = latest.NetRxBytes
	quota.CurrentNetTxBytes = latest.NetTxBytes
	quota.CurrentNetRxRate = "0 B/s"
	quota.CurrentNetTxRate = "0 B/s"
	if len(records) < 2 {
		return
	}
	prev := records[1]
	seconds := latest.RecordedAt.Sub(prev.RecordedAt).Seconds()
	if seconds <= 0 {
		return
	}
	if delta := latest.NetRxBytes - prev.NetRxBytes; delta > 0 {
		quota.CurrentNetRxRate = formatTrafficRate(float64(delta) / seconds)
	}
	if delta := latest.NetTxBytes - prev.NetTxBytes; delta > 0 {
		quota.CurrentNetTxRate = formatTrafficRate(float64(delta) / seconds)
	}
}

func formatTrafficRate(bytesPerSecond float64) string {
	if bytesPerSecond <= 0 {
		return "0 B/s"
	}
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytesPerSecond >= GB:
		return fmt.Sprintf("%.2f GB/s", bytesPerSecond/GB)
	case bytesPerSecond >= MB:
		return fmt.Sprintf("%.2f MB/s", bytesPerSecond/MB)
	case bytesPerSecond >= KB:
		return fmt.Sprintf("%.2f KB/s", bytesPerSecond/KB)
	default:
		return fmt.Sprintf("%.0f B/s", bytesPerSecond)
	}
}
