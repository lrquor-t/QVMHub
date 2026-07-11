package memory

import (
	"fmt"
	"time"

	"github.com/digitalocean/go-libvirt"
	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/service/libvirt_rpc"
	"qvmhub/utils"
)

type hostMemoryPressure struct {
	TotalKB     int64
	AvailableKB int64
	ReserveKB   int64
	Pressure    bool
}

func registerDynamicMemorySchedulers() {
	DynamicMemorySchedulerRegisterOnce.Do(func() {
		HookMemoryRegisterScheduler(SchedulerDefinition{
			Key:         SchedulerKeyDynamicMemoryBalloon,
			Name:        SchedulerNameDynamicMemoryBalloon,
			Group:       SchedulerGroupDynamicMemory,
			Description: "基于 virtio-balloon 的动态内存自动伸缩调度。",
			Enabled: func() bool {
				return config.GlobalConfig == nil || config.GlobalConfig.DynamicMemorySchedulerEnabled
			},
		})
		HookMemoryRegisterScheduler(SchedulerDefinition{
			Key:         SchedulerKeyDynamicMemoryVirtioMem,
			Name:        SchedulerNameDynamicMemoryVirtioMem,
			Group:       SchedulerGroupDynamicMemory,
			Description: "基于 Windows virtio-mem 的弹性内存自动伸缩调度。",
			Enabled: func() bool {
				return config.GlobalConfig == nil || config.GlobalConfig.DynamicMemorySchedulerEnabled
			},
		})
	})
}

// StartMemoryBalloonScheduler 启动动态内存调度器。
func StartMemoryBalloonScheduler() {
	registerDynamicMemorySchedulers()
	go func() {
		defer utils.RecoverAndLog("memory-balloon-scheduler")
		logger.App.Info("动态内存调度器已启动")
		for {
			interval := 30
			if config.GlobalConfig != nil && config.GlobalConfig.DynamicMemoryIntervalSeconds > 0 {
				interval = config.GlobalConfig.DynamicMemoryIntervalSeconds
			}
			time.Sleep(time.Duration(interval) * time.Second)
			if HookMemoryIsMaintenanceModeEnabled != nil && HookMemoryIsMaintenanceModeEnabled() {
				continue
			}
			if config.GlobalConfig != nil && !config.GlobalConfig.DynamicMemorySchedulerEnabled {
				continue
			}
			runMemoryBalloonScheduleOnce()
		}
	}()
}

func runMemoryBalloonScheduleOnce() {
	domains, err := libvirt_rpc.ListAllDomainsRPC()
	if err != nil {
		return
	}
	host := getHostMemoryPressure()
	for _, dom := range domains {
		l, _ := libvirt_rpc.GetLibvirt()
		if l == nil {
			continue
		}
		state, _, _, _, _, infoErr := l.DomainGetInfo(dom)
		if infoErr != nil || libvirt.DomainState(state) != libvirt.DomainRunning {
			continue
		}
		name := dom.Name
		if name == "" {
			continue
		}
		scheduleVMMemory(name, host)
	}
}

func startMemorySchedulerEvent(schedulerKey, schedulerName, vmName, vmBackend, reason string) interface{} {
	if HookMemoryStartSchedulerEvent == nil {
		return nil
	}
	event, err := HookMemoryStartSchedulerEvent(SchedulerEventStartInput{
		SchedulerKey:   schedulerKey,
		SchedulerName:  schedulerName,
		SchedulerGroup: SchedulerGroupDynamicMemory,
		VMName:         vmName,
		VMBackend:      vmBackend,
		TriggerReason:  reason,
	})
	if err != nil {
		logger.App.Warn("动态内存记录调度事件失败", "error", err)
		return nil
	}
	return event
}

func finishMemorySchedulerEventSuccess(event interface{}, message string) {
	if HookMemoryFinishSchedulerEventOk != nil {
		if err := HookMemoryFinishSchedulerEventOk(event, message); err != nil {
			logger.App.Warn("动态内存更新调度事件成功状态失败", "error", err)
		}
	}
}

func finishMemorySchedulerEventFailed(event interface{}, message string) {
	if HookMemoryFinishSchedulerEventFail != nil {
		if err := HookMemoryFinishSchedulerEventFail(event, message); err != nil {
			logger.App.Warn("动态内存更新调度事件失败状态失败", "error", err)
		}
	}
}

func formatPercent(value float64) string {
	return fmt.Sprintf("%.1f%%", value*100)
}

func buildBalloonExpandReason(usableRatio, threshold float64) string {
	return fmt.Sprintf("可用内存比例 %s 低于增长阈值 %s，触发扩容", formatPercent(usableRatio), formatPercent(threshold))
}

func buildBalloonReclaimReason(unusedRatio, threshold float64) string {
	return fmt.Sprintf("空闲内存比例 %s 高于回收阈值 %s，触发回收", formatPercent(unusedRatio), formatPercent(threshold))
}

func buildVirtioMemExpandReason(usageRatio float64) string {
	return fmt.Sprintf("来宾内存使用率 %s 超过 70.0%%，触发扩容", formatPercent(usageRatio))
}

func buildVirtioMemReclaimReason(usageRatio float64) string {
	return fmt.Sprintf("来宾内存使用率 %s 低于 50.0%%，触发缩容", formatPercent(usageRatio))
}

func buildVirtioMemResultMessage(actualMB, targetMB int, shrink bool) string {
	if shrink {
		return fmt.Sprintf("已请求将弹性内存目标从 %dMB 调整到 %dMB；若当前仍高于目标，表示来宾系统尚未完全释放内存", actualMB, targetMB)
	}
	return fmt.Sprintf("已请求将弹性内存目标从 %dMB 调整到 %dMB", actualMB, targetMB)
}

func getHostMemoryPressure() hostMemoryPressure {
	if HookMemoryGetHostStats == nil {
		return hostMemoryPressure{}
	}
	stats, _ := HookMemoryGetHostStats()
	if stats == nil {
		return hostMemoryPressure{}
	}
	availableKB := stats.MemFree
	if memInfo, err := utils.ReadMemInfo(); err == nil {
		if v, ok := memInfo["MemAvailable"]; ok && v > 0 {
			availableKB = v
		}
	}
	reserveMB := 2048
	reservePercent := 20
	if config.GlobalConfig != nil {
		if config.GlobalConfig.DynamicMemoryHostReserveMB > 0 {
			reserveMB = config.GlobalConfig.DynamicMemoryHostReserveMB
		}
		if config.GlobalConfig.DynamicMemoryHostReservePercent > 0 {
			reservePercent = config.GlobalConfig.DynamicMemoryHostReservePercent
		}
	}
	reserveByPercent := stats.MemTotal * int64(reservePercent) / 100
	reserveByMB := int64(reserveMB) * 1024
	reserveKB := reserveByPercent
	if reserveByMB > reserveKB {
		reserveKB = reserveByMB
	}
	return hostMemoryPressure{
		TotalKB:     stats.MemTotal,
		AvailableKB: availableKB,
		ReserveKB:   reserveKB,
		Pressure:    availableKB < reserveKB,
	}
}

func scheduleVMMemory(name string, host hostMemoryPressure) {
	meta, err := ReadVMMemoryMetadata(name)
	if err != nil || meta == nil || !meta.DynamicEnabled || meta.PendingApply {
		return
	}
	backend := NormalizeMemoryBackend(meta.MemoryBackend)
	if backend == MemoryBackendVirtioMem {
		scheduleVMVirtioMem(name, meta, host)
		return
	}
	if backend != MemoryBackendBalloon || !meta.AutoBalloon {
		return
	}

	now := time.Now()
	if meta.ManualPauseUntil > now.Unix() {
		return
	}

	MemorySchedulerState.Lock()
	lastAdjust := MemorySchedulerState.LastAdjust[name]
	MemorySchedulerState.Unlock()
	cooldownSeconds := 120
	if config.GlobalConfig != nil && config.GlobalConfig.DynamicMemoryCooldownSeconds > 0 {
		cooldownSeconds = config.GlobalConfig.DynamicMemoryCooldownSeconds
	}
	if !lastAdjust.IsZero() && now.Sub(lastAdjust) < time.Duration(cooldownSeconds)*time.Second {
		return
	}

	stats, err := getVMMemoryStats(name)
	if err != nil || stats.ActualKB <= 0 || stats.AvailableKB <= 0 {
		return
	}
	actualMB := int(stats.ActualKB / 1024)
	usedKB := stats.AvailableKB - stats.UnusedKB
	if usedKB < 0 {
		usedKB = 0
	}
	usedMB := int(usedKB / 1024)
	usableRatio := float64(stats.UsableKB) / float64(stats.ActualKB)
	unusedRatio := float64(stats.UnusedKB) / float64(stats.ActualKB)

	increaseThresholdValue := 15
	reclaimThresholdValue := 35
	if config.GlobalConfig != nil {
		increaseThresholdValue = config.GlobalConfig.DynamicMemoryIncreaseThresholdPercent
		reclaimThresholdValue = config.GlobalConfig.DynamicMemoryReclaimThresholdPercent
	}
	increaseThreshold := percentConfig(increaseThresholdValue, 15)
	reclaimThreshold := percentConfig(reclaimThresholdValue, 35)

	MemorySchedulerState.Lock()
	if usableRatio < increaseThreshold {
		MemorySchedulerState.LowUsable[name]++
	} else {
		MemorySchedulerState.LowUsable[name] = 0
	}
	if unusedRatio > reclaimThreshold {
		MemorySchedulerState.HighUnused[name]++
	} else {
		MemorySchedulerState.HighUnused[name] = 0
	}
	lowCount := MemorySchedulerState.LowUsable[name]
	highCount := MemorySchedulerState.HighUnused[name]
	MemorySchedulerState.Unlock()

	if lowCount >= 2 && actualMB < meta.MemoryMaxMB {
		targetMB := MaxInt(int(float64(usedMB)*1.35), int(float64(actualMB)*1.25))
		targetMB = MinInt(MaxInt(targetMB, actualMB+256), meta.MemoryMaxMB)
		reason := buildBalloonExpandReason(usableRatio, increaseThreshold)
		event := startMemorySchedulerEvent(SchedulerKeyDynamicMemoryBalloon, SchedulerNameDynamicMemoryBalloon, name, MemoryBackendBalloon, reason)
		needExtraKB := int64(targetMB-actualMB) * 1024
		if host.AvailableKB-needExtraKB < host.ReserveKB {
			message := fmt.Sprintf("宿主机可用内存不足，目标内存 %dMB，当前可用 %dMB，保留阈值 %dMB", targetMB, host.AvailableKB/1024, host.ReserveKB/1024)
			logger.App.Warn("动态内存跳过增长", "message", message, "vm", name, "targetMB", targetMB)
			finishMemorySchedulerEventFailed(event, message)
			return
		}
		if err := setVMMemoryLive(name, targetMB); err == nil {
			finishMemorySchedulerEventSuccess(event, fmt.Sprintf("已将当前内存从 %dMB 调整到 %dMB", actualMB, targetMB))
			markMemoryAdjusted(name)
		} else {
			finishMemorySchedulerEventFailed(event, err.Error())
		}
		return
	}

	reclaimFloorMB := meta.MemoryInitialMB
	if host.Pressure {
		reclaimFloorMB = meta.MemoryMinMB
	}
	if meta.ObservationUntil > now.Unix() {
		reclaimFloorMB = meta.MemoryInitialMB
	}
	if highCount >= 5 && actualMB > reclaimFloorMB {
		targetMB := MaxInt(int(float64(usedMB)*1.25), reclaimFloorMB)
		targetMB = MinInt(targetMB, actualMB-256)
		if targetMB < actualMB {
			reason := buildBalloonReclaimReason(unusedRatio, reclaimThreshold)
			event := startMemorySchedulerEvent(SchedulerKeyDynamicMemoryBalloon, SchedulerNameDynamicMemoryBalloon, name, MemoryBackendBalloon, reason)
			if err := setVMMemoryLive(name, targetMB); err == nil {
				finishMemorySchedulerEventSuccess(event, fmt.Sprintf("已将当前内存从 %dMB 调整到 %dMB", actualMB, targetMB))
				markMemoryAdjusted(name)
			} else {
				finishMemorySchedulerEventFailed(event, err.Error())
			}
		}
	}
}

func scheduleVMVirtioMem(name string, meta *VMMemoryMetadata, host hostMemoryPressure) {
	now := time.Now()
	if meta.ManualPauseUntil > now.Unix() {
		return
	}

	MemorySchedulerState.Lock()
	lastAdjust := MemorySchedulerState.LastAdjust[name]
	MemorySchedulerState.Unlock()
	cooldownSeconds := 120
	if config.GlobalConfig != nil && config.GlobalConfig.DynamicMemoryCooldownSeconds > 0 {
		cooldownSeconds = config.GlobalConfig.DynamicMemoryCooldownSeconds
	}
	if !lastAdjust.IsZero() && now.Sub(lastAdjust) < time.Duration(cooldownSeconds)*time.Second {
		return
	}

	stats, err := getVMMemoryStats(name)
	if err != nil || stats.ActualKB <= 0 {
		return
	}
	actualMB := int(stats.ActualKB / 1024)
	usedMB := calculateGuestUsedMemoryMB(stats)
	if usedMB <= 0 {
		return
	}

	requestedMB := MaxInt(actualMB-meta.MemoryInitialMB, 0)
	if xmlResult, err := libvirt_rpc.GetDomainXMLRPC(name, 0); err == nil {
		currentVirtioMemMB := ParseVirtioMemCurrentMB(xmlResult)
		requestedMB = parseVirtioMemRequestedMB(xmlResult)
		actualMB = MaxInt(actualMB, meta.MemoryInitialMB+currentVirtioMemMB)
	}
	actualMB = MaxInt(actualMB, meta.MemoryInitialMB)
	actualMB = MinInt(actualMB, meta.MemoryMaxMB)
	targetMB := calculateVirtioMemScheduleTarget(actualMB, usedMB, meta.MemoryInitialMB, meta.MemoryMaxMB)
	if targetMB == actualMB {
		return
	}

	usageRatio := float64(usedMB) / float64(actualMB)
	reason := buildVirtioMemExpandReason(usageRatio)
	if targetMB < actualMB {
		reason = buildVirtioMemReclaimReason(usageRatio)
	}

	targetRequestedMB := MaxInt(targetMB-meta.MemoryInitialMB, 0)
	if targetRequestedMB == requestedMB {
		return
	}

	event := startMemorySchedulerEvent(SchedulerKeyDynamicMemoryVirtioMem, SchedulerNameDynamicMemoryVirtioMem, name, MemoryBackendVirtioMem, reason)

	if targetMB > actualMB {
		needExtraKB := int64(targetMB-actualMB) * 1024
		if host.AvailableKB-needExtraKB < host.ReserveKB {
			message := fmt.Sprintf("宿主机可用内存不足，目标内存 %dMB，当前可用 %dMB，保留阈值 %dMB", targetMB, host.AvailableKB/1024, host.ReserveKB/1024)
			logger.App.Warn("动态内存跳过Windows弹性内存增长", "message", message, "vm", name, "targetMB", targetMB)
			finishMemorySchedulerEventFailed(event, message)
			return
		}
	}
	if err := setVirtioMemRequestedLive(name, targetMB-meta.MemoryInitialMB); err != nil {
		logger.App.Warn("动态内存调整Windows弹性内存失败", "vm", name, "targetMB", targetMB, "error", err)
		finishMemorySchedulerEventFailed(event, err.Error())
		return
	}
	finishMemorySchedulerEventSuccess(event, buildVirtioMemResultMessage(actualMB, targetMB, targetMB < actualMB))
	markMemoryAdjusted(name)
}

func calculateGuestUsedMemoryMB(stats *vmMemoryStatsValues) int {
	if stats == nil || stats.ActualKB <= 0 {
		return 0
	}
	usedKB := stats.ActualKB - stats.UnusedKB
	if stats.AvailableKB > 0 && stats.UsableKB > 0 {
		usedKB = stats.ActualKB - stats.UsableKB
	}
	if usedKB < 0 {
		usedKB = 0
	}
	if usedKB > stats.ActualKB {
		usedKB = stats.ActualKB
	}
	return int(usedKB / 1024)
}

func calculateVirtioMemScheduleTarget(actualMB, usedMB, initialMB, maxMB int) int {
	if actualMB <= 0 || usedMB < 0 || initialMB <= 0 || maxMB <= 0 {
		return actualMB
	}
	actualMB = MaxInt(actualMB, initialMB)
	actualMB = MinInt(actualMB, maxMB)
	usageRatio := float64(usedMB) / float64(actualMB)
	if usageRatio > 0.70 && actualMB < maxMB {
		return MinInt(actualMB+1024, maxMB)
	}
	if usageRatio < 0.50 && actualMB > initialMB {
		targetMB := MaxInt(ceilDivInt(usedMB*100, 70), initialMB)
		if targetMB < actualMB {
			return targetMB
		}
	}
	return actualMB
}

func ceilDivInt(a, b int) int {
	if b <= 0 {
		return 0
	}
	if a <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

func percentConfig(value int, fallback int) float64 {
	if value <= 0 {
		value = fallback
	}
	return float64(value) / 100
}

func setVMMemoryLive(name string, targetMB int) error {
	err := libvirt_rpc.SetDomainMemoryFlagsRPC(name, uint64(targetMB*1024), libvirt.DomainMemoryModFlags(1)) // 1=VIR_DOMAIN_AFFECT_LIVE
	if err != nil {
		logger.App.Warn("动态内存调整当前内存失败", "vm", name, "targetMB", targetMB, "error", err)
		return fmt.Errorf("调整当前内存失败: %w", err)
	}
	logger.App.Info("动态内存已调整当前内存", "vm", name, "targetMB", targetMB)
	return nil
}

func setVirtioMemRequestedLive(name string, requestedMB int) error {
	if requestedMB < 0 {
		requestedMB = 0
	}
	xmlResult, err := libvirt_rpc.GetDomainXMLRPC(name, 0)
	if err != nil {
		return fmt.Errorf("读取运行中虚拟机 XML 失败: %w", err)
	}
	alias := findVirtioMemAlias(xmlResult)
	if alias == "" {
		return fmt.Errorf("未找到 virtio-mem 设备，请确认 Windows 弹性内存已在关机状态下启用")
	}
	result, err := libvirt_rpc.QemuMonitorCommandRPC(name, fmt.Sprintf("setvmstate virtio-mem %s requested-size %d", alias, requestedMB*1024), libvirt_rpc.DomainQemuMonitorCommandHmp)
	if err != nil {
		return fmt.Errorf("调整 Windows 弹性内存失败: %w", err)
	}
	_ = result
	logger.App.Info("动态内存已调整virtio-mem requested", "vm", name, "requestedMB", requestedMB)
	return nil
}

func markMemoryAdjusted(name string) {
	MemorySchedulerState.Lock()
	MemorySchedulerState.LastAdjust[name] = time.Now()
	MemorySchedulerState.LowUsable[name] = 0
	MemorySchedulerState.HighUnused[name] = 0
	MemorySchedulerState.Unlock()
}
