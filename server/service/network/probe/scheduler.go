package probe

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/utils"
	schedpkg "qvmhub/service/scheduler"
)

var portForwardProbeRegisterOnce sync.Once

func registerPortForwardProbeScheduler() {
	portForwardProbeRegisterOnce.Do(func() {
		schedpkg.RegisterScheduler(schedpkg.SchedulerDefinition{
			Key:         portForwardProbeSchedulerKey,
			Name:        portForwardProbeSchedulerName,
			Group:       portForwardProbeSchedulerGroup,
			Description: "每小时探测 TCP 端口转发是否暴露明文 HTTP 建站服务，并按白名单自动封禁。",
			Enabled: func() bool {
				return config.GlobalConfig == nil || config.GlobalConfig.PortForwardHTTPProbeEnabled
			},
		})
	})
}

// StartPortForwardHTTPProbeScheduler 启动端口转发 HTTP 探测调度器。
func StartPortForwardHTTPProbeScheduler() {
	registerPortForwardProbeScheduler()
	go func() {
		defer utils.RecoverAndLog("probe-port-forward")
		for {
			intervalMinutes := 60
			if config.GlobalConfig != nil && config.GlobalConfig.PortForwardHTTPProbeIntervalMinutes > 0 {
				intervalMinutes = config.GlobalConfig.PortForwardHTTPProbeIntervalMinutes
			}
			if !HookIsMaintenanceModeEnabled() && (config.GlobalConfig == nil || config.GlobalConfig.PortForwardHTTPProbeEnabled) {
				if _, err := RunPortForwardHTTPProbeScan(context.Background(), "", "scheduler", nil); err != nil {
					logger.App.Warn("端口转发HTTP探测调度执行失败", "error", err)
				}
			}
			time.Sleep(time.Duration(intervalMinutes) * time.Minute)
		}
	}()
}

// ── Scheduler event helpers ──

func startPortForwardProbeEvent(rule *PortForwardRule, statusCode int) *model.SchedulerEvent {
	if rule == nil {
		return nil
	}
	return startPortForwardProbeSchedulerEvent(rule.VMName, strings.ToLower(strings.TrimSpace(rule.Protocol)), fmt.Sprintf("探测到明文 HTTP 响应状态码 %d，准备自动封禁端口转发 %s", statusCode, rule.AccessAddress))
}

func startPortForwardProbeSchedulerEvent(vmName, backend, reason string) *model.SchedulerEvent {
	event, err := schedpkg.StartSchedulerEvent(schedpkg.SchedulerEventStartInput{
		SchedulerKey:   portForwardProbeSchedulerKey,
		SchedulerName:  portForwardProbeSchedulerName,
		SchedulerGroup: portForwardProbeSchedulerGroup,
		VMName:         strings.TrimSpace(vmName),
		VMBackend:      strings.TrimSpace(backend),
		TriggerReason:  strings.TrimSpace(reason),
	})
	if err != nil {
		logger.App.Warn("端口转发HTTP探测记录调度事件失败", "error", err)
		return nil
	}
	return event
}

func finishPortForwardProbeEventSuccess(event *model.SchedulerEvent, message string) {
	if event == nil {
		return
	}
	if err := schedpkg.FinishSchedulerEventSuccess(event, message); err != nil {
		logger.App.Warn("端口转发HTTP探测更新成功事件失败", "error", err)
	}
}

func finishPortForwardProbeEventFailure(event *model.SchedulerEvent, message string) {
	if event == nil {
		return
	}
	if err := schedpkg.FinishSchedulerEventFailed(event, message); err != nil {
		logger.App.Warn("端口转发HTTP探测更新失败事件失败", "error", err)
	}
}
