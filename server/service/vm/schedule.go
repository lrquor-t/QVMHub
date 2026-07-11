package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/taskqueue"
	"qvmhub/utils"
)

const (
	VMScheduleEventTypePower     = "power"
	VMScheduleEventTypeLifecycle = "vm"

	VMScheduleActionStart    = "start"
	VMScheduleActionShutdown = "shutdown"
	VMScheduleActionDelete   = "delete"

	VMScheduleTypeOnce   = "once"
	VMScheduleTypeDaily  = "daily"
	VMScheduleTypeWeekly = "weekly"

	VMScheduleExecStatusPending = "pending"
	VMScheduleExecStatusRunning = "running"
	VMScheduleExecStatusSuccess = "success"
	VMScheduleExecStatusFailed  = "failed"

	vmScheduleSchedulerKey   = "vm_scheduled_task"
	vmScheduleSchedulerName  = "虚拟机定时任务"
	vmScheduleSchedulerGroup = "虚拟机管理"
)

var vmScheduleRunnerOnce sync.Once
var vmScheduleRegisterOnce sync.Once

// VMScheduleInput 定时任务写入参数。
type VMScheduleInput struct {
	EventType    string `json:"event_type"`
	Action       string `json:"action"`
	ScheduleType string `json:"schedule_type"`
	RunAt        string `json:"run_at"`
	Timezone     string `json:"timezone"`
	TimeOfDay    string `json:"time_of_day"`
	Weekdays     []int  `json:"weekdays"`
	Enabled      *bool  `json:"enabled"`
}

// VMScheduleItem 定时任务展示数据。
type VMScheduleItem struct {
	ID              uint       `json:"id"`
	VMName          string     `json:"vm_name"`
	EventType       string     `json:"event_type"`
	Action          string     `json:"action"`
	ScheduleType    string     `json:"schedule_type"`
	RunAt           *time.Time `json:"run_at"`
	Timezone        string     `json:"timezone"`
	TimeOfDay       string     `json:"time_of_day"`
	Weekdays        []int      `json:"weekdays"`
	Enabled         bool       `json:"enabled"`
	NextRunAt       *time.Time `json:"next_run_at"`
	LastTaskID      *uint      `json:"last_task_id"`
	LastTriggeredAt *time.Time `json:"last_triggered_at"`
	LastFinishedAt  *time.Time `json:"last_finished_at"`
	LastStatus      string     `json:"last_status"`
	LastMessage     string     `json:"last_message"`
	CreatedBy       string     `json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// VMScheduledActionTaskParams 定时任务触发后的执行参数。
type VMScheduledActionTaskParams struct {
	ScheduleID uint   `json:"schedule_id"`
	VMName     string `json:"vm_name"`
	EventType  string `json:"event_type"`
	Action     string `json:"action"`
}

// registerVMScheduleScheduler 注册虚拟机定时任务调度器（延迟到 StartVMScheduleRunner 中调用，
// 避免在 init() 阶段 D 尚未注入导致 nil pointer）。
func registerVMScheduleScheduler() {
	vmScheduleRegisterOnce.Do(func() {
		D.RegisterScheduler(SchedulerDefinition{
			Key:         vmScheduleSchedulerKey,
			Name:        vmScheduleSchedulerName,
			Group:       vmScheduleSchedulerGroup,
			Description: "按虚拟机详情中的定时任务配置执行开机、关机和删除动作",
			Enabled: func() bool {
				return true
			},
		})
	})
}

// ListVMSchedules 获取虚拟机定时任务列表。
func ListVMSchedules(vmName string) ([]VMScheduleItem, error) {
	list, err := model.ListVMSchedulesByVM(strings.TrimSpace(vmName))
	if err != nil {
		return nil, err
	}
	items := make([]VMScheduleItem, 0, len(list))
	for _, item := range list {
		items = append(items, buildVMScheduleItem(item))
	}
	return items, nil
}

// CreateVMSchedule 创建定时任务。
func CreateVMSchedule(vmName, createdBy string, input VMScheduleInput) (*VMScheduleItem, error) {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" {
		return nil, fmt.Errorf("虚拟机名称不能为空")
	}
	if _, err := GetVM(vmName); err != nil {
		return nil, err
	}
	schedule, err := buildVMScheduleModel(nil, vmName, createdBy, input)
	if err != nil {
		return nil, err
	}
	if err := model.CreateVMSchedule(schedule); err != nil {
		return nil, err
	}
	item := buildVMScheduleItem(*schedule)
	return &item, nil
}

// UpdateVMSchedule 更新定时任务。
func UpdateVMSchedule(vmName string, id uint, input VMScheduleInput) (*VMScheduleItem, error) {
	existing, err := model.GetVMScheduleByIDAndVM(id, strings.TrimSpace(vmName))
	if err != nil {
		return nil, err
	}
	schedule, err := buildVMScheduleModel(existing, existing.VMName, existing.CreatedBy, input)
	if err != nil {
		return nil, err
	}
	schedule.ID = existing.ID
	schedule.CreatedAt = existing.CreatedAt
	schedule.LastTaskID = existing.LastTaskID
	schedule.LastTriggeredAt = existing.LastTriggeredAt
	schedule.LastFinishedAt = existing.LastFinishedAt
	schedule.LastStatus = existing.LastStatus
	schedule.LastMessage = existing.LastMessage
	if err := model.UpdateVMSchedule(schedule); err != nil {
		return nil, err
	}
	item := buildVMScheduleItem(*schedule)
	return &item, nil
}

// DeleteVMSchedule 删除定时任务。
func DeleteVMSchedule(vmName string, id uint) error {
	schedule, err := model.GetVMScheduleByIDAndVM(id, strings.TrimSpace(vmName))
	if err != nil {
		return err
	}
	return model.DeleteVMSchedule(schedule.ID)
}

// DeleteVMSchedules 删除虚拟机关联的全部定时任务。
func DeleteVMSchedules(vmName string) error {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" {
		return nil
	}
	return model.DeleteVMSchedulesByVM(vmName)
}

// StartVMScheduleRunner 启动后台扫描器。
func StartVMScheduleRunner() {
	registerVMScheduleScheduler()
	vmScheduleRunnerOnce.Do(func() {
		go func() {
			defer utils.RecoverAndLog("vm-schedule-runner")
			runDueVMSchedulesOnce()
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				runDueVMSchedulesOnce()
			}
		}()
	})
}

func runDueVMSchedulesOnce() {
	now := time.Now()
	dueList, err := model.ListDueVMSchedules(now, 20)
	if err != nil {
		logger.App.Warn("扫描虚拟机定时任务失败", "error", err)
		return
	}
	for _, schedule := range dueList {
		if err := queueDueVMSchedule(schedule, now); err != nil {
			logger.App.Warn("提交虚拟机定时任务失败", "schedule_id", schedule.ID, "vm", schedule.VMName, "error", err)
		}
	}
}

func queueDueVMSchedule(schedule model.VMSchedule, now time.Time) error {
	params := VMScheduledActionTaskParams{
		ScheduleID: schedule.ID,
		VMName:     schedule.VMName,
		EventType:  schedule.EventType,
		Action:     schedule.Action,
	}
	createdBy := strings.TrimSpace(schedule.CreatedBy)
	if createdBy == "" {
		createdBy = "system:scheduler"
	}
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeVMScheduleAction, params, createdBy)
	if err != nil {
		_ = model.UpdateVMScheduleFields(schedule.ID, map[string]interface{}{
			"last_status":  VMScheduleExecStatusFailed,
			"last_message": "提交任务队列失败: " + err.Error(),
		})
		return err
	}

	// 将触发时间转换为定时任务配置的时区，确保前后端显示一致
	triggerTime := now
	if loc, _, locErr := resolveScheduleLocation(schedule.Timezone); locErr == nil {
		triggerTime = now.In(loc)
	}

	updates := map[string]interface{}{
		"last_task_id":      task.ID,
		"last_triggered_at": triggerTime,
		"last_status":       VMScheduleExecStatusPending,
		"last_message":      fmt.Sprintf("已提交任务中心，任务 #%d 等待执行", task.ID),
	}
	if schedule.ScheduleType == VMScheduleTypeOnce {
		updates["enabled"] = false
		updates["next_run_at"] = nil
	} else {
		nextBase := now
		if schedule.NextRunAt != nil && schedule.NextRunAt.After(nextBase) {
			nextBase = *schedule.NextRunAt
		}
		if nextRun, nextErr := calculateNextRunForModel(schedule, &nextBase); nextErr == nil {
			updates["next_run_at"] = nextRun
		}
	}
	return model.UpdateVMScheduleFields(schedule.ID, updates)
}

// RunVMScheduledAction 执行单个定时任务动作。
func RunVMScheduledAction(ctx context.Context, task *model.Task, progress func(int, string)) (string, error) {
	var params VMScheduledActionTaskParams
	if err := json.Unmarshal([]byte(task.Params), &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", taskqueue.ErrTaskCanceled
	}

	markScheduleRunning(params.ScheduleID, task.ID)

	event, _ := D.StartSchedulerEvent(SchedulerEventStartInput{
		SchedulerKey:   vmScheduleSchedulerKey,
		SchedulerName:  vmScheduleSchedulerName,
		SchedulerGroup: vmScheduleSchedulerGroup,
		VMName:         params.VMName,
		VMBackend:      params.EventType,
		TriggerReason:  buildScheduleTriggerReason(params),
	})

	progress(15, "开始执行虚拟机定时任务")
	result, resultMessage, err := executeScheduledVMAction(ctx, params, progress)
	if err != nil {
		finishScheduleExecution(params.ScheduleID, VMScheduleExecStatusFailed, err.Error())
		if event != nil {
			_ = D.FinishSchedulerEventFailed(event, err.Error())
		}
		return result, err
	}

	finishScheduleExecution(params.ScheduleID, VMScheduleExecStatusSuccess, resultMessage)
	if event != nil {
		_ = D.FinishSchedulerEventSuccess(event, resultMessage)
	}
	return result, nil
}

func executeScheduledVMAction(ctx context.Context, params VMScheduledActionTaskParams, progress func(int, string)) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", taskqueue.ErrTaskCanceled
	}

	vm, vmErr := GetVM(params.VMName)
	if params.Action != VMScheduleActionDelete && vmErr != nil {
		return "", "", fmt.Errorf("获取虚拟机状态失败: %w", vmErr)
	}

	owner := strings.TrimSpace(D.FindVMOwner(params.VMName))

	switch params.Action {
	case VMScheduleActionStart:
		if vm != nil && vm.Status == "running" {
			message := "虚拟机当前已在运行，已跳过本次定时开机"
			progress(100, message)
			return buildScheduledActionResult(params, true, message), message, nil
		}
		progress(40, "正在执行定时开机")
		if err := StartVM(params.VMName); err != nil {
			return "", "", err
		}
		if owner != "" && owner != "admin" {
			go func(username string) {
				defer utils.RecoverAndLog("vm-schedule-rebalance")
				if err := D.RebalanceUserBandwidth(username); err != nil {
					logger.App.Warn("定时开机后重新分配用户带宽失败", "user", username, "error", err)
				}
			}(owner)
		}
		message := "虚拟机开机指令已下发"
		progress(100, message)
		return buildScheduledActionResult(params, false, message), message, nil

	case VMScheduleActionShutdown:
		if vm != nil && vm.Status == "shut off" {
			message := "虚拟机当前已关机，已跳过本次定时关机"
			progress(100, message)
			return buildScheduledActionResult(params, true, message), message, nil
		}
		progress(40, "正在执行定时关机")
		if err := ShutdownVM(params.VMName); err != nil {
			return "", "", err
		}
		message := "虚拟机关机指令已下发"
		progress(100, message)
		return buildScheduledActionResult(params, false, message), message, nil

	case VMScheduleActionDelete:
		if vmErr != nil {
			if D.IsLibvirtUnavailableError(vmErr) {
				return "", "", fmt.Errorf("获取虚拟机状态失败: %w", vmErr)
			}
			message := "虚拟机不存在，视为删除完成"
			progress(100, message)
			return buildScheduledActionResult(params, true, message), message, nil
		}
		if IsVMLocked(params.VMName) {
			return "", "", fmt.Errorf("虚拟机已锁定，无法执行定时删除")
		}
		progress(35, "正在删除虚拟机")
		if err := D.DeleteVM(params.VMName); err != nil {
			return "", "", err
		}
		_ = DeleteVMCredential(params.VMName)
		_ = model.DeleteVMLock(params.VMName)
		if owner != "" && owner != "admin" {
			go func(username string) {
				defer utils.RecoverAndLog("vm-schedule-delete-rebalance")
				if err := D.RebalanceUserBandwidth(username); err != nil {
					logger.App.Warn("定时删除虚拟机后重新分配用户带宽失败", "user", username, "error", err)
				}
			}(owner)
		}
		message := "虚拟机已删除"
		progress(100, message)
		return buildScheduledActionResult(params, false, message), message, nil
	default:
		return "", "", fmt.Errorf("不支持的定时任务动作: %s", params.Action)
	}
}

func buildScheduledActionResult(params VMScheduledActionTaskParams, skipped bool, message string) string {
	payload := map[string]interface{}{
		"schedule_id": params.ScheduleID,
		"vm_name":     params.VMName,
		"event_type":  params.EventType,
		"action":      params.Action,
		"skipped":     skipped,
		"message":     message,
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

// scheduleTimeNow 获取当前时间并按定时任务配置的时区转换。
// 用于记录 last_triggered_at / last_finished_at，
// 确保展示时间与用户配置的时区一致。
func scheduleTimeNow(scheduleID uint) time.Time {
	now := time.Now()
	schedule, err := model.GetVMScheduleByID(scheduleID)
	if err != nil {
		return now
	}
	loc, _, err := resolveScheduleLocation(schedule.Timezone)
	if err != nil {
		return now
	}
	return now.In(loc)
}

func markScheduleRunning(scheduleID, taskID uint) {
	if scheduleID == 0 {
		return
	}
	fields := map[string]interface{}{
		"last_task_id":      taskID,
		"last_status":       VMScheduleExecStatusRunning,
		"last_message":      "任务执行中",
		"last_triggered_at": scheduleTimeNow(scheduleID),
	}
	if err := model.UpdateVMScheduleFields(scheduleID, fields); err != nil && err != gorm.ErrRecordNotFound {
		logger.App.Warn("更新定时任务运行状态失败", "schedule_id", scheduleID, "error", err)
	}
}

func finishScheduleExecution(scheduleID uint, status, message string) {
	if scheduleID == 0 {
		return
	}
	fields := map[string]interface{}{
		"last_status":      status,
		"last_message":     strings.TrimSpace(message),
		"last_finished_at": scheduleTimeNow(scheduleID),
	}
	if err := model.UpdateVMScheduleFields(scheduleID, fields); err != nil && err != gorm.ErrRecordNotFound {
		logger.App.Warn("更新定时任务执行结果失败", "schedule_id", scheduleID, "error", err)
	}
}

func buildScheduleTriggerReason(params VMScheduledActionTaskParams) string {
	return fmt.Sprintf("虚拟机详情中的定时任务已触发：%s/%s", scheduleEventTypeText(params.EventType), scheduleActionText(params.Action))
}

func scheduleEventTypeText(eventType string) string {
	switch eventType {
	case VMScheduleEventTypePower:
		return "电源事件"
	case VMScheduleEventTypeLifecycle:
		return "虚拟机事件"
	default:
		return eventType
	}
}

func scheduleActionText(action string) string {
	switch action {
	case VMScheduleActionStart:
		return "开机"
	case VMScheduleActionShutdown:
		return "关机"
	case VMScheduleActionDelete:
		return "删除虚拟机"
	default:
		return action
	}
}

func buildVMScheduleItem(schedule model.VMSchedule) VMScheduleItem {
	return VMScheduleItem{
		ID:              schedule.ID,
		VMName:          schedule.VMName,
		EventType:       schedule.EventType,
		Action:          schedule.Action,
		ScheduleType:    schedule.ScheduleType,
		RunAt:           schedule.RunAt,
		Timezone:        schedule.Timezone,
		TimeOfDay:       schedule.TimeOfDay,
		Weekdays:        parseScheduleWeekdays(schedule.Weekdays),
		Enabled:         schedule.Enabled,
		NextRunAt:       schedule.NextRunAt,
		LastTaskID:      schedule.LastTaskID,
		LastTriggeredAt: schedule.LastTriggeredAt,
		LastFinishedAt:  schedule.LastFinishedAt,
		LastStatus:      schedule.LastStatus,
		LastMessage:     schedule.LastMessage,
		CreatedBy:       schedule.CreatedBy,
		CreatedAt:       schedule.CreatedAt,
		UpdatedAt:       schedule.UpdatedAt,
	}
}

func buildVMScheduleModel(existing *model.VMSchedule, vmName, createdBy string, input VMScheduleInput) (*model.VMSchedule, error) {
	eventType := strings.ToLower(strings.TrimSpace(input.EventType))
	action := strings.ToLower(strings.TrimSpace(input.Action))
	scheduleType := strings.ToLower(strings.TrimSpace(input.ScheduleType))
	if eventType == "" {
		return nil, fmt.Errorf("请选择事件类型")
	}
	if action == "" {
		return nil, fmt.Errorf("请选择执行动作")
	}
	if scheduleType == "" {
		return nil, fmt.Errorf("请选择执行计划")
	}

	switch eventType {
	case VMScheduleEventTypePower:
		if action != VMScheduleActionStart && action != VMScheduleActionShutdown {
			return nil, fmt.Errorf("电源事件仅支持定时开机或关机")
		}
	case VMScheduleEventTypeLifecycle:
		if action != VMScheduleActionDelete {
			return nil, fmt.Errorf("虚拟机事件当前仅支持删除虚拟机")
		}
	default:
		return nil, fmt.Errorf("不支持的事件类型: %s", eventType)
	}

	switch scheduleType {
	case VMScheduleTypeOnce, VMScheduleTypeDaily, VMScheduleTypeWeekly:
	default:
		return nil, fmt.Errorf("不支持的计划类型: %s", scheduleType)
	}
	if action == VMScheduleActionDelete && scheduleType != VMScheduleTypeOnce {
		return nil, fmt.Errorf("删除虚拟机仅支持一次性任务")
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	} else if existing != nil {
		enabled = existing.Enabled
	}

	location, timezone, err := resolveScheduleLocation(input.Timezone)
	if err != nil {
		return nil, err
	}

	runAt, err := parseScheduleRunAt(input.RunAt, location)
	if err != nil {
		return nil, err
	}
	timeOfDay := strings.TrimSpace(input.TimeOfDay)
	weekdays, err := normalizeScheduleWeekdays(input.Weekdays)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	nextRunAt, err := calculateNextRun(eventType, action, scheduleType, runAt, timeOfDay, weekdays, location, now)
	if err != nil {
		return nil, err
	}
	if !enabled {
		nextRunAt = nil
	}

	schedule := &model.VMSchedule{
		VMName:       strings.TrimSpace(vmName),
		EventType:    eventType,
		Action:       action,
		ScheduleType: scheduleType,
		RunAt:        runAt,
		Timezone:     timezone,
		TimeOfDay:    timeOfDay,
		Weekdays:     joinScheduleWeekdays(weekdays),
		Enabled:      enabled,
		NextRunAt:    nextRunAt,
		CreatedBy:    strings.TrimSpace(createdBy),
	}
	if schedule.CreatedBy == "" {
		schedule.CreatedBy = "system"
	}
	if existing != nil {
		schedule.CreatedBy = existing.CreatedBy
	}
	return schedule, nil
}

func calculateNextRunForModel(schedule model.VMSchedule, after *time.Time) (*time.Time, error) {
	location, _, err := resolveScheduleLocation(schedule.Timezone)
	if err != nil {
		return nil, err
	}
	base := time.Now()
	if after != nil {
		base = *after
	}
	return calculateNextRun(
		schedule.EventType,
		schedule.Action,
		schedule.ScheduleType,
		schedule.RunAt,
		schedule.TimeOfDay,
		parseScheduleWeekdays(schedule.Weekdays),
		location,
		base,
	)
}

func calculateNextRun(eventType, action, scheduleType string, runAt *time.Time, timeOfDay string, weekdays []int, location *time.Location, now time.Time) (*time.Time, error) {
	switch scheduleType {
	case VMScheduleTypeOnce:
		if runAt == nil {
			return nil, fmt.Errorf("请填写执行时间")
		}
		if !runAt.After(now) {
			return nil, fmt.Errorf("执行时间必须晚于当前时间")
		}
		value := runAt.In(location)
		return &value, nil

	case VMScheduleTypeDaily:
		hour, minute, err := parseTimeOfDay(timeOfDay)
		if err != nil {
			return nil, err
		}
		current := now.In(location)
		candidate := time.Date(current.Year(), current.Month(), current.Day(), hour, minute, 0, 0, location)
		if !candidate.After(current) {
			candidate = candidate.AddDate(0, 0, 1)
		}
		return &candidate, nil

	case VMScheduleTypeWeekly:
		hour, minute, err := parseTimeOfDay(timeOfDay)
		if err != nil {
			return nil, err
		}
		if len(weekdays) == 0 {
			return nil, fmt.Errorf("请选择每周执行的日期")
		}
		current := now.In(location)
		for offset := 0; offset <= 7; offset++ {
			day := current.AddDate(0, 0, offset)
			if !slices.Contains(weekdays, isoWeekday(day.Weekday())) {
				continue
			}
			candidate := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, location)
			if candidate.After(current) {
				return &candidate, nil
			}
		}
		return nil, fmt.Errorf("无法计算下次执行时间")
	}

	return nil, fmt.Errorf("不支持的计划类型: %s", scheduleType)
}

func parseScheduleRunAt(raw string, location *time.Location) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return &parsed, nil
		}
		if parsed, err := time.ParseInLocation(layout, raw, location); err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("无法识别执行时间格式")
}

func parseTimeOfDay(raw string) (int, int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, 0, fmt.Errorf("请填写执行时刻")
	}
	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("执行时刻格式错误")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("执行时刻的小时不合法")
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("执行时刻的分钟不合法")
	}
	return hour, minute, nil
}

func resolveScheduleLocation(raw string) (*time.Location, string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		if time.Local != nil {
			return time.Local, time.Local.String(), nil
		}
		return time.UTC, time.UTC.String(), nil
	}
	if strings.EqualFold(value, "local") {
		if time.Local != nil {
			return time.Local, time.Local.String(), nil
		}
		return time.UTC, time.UTC.String(), nil
	}
	loc, err := time.LoadLocation(value)
	if err != nil {
		return nil, "", fmt.Errorf("时区无效: %s", value)
	}
	return loc, value, nil
}

func normalizeScheduleWeekdays(input []int) ([]int, error) {
	if len(input) == 0 {
		return nil, nil
	}
	set := make(map[int]struct{})
	values := make([]int, 0, len(input))
	for _, item := range input {
		if item < 1 || item > 7 {
			return nil, fmt.Errorf("每周执行日必须在 1-7 之间")
		}
		if _, exists := set[item]; exists {
			continue
		}
		set[item] = struct{}{}
		values = append(values, item)
	}
	sort.Ints(values)
	return values, nil
}

func joinScheduleWeekdays(values []int) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, item := range values {
		parts = append(parts, strconv.Itoa(item))
	}
	return strings.Join(parts, ",")
}

func parseScheduleWeekdays(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	values := make([]int, 0)
	for _, part := range strings.Split(raw, ",") {
		num, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || num < 1 || num > 7 {
			continue
		}
		values = append(values, num)
	}
	sort.Ints(values)
	return slices.Compact(values)
}

func isoWeekday(day time.Weekday) int {
	switch day {
	case time.Sunday:
		return 7
	default:
		return int(day)
	}
}
