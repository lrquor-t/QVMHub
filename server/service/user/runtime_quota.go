package user

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/service/security"
	"qvmhub/taskqueue"
	"qvmhub/utils"
)

const RuntimeQuotaWarningThreshold = 2 * time.Hour

var runtimeQuotaEnforcementState = struct {
	sync.Mutex
	users map[string]bool
}{
	users: make(map[string]bool),
}

// InitializeUserRuntimeQuotaTracker 初始化用户运行时长配额跟踪器
func InitializeUserRuntimeQuotaTracker() {
	SyncAllUserRuntimeQuotaStates(time.Now())
}

// BuildUserRuntimeQuotaSnapshot 根据当前用户状态构建运行时长配额快照
func BuildUserRuntimeQuotaSnapshot(user *model.User, observedAt time.Time) UserRuntimeQuotaSnapshot {
	if user == nil {
		return UserRuntimeQuotaSnapshot{}
	}

	usedSeconds := user.UsedRuntimeSeconds
	if usedSeconds < 0 {
		usedSeconds = 0
	}

	if user.RuntimeLastObservedAt != nil && !observedAt.Before(*user.RuntimeLastObservedAt) && user.RuntimeActiveVMCount > 0 {
		deltaSeconds := int64(observedAt.Sub(*user.RuntimeLastObservedAt).Seconds())
		if deltaSeconds > 0 {
			usedSeconds += deltaSeconds * int64(user.RuntimeActiveVMCount)
		}
	}

	snapshot := UserRuntimeQuotaSnapshot{UsedSeconds: usedSeconds}
	if user.MaxRuntimeHours <= 0 {
		return snapshot
	}

	limitSeconds := int64(user.MaxRuntimeHours) * int64(time.Hour/time.Second)
	remainingSeconds := limitSeconds - usedSeconds
	snapshot.RemainingSeconds = remainingSeconds
	snapshot.QuotaReached = remainingSeconds <= 0
	return snapshot
}

// FormatRuntimeQuotaDuration 将秒数格式化为便于展示的中文时长
func FormatRuntimeQuotaDuration(seconds int64) string {
	if seconds <= 0 {
		return "0秒"
	}

	days := seconds / 86400
	seconds %= 86400
	hours := seconds / 3600
	seconds %= 3600
	minutes := seconds / 60

	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%d天", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d小时", hours))
	}
	if minutes > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d分钟", minutes))
	}
	return strings.Join(parts, " ")
}

// CheckRuntimeQuotaAvailable 校验用户是否仍可继续开机
func CheckRuntimeQuotaAvailable(username string) error {
	var user model.User
	if err := model.DB.Where("username = ?", strings.TrimSpace(username)).First(&user).Error; err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}
	return CheckRuntimeQuotaAvailableForUser(&user, time.Now())
}

// CheckRuntimeQuotaAvailableForUser 校验指定用户是否仍可继续开机
func CheckRuntimeQuotaAvailableForUser(user *model.User, observedAt time.Time) error {
	if user == nil || user.Role == "admin" || user.MaxRuntimeHours <= 0 {
		return nil
	}

	snapshot := BuildUserRuntimeQuotaSnapshot(user, observedAt)
	if !snapshot.QuotaReached {
		return nil
	}

	return fmt.Errorf("总运行时长配额已用尽（已用 %s / 上限 %d 小时），系统已禁止继续开机，请联系管理员调整配额",
		FormatRuntimeQuotaDuration(snapshot.UsedSeconds), user.MaxRuntimeHours)
}

// SyncAllUserRuntimeQuotaStates 同步所有普通用户的总运行时长配额状态
func SyncAllUserRuntimeQuotaStates(observedAt time.Time) {
	activeVMs, err := GetRuntimeActiveVMSetFromHost()
	if err != nil {
		logger.App.Warn("获取宿主机运行中虚拟机列表失败", "component", "运行时长", "error", err)
		return
	}
	SyncAllUserRuntimeQuotaStatesWithActiveVMs(activeVMs, observedAt)
}

// SyncAllUserRuntimeQuotaStatesWithActiveVMs 使用已获取的运行态 VM 集合同步用户总运行时长配额状态。
func SyncAllUserRuntimeQuotaStatesWithActiveVMs(activeVMs map[string]struct{}, observedAt time.Time) {
	var users []model.User
	if err := model.DB.Where("role = ?", "user").Find(&users).Error; err != nil {
		logger.App.Warn("查询用户列表失败", "component", "运行时长", "error", err)
		return
	}

	for _, user := range users {
		activeCount := countUserActiveVMs(user.Username, activeVMs)
		if err := syncUserRuntimeQuotaState(&user, activeCount, observedAt); err != nil {
			logger.App.Warn("同步用户配额失败", "component", "运行时长", "user", user.Username, "error", err)
		}
	}
}

// SyncUserRuntimeQuotaState 立即同步单个用户的总运行时长配额状态
func SyncUserRuntimeQuotaState(username string, observedAt time.Time) {
	username = strings.TrimSpace(username)
	if username == "" {
		return
	}

	var user model.User
	if err := model.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return
	}
	if user.Role == "admin" {
		return
	}

	activeVMs, err := GetRuntimeActiveVMSetFromHost()
	if err != nil {
		logger.App.Warn("获取用户运行中虚拟机列表失败", "component", "运行时长", "user", username, "error", err)
		return
	}

	activeCount := countUserActiveVMs(username, activeVMs)
	if err := syncUserRuntimeQuotaState(&user, activeCount, observedAt); err != nil {
		logger.App.Warn("同步用户配额失败", "component", "运行时长", "user", username, "error", err)
	}
}

func syncUserRuntimeQuotaState(user *model.User, activeCount int, observedAt time.Time) error {
	if user == nil {
		return nil
	}

	snapshot, shouldWarn, err := persistUserRuntimeQuotaState(user, activeCount, observedAt)
	if err != nil {
		return err
	}

	if shouldWarn {
		if err := sendRuntimeQuotaWarningEmail(user, snapshot); err != nil {
			logger.App.Warn("向用户发送配额预警邮件失败", "component", "运行时长", "user", user.Username, "error", err)
		}
	}

	maybeReleaseRuntimeQuotaEnforcement(user.Username, snapshot, activeCount)
	maybeSubmitRuntimeQuotaShutdownTask(user, snapshot, activeCount)
	return nil
}

func persistUserRuntimeQuotaState(user *model.User, activeCount int, observedAt time.Time) (UserRuntimeQuotaSnapshot, bool, error) {
	snapshot := BuildUserRuntimeQuotaSnapshot(user, observedAt)
	updates := map[string]interface{}{
		"used_runtime_seconds":     snapshot.UsedSeconds,
		"runtime_active_vm_count":  activeCount,
		"runtime_last_observed_at": &observedAt,
	}

	shouldWarn := false
	if user.MaxRuntimeHours <= 0 {
		if user.RuntimeWarningSentAt != nil {
			updates["runtime_warning_sent_at"] = nil
			user.RuntimeWarningSentAt = nil
		}
	} else {
		thresholdSeconds := int64(RuntimeQuotaWarningThreshold / time.Second)
		if snapshot.RemainingSeconds > thresholdSeconds {
			if user.RuntimeWarningSentAt != nil {
				updates["runtime_warning_sent_at"] = nil
				user.RuntimeWarningSentAt = nil
			}
		} else if snapshot.RemainingSeconds > 0 && user.RuntimeWarningSentAt == nil {
			shouldWarn = true
		}
	}

	if err := model.DB.Model(&model.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
		return UserRuntimeQuotaSnapshot{}, false, fmt.Errorf("更新用户运行时长状态失败: %w", err)
	}

	user.UsedRuntimeSeconds = snapshot.UsedSeconds
	user.RuntimeActiveVMCount = activeCount
	user.RuntimeLastObservedAt = &observedAt
	return snapshot, shouldWarn, nil
}

func GetRuntimeActiveVMSetFromHost() (map[string]struct{}, error) {
	activeVMs := make(map[string]struct{})
	for _, args := range [][]string{
		{"list", "--name", "--state-running"},
		{"list", "--name", "--state-paused"},
	} {
		result := utils.ExecCommand("virsh", args...)
		if result.Error != nil {
			message := strings.TrimSpace(result.Stderr)
			if message == "" {
				message = result.Error.Error()
			}
			if HookIsMaintenanceModeEnabled() && (HookIsLibvirtUnavailableText(message) || HookIsLibvirtUnavailableError(result.Error)) {
				return activeVMs, nil
			}
			return nil, fmt.Errorf("%s", message)
		}
		for _, line := range strings.Split(result.Stdout, "\n") {
			name := strings.TrimSpace(line)
			if name == "" {
				continue
			}
			activeVMs[name] = struct{}{}
		}
	}
	return activeVMs, nil
}

func countUserActiveVMs(username string, activeVMs map[string]struct{}) int {
	count := 0
	for _, vmName := range GetUserVMList(username) {
		if _, ok := activeVMs[vmName]; ok {
			count++
		}
	}
	return count
}

func sendRuntimeQuotaWarningEmail(user *model.User, snapshot UserRuntimeQuotaSnapshot) error {
	if user == nil || strings.TrimSpace(user.Username) == "" {
		return fmt.Errorf("用户信息为空")
	}
	if user.Status != security.UserStatusActive {
		return nil
	}

	body := fmt.Sprintf(
		"您好，%s：\n\n您的账户总运行时长配额即将用尽。\n\n已用时长：%s\n剩余时长：%s\n配额上限：%d 小时\n\n当总运行时长耗尽后，系统会自动关闭您当前运行中的全部虚拟机，并禁止再次开机。\n如需继续使用，请尽快联系管理员调整配额。\n",
		user.Username,
		FormatRuntimeQuotaDuration(snapshot.UsedSeconds),
		FormatRuntimeQuotaDuration(snapshot.RemainingSeconds),
		user.MaxRuntimeHours,
	)
	if err := HookSendEmail(user.Email, "总运行时长配额即将耗尽", body); err != nil {
		return err
	}

	now := time.Now()
	if err := model.DB.Model(&model.User{}).Where("id = ?", user.ID).Update("runtime_warning_sent_at", &now).Error; err != nil {
		return fmt.Errorf("记录预警邮件发送时间失败: %w", err)
	}
	user.RuntimeWarningSentAt = &now
	return nil
}

func maybeSubmitRuntimeQuotaShutdownTask(user *model.User, snapshot UserRuntimeQuotaSnapshot, activeCount int) {
	if user == nil || user.MaxRuntimeHours <= 0 || !snapshot.QuotaReached || activeCount <= 0 {
		return
	}
	if user.Status != security.UserStatusActive {
		return
	}

	if !markRuntimeQuotaEnforcementInProgress(user.Username) {
		return
	}

	task, err := taskqueue.SubmitWithStruct(model.TaskTypeRuntimeQuotaShutdown, map[string]string{
		"username": user.Username,
	}, "system")
	if err != nil {
		releaseRuntimeQuotaEnforcement(user.Username)
		logger.App.Warn("提交用户自动关机任务失败", "component", "运行时长", "user", user.Username, "error", err)
		return
	}

	logger.App.Info("用户总运行时长配额已耗尽，已提交自动关机任务", "component", "运行时长", "user", user.Username, "task_id", task.ID)
}

func maybeReleaseRuntimeQuotaEnforcement(username string, snapshot UserRuntimeQuotaSnapshot, activeCount int) {
	if snapshot.QuotaReached && activeCount > 0 {
		return
	}
	releaseRuntimeQuotaEnforcement(username)
}

func markRuntimeQuotaEnforcementInProgress(username string) bool {
	username = strings.TrimSpace(username)
	if username == "" {
		return false
	}

	runtimeQuotaEnforcementState.Lock()
	defer runtimeQuotaEnforcementState.Unlock()
	if runtimeQuotaEnforcementState.users[username] {
		return false
	}
	runtimeQuotaEnforcementState.users[username] = true
	return true
}

func releaseRuntimeQuotaEnforcement(username string) {
	username = strings.TrimSpace(username)
	if username == "" {
		return
	}

	runtimeQuotaEnforcementState.Lock()
	delete(runtimeQuotaEnforcementState.users, username)
	runtimeQuotaEnforcementState.Unlock()
}

// EnforceUserRuntimeQuotaShutdown 执行运行时长配额触发的自动关机
func EnforceUserRuntimeQuotaShutdown(username string, progressFn func(int, string)) (*RuntimeQuotaShutdownResult, error) {
	if progressFn == nil {
		progressFn = func(int, string) {}
	}
	defer releaseRuntimeQuotaEnforcement(username)

	var user model.User
	if err := model.DB.Where("username = ?", strings.TrimSpace(username)).First(&user).Error; err != nil {
		return nil, fmt.Errorf("用户 %s 不存在", username)
	}
	if user.Role == "admin" {
		return nil, fmt.Errorf("管理员不受总运行时长配额限制")
	}

	result := &RuntimeQuotaShutdownResult{Username: username}
	progressFn(10, fmt.Sprintf("正在关闭用户 %s 的运行中虚拟机...", username))
	result.StoppedVMs, result.Warnings = stopUserVMsForDisable(username, result.Warnings)

	SyncUserRuntimeQuotaState(username, time.Now())
	progressFn(100, "总运行时长配额已耗尽，相关虚拟机已关闭")
	return result, nil
}
