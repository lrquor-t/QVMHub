package service

import (
	"qvmhub/model"
	"qvmhub/service/lxc"
	schedpkg "qvmhub/service/scheduler"
	"qvmhub/service/vm/memory"
)

// init wires scheduler package hook variables to service root implementations,
// and registers scheduler functions into other subpackage hooks.
// This breaks the circular dependency: scheduler package cannot import service,
// so it exposes hook variables that we set here.
func init() {
	// ── 向 memory 子包注册 scheduler 函数（替换原 hooks_init.go 中的逻辑） ──
	memory.HookMemoryRegisterScheduler = func(def memory.SchedulerDefinition) {
		RegisterScheduler(SchedulerDefinition{
			Key:         def.Key,
			Name:        def.Name,
			Group:       def.Group,
			Description: def.Description,
			Enabled:     def.Enabled,
		})
	}
	memory.HookMemoryStartSchedulerEvent = func(input memory.SchedulerEventStartInput) (interface{}, error) {
		return StartSchedulerEvent(SchedulerEventStartInput{
			SchedulerKey:   input.SchedulerKey,
			SchedulerName:  input.SchedulerName,
			SchedulerGroup: input.SchedulerGroup,
			VMName:         input.VMName,
			VMBackend:      input.VMBackend,
			TriggerReason:  input.TriggerReason,
		})
	}
	memory.HookMemoryFinishSchedulerEventOk = func(event interface{}, msg string) error {
		if e, ok := event.(*model.SchedulerEvent); ok {
			return FinishSchedulerEventSuccess(e, msg)
		}
		return nil
	}
	memory.HookMemoryFinishSchedulerEventFail = func(event interface{}, msg string) error {
		if e, ok := event.(*model.SchedulerEvent); ok {
			return FinishSchedulerEventFailed(e, msg)
		}
		return nil
	}

	// ── 向 lxc 子包注册 scheduler 函数（LXC 定时任务执行事件用）──
	lxc.HookRegisterScheduler = func(def lxc.SchedulerDefinition) {
		RegisterScheduler(SchedulerDefinition{
			Key:         def.Key,
			Name:        def.Name,
			Group:       def.Group,
			Description: def.Description,
			Enabled:     def.Enabled,
		})
	}
	lxc.HookStartSchedulerEvent = func(input lxc.SchedulerEventStartInput) (*model.SchedulerEvent, error) {
		return StartSchedulerEvent(SchedulerEventStartInput{
			SchedulerKey:   input.SchedulerKey,
			SchedulerName:  input.SchedulerName,
			SchedulerGroup: input.SchedulerGroup,
			VMName:         input.VMName,
			VMBackend:      input.VMBackend,
			TriggerReason:  input.TriggerReason,
		})
	}
	lxc.HookFinishSchedulerEventSuccess = FinishSchedulerEventSuccess
	lxc.HookFinishSchedulerEventFailed = FinishSchedulerEventFailed
}

// ── Type aliases（向后兼容，让 service 根包和外部调用方可直接使用类型名） ──

type SchedulerDefinition = schedpkg.SchedulerDefinition
type SchedulerListItem = schedpkg.SchedulerListItem
type SchedulerEventMessage = schedpkg.SchedulerEventMessage
type SchedulerEventStartInput = schedpkg.SchedulerEventStartInput

// ── Exported delegates ──

func RegisterScheduler(def SchedulerDefinition) {
	schedpkg.RegisterScheduler(def)
}

func ListSchedulers() ([]SchedulerListItem, error) {
	return schedpkg.ListSchedulers()
}

func StartSchedulerEvent(input SchedulerEventStartInput) (*model.SchedulerEvent, error) {
	return schedpkg.StartSchedulerEvent(input)
}

func FinishSchedulerEventSuccess(event *model.SchedulerEvent, resultMessage string) error {
	return schedpkg.FinishSchedulerEventSuccess(event, resultMessage)
}

func FinishSchedulerEventFailed(event *model.SchedulerEvent, errorMessage string) error {
	return schedpkg.FinishSchedulerEventFailed(event, errorMessage)
}

func RegisterSchedulerSSEClient(ch chan SchedulerEventMessage) {
	schedpkg.RegisterSchedulerSSEClient(ch)
}

func UnregisterSchedulerSSEClient(ch chan SchedulerEventMessage) {
	schedpkg.UnregisterSchedulerSSEClient(ch)
}

func StartSchedulerEventCleanup() {
	schedpkg.StartSchedulerEventCleanup()
}
