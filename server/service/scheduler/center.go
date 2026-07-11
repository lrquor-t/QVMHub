package scheduler

import (
	"sort"
	"strings"
	"sync"
	"time"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/utils"
)

var schedulerRegistry = struct {
	sync.RWMutex
	items map[string]SchedulerDefinition
}{items: make(map[string]SchedulerDefinition)}

var schedulerSSEHub = struct {
	sync.RWMutex
	clients map[chan SchedulerEventMessage]bool
}{clients: make(map[chan SchedulerEventMessage]bool)}

// RegisterScheduler 注册调度器。
func RegisterScheduler(def SchedulerDefinition) {
	if strings.TrimSpace(def.Key) == "" {
		return
	}
	schedulerRegistry.Lock()
	defer schedulerRegistry.Unlock()
	schedulerRegistry.items[def.Key] = def
}

// ListSchedulers 获取调度器概览。
func ListSchedulers() ([]SchedulerListItem, error) {
	schedulerRegistry.RLock()
	definitions := make([]SchedulerDefinition, 0, len(schedulerRegistry.items))
	for _, item := range schedulerRegistry.items {
		definitions = append(definitions, item)
	}
	schedulerRegistry.RUnlock()

	latestMap, err := model.GetSchedulerLatestEventTimes()
	if err != nil {
		return nil, err
	}

	result := make([]SchedulerListItem, 0, len(definitions))
	for _, item := range definitions {
		var lastEventAt *time.Time
		if ts, ok := latestMap[item.Key]; ok {
			value := ts
			lastEventAt = &value
		}
		enabled := true
		if item.Enabled != nil {
			enabled = item.Enabled()
		}
		result = append(result, SchedulerListItem{
			Key:         item.Key,
			Name:        item.Name,
			Group:       item.Group,
			Enabled:     enabled,
			Description: item.Description,
			LastEventAt: lastEventAt,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Group == result[j].Group {
			return result[i].Name < result[j].Name
		}
		return result[i].Group < result[j].Group
	})
	return result, nil
}

// StartSchedulerEvent 创建运行中的调度事件。
func StartSchedulerEvent(input SchedulerEventStartInput) (*model.SchedulerEvent, error) {
	now := time.Now()
	event := &model.SchedulerEvent{
		SchedulerKey:   strings.TrimSpace(input.SchedulerKey),
		SchedulerName:  strings.TrimSpace(input.SchedulerName),
		SchedulerGroup: strings.TrimSpace(input.SchedulerGroup),
		VMName:         strings.TrimSpace(input.VMName),
		VMBackend:      strings.TrimSpace(input.VMBackend),
		Status:         SchedulerEventStatusRunning,
		TriggerReason:  strings.TrimSpace(input.TriggerReason),
		StartedAt:      now,
	}
	if err := model.CreateSchedulerEvent(event); err != nil {
		return nil, err
	}
	broadcastSchedulerEvent(SchedulerEventMessage{Action: "upsert", Event: *event})
	return event, nil
}

// FinishSchedulerEventSuccess 将调度事件更新为成功。
func FinishSchedulerEventSuccess(event *model.SchedulerEvent, resultMessage string) error {
	return finishSchedulerEvent(event, SchedulerEventStatusSuccess, strings.TrimSpace(resultMessage), "")
}

// FinishSchedulerEventFailed 将调度事件更新为失败。
func FinishSchedulerEventFailed(event *model.SchedulerEvent, errorMessage string) error {
	return finishSchedulerEvent(event, SchedulerEventStatusFailed, "", strings.TrimSpace(errorMessage))
}

func finishSchedulerEvent(event *model.SchedulerEvent, status, resultMessage, errorMessage string) error {
	if event == nil {
		return nil
	}
	now := time.Now()
	event.Status = status
	event.ResultMessage = resultMessage
	event.ErrorMessage = errorMessage
	event.FinishedAt = &now
	if err := model.UpdateSchedulerEvent(event); err != nil {
		return err
	}
	broadcastSchedulerEvent(SchedulerEventMessage{Action: "upsert", Event: *event})
	return nil
}

// RegisterSchedulerSSEClient 注册调度事件 SSE 客户端。
func RegisterSchedulerSSEClient(ch chan SchedulerEventMessage) {
	schedulerSSEHub.Lock()
	defer schedulerSSEHub.Unlock()
	schedulerSSEHub.clients[ch] = true
	logger.App.Info("调度事件 SSE 客户端已连接", "clients", len(schedulerSSEHub.clients))
}

// UnregisterSchedulerSSEClient 注销调度事件 SSE 客户端。
func UnregisterSchedulerSSEClient(ch chan SchedulerEventMessage) {
	schedulerSSEHub.Lock()
	defer schedulerSSEHub.Unlock()
	delete(schedulerSSEHub.clients, ch)
	close(ch)
	logger.App.Info("调度事件 SSE 客户端已断开", "clients", len(schedulerSSEHub.clients))
}

func broadcastSchedulerEvent(message SchedulerEventMessage) {
	schedulerSSEHub.RLock()
	defer schedulerSSEHub.RUnlock()
	for ch := range schedulerSSEHub.clients {
		select {
		case ch <- message:
		default:
		}
	}
}

// StartSchedulerEventCleanup 启动调度事件清理协程。
func StartSchedulerEventCleanup() {
	go func() {
		defer utils.RecoverAndLog("scheduler-event-cleanup")
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			runSchedulerEventCleanupOnce()
			<-ticker.C
		}
	}()
}

func runSchedulerEventCleanupOnce() {
	retentionHours := 168
	if config.GlobalConfig != nil && config.GlobalConfig.SchedulerEventRetentionHours > 0 {
		retentionHours = config.GlobalConfig.SchedulerEventRetentionHours
	}
	cutoff := time.Now().Add(-time.Duration(retentionHours) * time.Hour)
	count, err := model.DeleteSchedulerEventsBefore(cutoff)
	if err != nil {
		logger.App.Warn("清理调度事件失败", "error", err)
		return
	}
	if count > 0 {
		logger.App.Info("已清理过期调度事件", "count", count)
	}
}
