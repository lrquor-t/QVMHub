package scheduler

import (
	"time"

	"qvmhub/model"
)

const (
	SchedulerEventStatusRunning = "running"
	SchedulerEventStatusSuccess = "success"
	SchedulerEventStatusFailed  = "failed"
)

// SchedulerDefinition 调度器定义。
type SchedulerDefinition struct {
	Key         string
	Name        string
	Group       string
	Description string
	Enabled     func() bool
}

// SchedulerListItem 调度器概览。
type SchedulerListItem struct {
	Key         string     `json:"key"`
	Name        string     `json:"name"`
	Group       string     `json:"group"`
	Enabled     bool       `json:"enabled"`
	Description string     `json:"description"`
	LastEventAt *time.Time `json:"last_event_at"`
}

// SchedulerEventMessage 调度事件 SSE 消息。
type SchedulerEventMessage struct {
	Action string               `json:"action"`
	Event  model.SchedulerEvent `json:"event"`
}

// SchedulerEventStartInput 调度事件开始参数。
type SchedulerEventStartInput struct {
	SchedulerKey   string
	SchedulerName  string
	SchedulerGroup string
	VMName         string
	VMBackend      string
	TriggerReason  string
}
