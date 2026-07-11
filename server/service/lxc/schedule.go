package lxc

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

// ---- scheduler hook（由 service 根包在 scheduler_wire.go init() 注入，打破循环依赖）----
var (
	HookRegisterScheduler           func(def SchedulerDefinition)
	HookStartSchedulerEvent         func(input SchedulerEventStartInput) (*model.SchedulerEvent, error)
	HookFinishSchedulerEventSuccess func(event *model.SchedulerEvent, message string) error
	HookFinishSchedulerEventFailed  func(event *model.SchedulerEvent, message string) error
)

// SchedulerDefinition 调度器定义（镜像 scheduler.SchedulerDefinition，避免 import scheduler）。
type SchedulerDefinition struct {
	Key         string
	Name        string
	Group       string
	Description string
	Enabled     func() bool
}

// SchedulerEventStartInput 调度事件开始参数（字段名与 SchedulerEvent 表列对齐：容器名填 VMName，event_type 填 VMBackend）。
type SchedulerEventStartInput struct {
	SchedulerKey   string
	SchedulerName  string
	SchedulerGroup string
	VMName         string
	VMBackend      string
	TriggerReason  string
}

const (
	LXCScheduleEventTypePower = "power"
	LXCScheduleEventTypeLXC   = "lxc"

	LXCScheduleActionStart  = "start"
	LXCScheduleActionStop   = "stop"
	LXCScheduleActionDelete = "delete"

	LXCScheduleTypeOnce   = "once"
	LXCScheduleTypeDaily  = "daily"
	LXCScheduleTypeWeekly = "weekly"

	LXCScheduleExecStatusPending = "pending"
	LXCScheduleExecStatusRunning = "running"
	LXCScheduleExecStatusSuccess = "success"
	LXCScheduleExecStatusFailed  = "failed"

	lxcScheduleSchedulerKey   = "lxc_scheduled_task"
	lxcScheduleSchedulerName  = "容器定时任务"
	lxcScheduleSchedulerGroup = "容器管理"
)

var lxcScheduleRunnerOnce sync.Once
var lxcScheduleRegisterOnce sync.Once

// LXCScheduleInput 定时任务写入参数。
type LXCScheduleInput struct {
	EventType    string `json:"event_type"`
	Action       string `json:"action"`
	ScheduleType string `json:"schedule_type"`
	RunAt        string `json:"run_at"`
	Timezone     string `json:"timezone"`
	TimeOfDay    string `json:"time_of_day"`
	Weekdays     []int  `json:"weekdays"`
	Enabled      *bool  `json:"enabled"`
}

// LXCScheduleItem 定时任务展示数据。
type LXCScheduleItem struct {
	ID              uint       `json:"id"`
	Name            string     `json:"name"`
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

// LXCScheduledActionTaskParams 定时任务触发后的执行参数。
type LXCScheduledActionTaskParams struct {
	ScheduleID uint   `json:"schedule_id"`
	Name       string `json:"name"`
	EventType  string `json:"event_type"`
	Action     string `json:"action"`
}

// registerLXCScheduleScheduler 注册 LXC 定时任务调度器（延迟到 StartLXCScheduleRunner 中调用）。
func registerLXCScheduleScheduler() {
	lxcScheduleRegisterOnce.Do(func() {
		if HookRegisterScheduler == nil {
			return
		}
		HookRegisterScheduler(SchedulerDefinition{
			Key:         lxcScheduleSchedulerKey,
			Name:        lxcScheduleSchedulerName,
			Group:       lxcScheduleSchedulerGroup,
			Description: "按容器管理中的定时任务配置执行启动、停止和删除动作",
			Enabled: func() bool {
				return true
			},
		})
	})
}

// getContainerCacheForSchedule 查容器缓存行；不存在返回 gorm.ErrRecordNotFound。
func getContainerCacheForSchedule(name string) (*model.LXCCache, error) {
	var c model.LXCCache
	if err := model.DB.Where("name = ?", name).First(&c).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("容器不存在: %s", name)
		}
		return nil, err
	}
	return &c, nil
}

// ListLXCSchedules 获取容器定时任务列表。
func ListLXCSchedules(name string) ([]LXCScheduleItem, error) {
	list, err := model.ListLXCSchedulesByContainer(strings.TrimSpace(name))
	if err != nil {
		return nil, err
	}
	items := make([]LXCScheduleItem, 0, len(list))
	for _, item := range list {
		items = append(items, buildLXCScheduleItem(item))
	}
	return items, nil
}

// CreateLXCSchedule 创建定时任务。
func CreateLXCSchedule(name, createdBy string, input LXCScheduleInput) (*LXCScheduleItem, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("容器名称不能为空")
	}
	if _, err := getContainerCacheForSchedule(name); err != nil {
		return nil, err
	}
	schedule, err := buildLXCScheduleModel(nil, name, createdBy, input)
	if err != nil {
		return nil, err
	}
	if err := model.CreateLXCSchedule(schedule); err != nil {
		return nil, err
	}
	item := buildLXCScheduleItem(*schedule)
	return &item, nil
}

// UpdateLXCSchedule 更新定时任务。
func UpdateLXCSchedule(name string, id uint, input LXCScheduleInput) (*LXCScheduleItem, error) {
	existing, err := model.GetLXCScheduleByIDAndContainer(id, strings.TrimSpace(name))
	if err != nil {
		return nil, err
	}
	schedule, err := buildLXCScheduleModel(existing, existing.Name, existing.CreatedBy, input)
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
	if err := model.UpdateLXCSchedule(schedule); err != nil {
		return nil, err
	}
	item := buildLXCScheduleItem(*schedule)
	return &item, nil
}

// DeleteLXCSchedule 删除定时任务。
func DeleteLXCSchedule(name string, id uint) error {
	schedule, err := model.GetLXCScheduleByIDAndContainer(id, strings.TrimSpace(name))
	if err != nil {
		return err
	}
	return model.DeleteLXCSchedule(schedule.ID)
}

// DeleteLXCSchedules 删除容器关联的全部定时任务。
func DeleteLXCSchedules(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	return model.DeleteLXCSchedulesByContainer(name)
}

// StartLXCScheduleRunner 启动后台扫描器。
func StartLXCScheduleRunner() {
	registerLXCScheduleScheduler()
	lxcScheduleRunnerOnce.Do(func() {
		go func() {
			defer utils.RecoverAndLog("lxc-schedule-runner")
			runDueLXCSchedulesOnce()
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				runDueLXCSchedulesOnce()
			}
		}()
	})
}

func runDueLXCSchedulesOnce() {
	now := time.Now()
	dueList, err := model.ListDueLXCSchedules(now, 20)
	if err != nil {
		logger.App.Warn("扫描 LXC 定时任务失败", "error", err)
		return
	}
	for _, schedule := range dueList {
		if err := queueDueLXCSchedule(schedule, now); err != nil {
			logger.App.Warn("提交 LXC 定时任务失败", "schedule_id", schedule.ID, "name", schedule.Name, "error", err)
		}
	}
}

func queueDueLXCSchedule(schedule model.LXCSchedule, now time.Time) error {
	params := LXCScheduledActionTaskParams{
		ScheduleID: schedule.ID,
		Name:       schedule.Name,
		EventType:  schedule.EventType,
		Action:     schedule.Action,
	}
	createdBy := strings.TrimSpace(schedule.CreatedBy)
	if createdBy == "" {
		createdBy = "system:scheduler"
	}
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeLXCScheduleAction, params, createdBy)
	if err != nil {
		_ = model.UpdateLXCScheduleFields(schedule.ID, map[string]interface{}{
			"last_status":  LXCScheduleExecStatusFailed,
			"last_message": "提交任务队列失败: " + err.Error(),
		})
		return err
	}

	triggerTime := now
	if loc, _, locErr := resolveScheduleLocation(schedule.Timezone); locErr == nil {
		triggerTime = now.In(loc)
	}

	updates := map[string]interface{}{
		"last_task_id":      task.ID,
		"last_triggered_at": triggerTime,
		"last_status":       LXCScheduleExecStatusPending,
		"last_message":      fmt.Sprintf("已提交任务中心，任务 #%d 等待执行", task.ID),
	}
	if schedule.ScheduleType == LXCScheduleTypeOnce {
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
	return model.UpdateLXCScheduleFields(schedule.ID, updates)
}

// RunLXCScheduledAction 执行单个定时任务动作。
func RunLXCScheduledAction(ctx context.Context, task *model.Task, progress func(int, string)) (string, error) {
	var params LXCScheduledActionTaskParams
	if err := json.Unmarshal([]byte(task.Params), &params); err != nil {
		return "", fmt.Errorf("解析参数失败: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", taskqueue.ErrTaskCanceled
	}

	markScheduleRunning(params.ScheduleID, task.ID)

	var event *model.SchedulerEvent
	if HookStartSchedulerEvent != nil {
		event, _ = HookStartSchedulerEvent(SchedulerEventStartInput{
			SchedulerKey:   lxcScheduleSchedulerKey,
			SchedulerName:  lxcScheduleSchedulerName,
			SchedulerGroup: lxcScheduleSchedulerGroup,
			VMName:         params.Name,
			VMBackend:      params.EventType,
			TriggerReason:  buildScheduleTriggerReason(params),
		})
	}

	progress(15, "开始执行容器定时任务")
	result, resultMessage, err := executeScheduledContainerAction(ctx, params, progress)
	if err != nil {
		finishScheduleExecution(params.ScheduleID, LXCScheduleExecStatusFailed, err.Error())
		if event != nil && HookFinishSchedulerEventFailed != nil {
			_ = HookFinishSchedulerEventFailed(event, err.Error())
		}
		return result, err
	}

	finishScheduleExecution(params.ScheduleID, LXCScheduleExecStatusSuccess, resultMessage)
	if event != nil && HookFinishSchedulerEventSuccess != nil {
		_ = HookFinishSchedulerEventSuccess(event, resultMessage)
	}
	return result, nil
}

func executeScheduledContainerAction(ctx context.Context, params LXCScheduledActionTaskParams, progress func(int, string)) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", taskqueue.ErrTaskCanceled
	}

	container, containerErr := getContainerCacheForSchedule(params.Name)
	if params.Action != LXCScheduleActionDelete && containerErr != nil {
		return "", "", fmt.Errorf("获取容器状态失败: %w", containerErr)
	}

	switch params.Action {
	case LXCScheduleActionStart:
		if container != nil && container.Status == "RUNNING" {
			message := "容器当前已运行，已跳过本次定时启动"
			progress(100, message)
			return buildScheduledActionResult(params, true, message), message, nil
		}
		progress(40, "正在执行定时启动")
		if err := StartContainer(params.Name); err != nil {
			return "", "", err
		}
		message := "容器启动指令已下发"
		progress(100, message)
		return buildScheduledActionResult(params, false, message), message, nil

	case LXCScheduleActionStop:
		if container != nil && container.Status == "STOPPED" {
			message := "容器当前已停止，已跳过本次定时停止"
			progress(100, message)
			return buildScheduledActionResult(params, true, message), message, nil
		}
		progress(40, "正在执行定时停止")
		if err := StopContainer(params.Name); err != nil {
			return "", "", err
		}
		message := "容器停止指令已下发"
		progress(100, message)
		return buildScheduledActionResult(params, false, message), message, nil

	case LXCScheduleActionDelete:
		if containerErr != nil {
			message := "容器不存在，视为删除完成"
			progress(100, message)
			return buildScheduledActionResult(params, true, message), message, nil
		}
		progress(35, "正在删除容器")
		if err := DestroyContainer(params.Name); err != nil {
			return "", "", err
		}
		message := "容器已删除"
		progress(100, message)
		return buildScheduledActionResult(params, false, message), message, nil
	default:
		return "", "", fmt.Errorf("不支持的定时任务动作: %s", params.Action)
	}
}

func buildScheduledActionResult(params LXCScheduledActionTaskParams, skipped bool, message string) string {
	payload := map[string]interface{}{
		"schedule_id": params.ScheduleID,
		"name":        params.Name,
		"event_type":  params.EventType,
		"action":      params.Action,
		"skipped":     skipped,
		"message":     message,
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

// scheduleTimeNow 获取当前时间并按定时任务配置的时区转换。
func scheduleTimeNow(scheduleID uint) time.Time {
	now := time.Now()
	schedule, err := model.GetLXCScheduleByID(scheduleID)
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
		"last_status":       LXCScheduleExecStatusRunning,
		"last_message":      "任务执行中",
		"last_triggered_at": scheduleTimeNow(scheduleID),
	}
	if err := model.UpdateLXCScheduleFields(scheduleID, fields); err != nil && err != gorm.ErrRecordNotFound {
		logger.App.Warn("更新 LXC 定时任务运行状态失败", "schedule_id", scheduleID, "error", err)
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
	if err := model.UpdateLXCScheduleFields(scheduleID, fields); err != nil && err != gorm.ErrRecordNotFound {
		logger.App.Warn("更新 LXC 定时任务执行结果失败", "schedule_id", scheduleID, "error", err)
	}
}

func buildScheduleTriggerReason(params LXCScheduledActionTaskParams) string {
	return fmt.Sprintf("容器管理中的定时任务已触发：%s/%s", scheduleEventTypeText(params.EventType), scheduleActionText(params.Action))
}

func scheduleEventTypeText(eventType string) string {
	switch eventType {
	case LXCScheduleEventTypePower:
		return "电源事件"
	case LXCScheduleEventTypeLXC:
		return "容器事件"
	default:
		return eventType
	}
}

func scheduleActionText(action string) string {
	switch action {
	case LXCScheduleActionStart:
		return "启动"
	case LXCScheduleActionStop:
		return "停止"
	case LXCScheduleActionDelete:
		return "删除容器"
	default:
		return action
	}
}

func buildLXCScheduleItem(schedule model.LXCSchedule) LXCScheduleItem {
	return LXCScheduleItem{
		ID:              schedule.ID,
		Name:            schedule.Name,
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

func buildLXCScheduleModel(existing *model.LXCSchedule, name, createdBy string, input LXCScheduleInput) (*model.LXCSchedule, error) {
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
	case LXCScheduleEventTypePower:
		if action != LXCScheduleActionStart && action != LXCScheduleActionStop {
			return nil, fmt.Errorf("电源事件仅支持定时启动或停止")
		}
	case LXCScheduleEventTypeLXC:
		if action != LXCScheduleActionDelete {
			return nil, fmt.Errorf("容器事件当前仅支持删除容器")
		}
	default:
		return nil, fmt.Errorf("不支持的事件类型: %s", eventType)
	}

	switch scheduleType {
	case LXCScheduleTypeOnce, LXCScheduleTypeDaily, LXCScheduleTypeWeekly:
	default:
		return nil, fmt.Errorf("不支持的计划类型: %s", scheduleType)
	}
	if action == LXCScheduleActionDelete && scheduleType != LXCScheduleTypeOnce {
		return nil, fmt.Errorf("删除容器仅支持一次性任务")
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

	schedule := &model.LXCSchedule{
		Name:         strings.TrimSpace(name),
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

func calculateNextRunForModel(schedule model.LXCSchedule, after *time.Time) (*time.Time, error) {
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
	case LXCScheduleTypeOnce:
		if runAt == nil {
			return nil, fmt.Errorf("请填写执行时间")
		}
		if !runAt.After(now) {
			return nil, fmt.Errorf("执行时间必须晚于当前时间")
		}
		value := runAt.In(location)
		return &value, nil

	case LXCScheduleTypeDaily:
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

	case LXCScheduleTypeWeekly:
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
