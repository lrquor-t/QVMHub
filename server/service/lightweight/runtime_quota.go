package lightweight

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/taskqueue"
	"qvmhub/utils"
)

var lightweightRuntimeQuotaEnforcementState = struct {
	sync.Mutex
	vms map[string]bool
}{
	vms: make(map[string]bool),
}

// InitializeLightweightRuntimeQuotaTracker 初始化轻量云运行时长配额跟踪器。
func InitializeLightweightRuntimeQuotaTracker() {
	SyncAllLightweightVMRuntimeQuotaStates(time.Now())
}

// BuildLightweightVMRuntimeQuotaSnapshot 根据当前单 VM 配额状态构建快照。
func BuildLightweightVMRuntimeQuotaSnapshot(quota *model.LightweightVMQuota, observedAt time.Time) LightweightVMRuntimeQuotaSnapshot {
	if quota == nil {
		return LightweightVMRuntimeQuotaSnapshot{}
	}

	usedSeconds := quota.UsedRuntimeSeconds
	if usedSeconds < 0 {
		usedSeconds = 0
	}
	if quota.RuntimeLastObservedAt != nil && quota.RuntimeIsActive && !observedAt.Before(*quota.RuntimeLastObservedAt) {
		deltaSeconds := int64(observedAt.Sub(*quota.RuntimeLastObservedAt).Seconds())
		if deltaSeconds > 0 {
			usedSeconds += deltaSeconds
		}
	}

	snapshot := LightweightVMRuntimeQuotaSnapshot{UsedSeconds: usedSeconds}
	if quota.MaxRuntimeHours <= 0 {
		return snapshot
	}

	limitSeconds := int64(quota.MaxRuntimeHours) * int64(time.Hour/time.Second)
	remainingSeconds := limitSeconds - usedSeconds
	snapshot.RemainingSeconds = remainingSeconds
	snapshot.QuotaReached = remainingSeconds <= 0
	return snapshot
}

func fillLightweightVMRuntimeSnapshot(quota *model.LightweightVMQuota, observedAt time.Time) {
	if quota == nil {
		return
	}
	snapshot := BuildLightweightVMRuntimeQuotaSnapshot(quota, observedAt)
	quota.UsedRuntimeSeconds = snapshot.UsedSeconds
	quota.UsedRuntimeDisplay = HookFormatRuntimeQuotaDuration(snapshot.UsedSeconds)
	quota.RemainingRuntimeSeconds = snapshot.RemainingSeconds
	quota.RemainingRuntimeDisplay = HookFormatRuntimeQuotaDuration(snapshot.RemainingSeconds)
	quota.RuntimeQuotaReached = snapshot.QuotaReached
}

// CheckLightweightVMRuntimeQuotaAvailable 校验轻量云 VM 是否仍可继续开机。
func CheckLightweightVMRuntimeQuotaAvailable(vmName string) error {
	var quota model.LightweightVMQuota
	if err := model.DB.Where("vm_name = ?", strings.TrimSpace(vmName)).First(&quota).Error; err != nil {
		return nil
	}
	return CheckLightweightVMRuntimeQuotaAvailableForQuota(&quota, time.Now())
}

// CheckLightweightVMRuntimeQuotaAvailableForQuota 校验指定轻量云 VM 配额是否仍可继续开机。
func CheckLightweightVMRuntimeQuotaAvailableForQuota(quota *model.LightweightVMQuota, observedAt time.Time) error {
	if quota == nil || quota.MaxRuntimeHours <= 0 {
		return nil
	}
	snapshot := BuildLightweightVMRuntimeQuotaSnapshot(quota, observedAt)
	if !snapshot.QuotaReached {
		return nil
	}
	return fmt.Errorf("当前轻量云服务器运行时长配额已用尽（已用 %s / 上限 %d 小时），系统已禁止继续开机，请联系管理员调整配额",
		HookFormatRuntimeQuotaDuration(snapshot.UsedSeconds), quota.MaxRuntimeHours)
}

// SyncAllLightweightVMRuntimeQuotaStates 同步全部轻量云 VM 的运行时长配额状态。
func SyncAllLightweightVMRuntimeQuotaStates(observedAt time.Time) {
	activeVMs, err := HookGetRuntimeActiveVMSetFromHost()
	if err != nil {
		logger.App.Warn("获取宿主机运行中虚拟机列表失败", "component", "轻量云运行时长", "error", err)
		return
	}
	SyncAllLightweightVMRuntimeQuotaStatesWithActiveVMs(activeVMs, observedAt)
}

func SyncAllLightweightVMRuntimeQuotaStatesWithActiveVMs(activeVMs map[string]struct{}, observedAt time.Time) {
	if model.DB == nil {
		return
	}
	var quotas []model.LightweightVMQuota
	if err := model.DB.Find(&quotas).Error; err != nil {
		logger.App.Warn("查询配额列表失败", "component", "轻量云运行时长", "error", err)
		return
	}
	for i := range quotas {
		active := isRuntimeVMActive(quotas[i].VMName, activeVMs)
		if err := syncLightweightVMRuntimeQuotaState(&quotas[i], active, observedAt); err != nil {
			logger.App.Warn("同步 VM 配额失败", "component", "轻量云运行时长", "vm", quotas[i].VMName, "error", err)
		}
	}
}

// SyncLightweightVMRuntimeQuotaState 立即同步单台轻量云 VM 的运行时长配额状态。
func SyncLightweightVMRuntimeQuotaState(vmName string, observedAt time.Time) {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" || model.DB == nil {
		return
	}
	var quota model.LightweightVMQuota
	if err := model.DB.Where("vm_name = ?", vmName).First(&quota).Error; err != nil {
		return
	}

	activeVMs, err := HookGetRuntimeActiveVMSetFromHost()
	if err != nil {
		logger.App.Warn("获取 VM 运行状态失败", "component", "轻量云运行时长", "vm", vmName, "error", err)
		return
	}
	active := isRuntimeVMActive(vmName, activeVMs)
	if err := syncLightweightVMRuntimeQuotaState(&quota, active, observedAt); err != nil {
		logger.App.Warn("同步 VM 配额失败", "component", "轻量云运行时长", "vm", vmName, "error", err)
	}
}

func syncLightweightVMRuntimeQuotaState(quota *model.LightweightVMQuota, active bool, observedAt time.Time) error {
	if quota == nil {
		return nil
	}

	snapshot, shouldWarn, err := persistLightweightVMRuntimeQuotaState(quota, active, observedAt)
	if err != nil {
		return err
	}
	if shouldWarn {
		if err := sendLightweightVMRuntimeQuotaWarningEmail(quota, snapshot); err != nil {
			logger.App.Warn("向用户发送 VM 预警邮件失败", "component", "轻量云运行时长", "user", quota.Username, "vm", quota.VMName, "error", err)
		}
	}

	maybeReleaseLightweightRuntimeQuotaEnforcement(quota.VMName, snapshot, active)
	maybeSubmitLightweightRuntimeQuotaShutdownTask(quota, snapshot, active)
	return nil
}

func persistLightweightVMRuntimeQuotaState(quota *model.LightweightVMQuota, active bool, observedAt time.Time) (LightweightVMRuntimeQuotaSnapshot, bool, error) {
	snapshot := BuildLightweightVMRuntimeQuotaSnapshot(quota, observedAt)
	updates := map[string]interface{}{
		"used_runtime_seconds":     snapshot.UsedSeconds,
		"runtime_is_active":        active,
		"runtime_last_observed_at": &observedAt,
	}

	shouldWarn := false
	if quota.MaxRuntimeHours <= 0 {
		if quota.RuntimeWarningSentAt != nil {
			updates["runtime_warning_sent_at"] = nil
			quota.RuntimeWarningSentAt = nil
		}
	} else {
		thresholdSeconds := int64(RuntimeQuotaWarningThreshold / time.Second)
		if snapshot.RemainingSeconds > thresholdSeconds {
			if quota.RuntimeWarningSentAt != nil {
				updates["runtime_warning_sent_at"] = nil
				quota.RuntimeWarningSentAt = nil
			}
		} else if snapshot.RemainingSeconds > 0 && quota.RuntimeWarningSentAt == nil {
			shouldWarn = true
		}
	}

	if err := model.DB.Model(&model.LightweightVMQuota{}).Where("id = ?", quota.ID).Updates(updates).Error; err != nil {
		return LightweightVMRuntimeQuotaSnapshot{}, false, fmt.Errorf("更新轻量云 VM 运行时长状态失败: %w", err)
	}

	quota.UsedRuntimeSeconds = snapshot.UsedSeconds
	quota.RuntimeIsActive = active
	quota.RuntimeLastObservedAt = &observedAt
	fillLightweightVMRuntimeSnapshot(quota, observedAt)
	return snapshot, shouldWarn, nil
}

func sendLightweightVMRuntimeQuotaWarningEmail(quota *model.LightweightVMQuota, snapshot LightweightVMRuntimeQuotaSnapshot) error {
	if quota == nil || strings.TrimSpace(quota.Username) == "" || strings.TrimSpace(quota.VMName) == "" {
		return fmt.Errorf("轻量云 VM 配额信息为空")
	}
	var user model.User
	if err := model.DB.Where("username = ?", quota.Username).First(&user).Error; err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}
	if user.Status != UserStatusActive {
		return nil
	}
	if !HookIsSMTPConfigured() {
		return fmt.Errorf("SMTP 未配置")
	}
	if user.EmailVerifiedAt == nil || strings.TrimSpace(user.Email) == "" {
		return fmt.Errorf("用户未绑定可用邮箱")
	}

	body := fmt.Sprintf(
		"您好，%s：\n\n您的轻量云服务器 %s 运行时长配额即将用尽。\n\n已用时长：%s\n剩余时长：%s\n配额上限：%d 小时\n\n当该服务器运行时长耗尽后，系统会自动关闭这台服务器，并禁止再次开机。\n如需继续使用，请尽快联系管理员调整配额。\n",
		user.Username,
		quota.VMName,
		HookFormatRuntimeQuotaDuration(snapshot.UsedSeconds),
		HookFormatRuntimeQuotaDuration(snapshot.RemainingSeconds),
		quota.MaxRuntimeHours,
	)
	if err := HookSendEmail(user.Email, "轻量云服务器运行时长配额即将耗尽", body); err != nil {
		return err
	}

	now := time.Now()
	if err := model.DB.Model(&model.LightweightVMQuota{}).Where("id = ?", quota.ID).Update("runtime_warning_sent_at", &now).Error; err != nil {
		return fmt.Errorf("记录轻量云 VM 预警邮件发送时间失败: %w", err)
	}
	quota.RuntimeWarningSentAt = &now
	return nil
}

func maybeSubmitLightweightRuntimeQuotaShutdownTask(quota *model.LightweightVMQuota, snapshot LightweightVMRuntimeQuotaSnapshot, active bool) {
	if quota == nil || quota.MaxRuntimeHours <= 0 || !snapshot.QuotaReached || !active {
		return
	}
	if !markLightweightRuntimeQuotaEnforcementInProgress(quota.VMName) {
		return
	}

	task, err := taskqueue.SubmitWithStruct(model.TaskTypeLightweightRuntimeQuotaShutdown, map[string]string{
		"username": quota.Username,
		"vm_name":  quota.VMName,
	}, "system")
	if err != nil {
		releaseLightweightRuntimeQuotaEnforcement(quota.VMName)
		logger.App.Warn("提交 VM 自动关机任务失败", "component", "轻量云运行时长", "vm", quota.VMName, "error", err)
		return
	}

	logger.App.Info("VM 运行时长配额已耗尽，已提交自动关机任务", "component", "轻量云运行时长", "vm", quota.VMName, "task_id", task.ID)
}

func maybeReleaseLightweightRuntimeQuotaEnforcement(vmName string, snapshot LightweightVMRuntimeQuotaSnapshot, active bool) {
	if snapshot.QuotaReached && active {
		return
	}
	releaseLightweightRuntimeQuotaEnforcement(vmName)
}

func markLightweightRuntimeQuotaEnforcementInProgress(vmName string) bool {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" {
		return false
	}
	lightweightRuntimeQuotaEnforcementState.Lock()
	defer lightweightRuntimeQuotaEnforcementState.Unlock()
	if lightweightRuntimeQuotaEnforcementState.vms[vmName] {
		return false
	}
	lightweightRuntimeQuotaEnforcementState.vms[vmName] = true
	return true
}

func releaseLightweightRuntimeQuotaEnforcement(vmName string) {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" {
		return
	}
	lightweightRuntimeQuotaEnforcementState.Lock()
	delete(lightweightRuntimeQuotaEnforcementState.vms, vmName)
	lightweightRuntimeQuotaEnforcementState.Unlock()
}

func isRuntimeVMActive(vmName string, activeVMs map[string]struct{}) bool {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" || activeVMs == nil {
		return false
	}
	_, ok := activeVMs[vmName]
	return ok
}

// EnforceLightweightVMRuntimeQuotaShutdown 执行轻量云单 VM 运行时长配额触发的自动关机。
func EnforceLightweightVMRuntimeQuotaShutdown(vmName string, progressFn func(int, string)) (*LightweightRuntimeQuotaShutdownResult, error) {
	if progressFn == nil {
		progressFn = func(int, string) {}
	}
	vmName = strings.TrimSpace(vmName)
	defer releaseLightweightRuntimeQuotaEnforcement(vmName)

	var quota model.LightweightVMQuota
	if err := model.DB.Where("vm_name = ?", vmName).First(&quota).Error; err != nil {
		return nil, fmt.Errorf("轻量云 VM %s 不存在", vmName)
	}

	result := &LightweightRuntimeQuotaShutdownResult{
		Username: quota.Username,
		VMName:   vmName,
	}
	progressFn(10, fmt.Sprintf("正在关闭轻量云服务器 %s ...", vmName))

	stateResult := utils.ExecCommand("virsh", "domstate", vmName)
	if stateResult.Error != nil {
		return nil, fmt.Errorf("获取虚拟机 %s 状态失败: %s", vmName, HookFirstNonEmpty(stateResult.Stderr, stateResult.Error.Error()))
	}
	state := strings.ToLower(strings.TrimSpace(stateResult.Stdout))
	if state != "" && !strings.Contains(state, "shut off") {
		needForceOff := strings.Contains(state, "paused")
		if !needForceOff {
			if err := HookShutdownVM(vmName); err != nil {
				needForceOff = true
				result.Warnings = append(result.Warnings, "优雅关机失败，已尝试强制断电")
			} else if !HookWaitVMShutdownForDisable(vmName, 40*time.Second) {
				needForceOff = true
				result.Warnings = append(result.Warnings, "等待优雅关机超时，已尝试强制断电")
			}
		}
		if needForceOff {
			if err := HookDestroyVM(vmName); err != nil {
				return nil, fmt.Errorf("关闭虚拟机 %s 失败: %w", vmName, err)
			}
		}
		result.Stopped = true
	}

	SyncLightweightVMRuntimeQuotaState(vmName, time.Now())
	progressFn(100, "轻量云服务器运行时长配额已耗尽，相关虚拟机已关闭")
	return result, nil
}
