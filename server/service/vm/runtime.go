package vm

import (
	"strings"
	"sync"
	"time"

	"qvmhub/logger"
	"qvmhub/utils"
)

// VMRuntimeInfo 虚拟机连续运行时间快照
type VMRuntimeInfo struct {
	ContinuousRuntimeSeconds int64
	ContinuousRunningSince   string
}

type vmRuntimeRecord struct {
	VMName              string
	LastState           string
	CurrentRunStartedAt *time.Time
}

var vmRuntimeCache = struct {
	sync.RWMutex
	data map[string]*vmRuntimeRecord
}{data: make(map[string]*vmRuntimeRecord)}

// InitializeVMRuntimeTracker 初始化连续运行时间跟踪器
func InitializeVMRuntimeTracker() {
	SyncVMRuntimeStatesFromHost(time.Now())
}

// SyncVMRuntimeStatesFromHost 从宿主机同步虚拟机运行状态
func SyncVMRuntimeStatesFromHost(observedAt time.Time) {
	observed := make(map[string]string)

	for _, item := range []struct {
		state string
		args  []string
	}{
		{state: "running", args: []string{"list", "--name", "--state-running"}},
		{state: "paused", args: []string{"list", "--name", "--state-paused"}},
	} {
		result := utils.ExecCommand("virsh", item.args...)
		if result.Error != nil {
			continue
		}
		for _, line := range strings.Split(result.Stdout, "\n") {
			name := strings.TrimSpace(line)
			if name == "" {
				continue
			}
			observed[name] = item.state
		}
	}

	syncVMRuntimeStates(observed, observedAt)
}

// UpdateVMRuntimeState 更新单台虚拟机连续运行状态
func UpdateVMRuntimeState(name, status string, observedAt time.Time) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}

	vmRuntimeCache.Lock()
	record := ensureVMRuntimeRecordLocked(name)
	changed := applyVMRuntimeObservation(record, status, observedAt)
	vmRuntimeCache.Unlock()

	if changed {
		logger.App.Info("虚拟机连续运行状态已更新", "vm", name, "state", normalizeVMRuntimeState(status))
	}
}

// ResetVMContinuousRuntime 重置单台虚拟机连续运行起点
func ResetVMContinuousRuntime(name string, observedAt time.Time) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}

	vmRuntimeCache.Lock()
	record := ensureVMRuntimeRecordLocked(name)
	startedAt := observedAt
	changed := false
	if record.CurrentRunStartedAt == nil || !record.CurrentRunStartedAt.Equal(startedAt) {
		record.CurrentRunStartedAt = &startedAt
		changed = true
	}
	if record.LastState != "running" {
		record.LastState = "running"
		changed = true
	}
	vmRuntimeCache.Unlock()

	if changed {
		logger.App.Info("虚拟机连续运行时间已重置", "vm", name)
	}
}

// DeleteVMRuntimeRecord 删除虚拟机连续运行记录
func DeleteVMRuntimeRecord(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}

	vmRuntimeCache.Lock()
	delete(vmRuntimeCache.data, name)
	vmRuntimeCache.Unlock()
}

// GetVMRuntimeInfo 获取单台虚拟机连续运行时间信息
func GetVMRuntimeInfo(name, status string) VMRuntimeInfo {
	name = strings.TrimSpace(name)
	if name == "" {
		return VMRuntimeInfo{}
	}

	vmRuntimeCache.RLock()
	record := cloneVMRuntimeRecord(vmRuntimeCache.data[name])
	vmRuntimeCache.RUnlock()

	return buildVMRuntimeInfo(record, status, time.Now())
}

func syncVMRuntimeStates(observed map[string]string, observedAt time.Time) {
	vmRuntimeCache.Lock()
	known := make(map[string]struct{}, len(vmRuntimeCache.data)+len(observed))
	for name := range vmRuntimeCache.data {
		known[name] = struct{}{}
	}
	for name := range observed {
		known[name] = struct{}{}
	}

	for name := range known {
		record := ensureVMRuntimeRecordLocked(name)
		status := observed[name]
		if status == "" {
			status = "shut off"
		}
		applyVMRuntimeObservation(record, status, observedAt)
	}
	vmRuntimeCache.Unlock()
}

func ensureVMRuntimeRecordLocked(name string) *vmRuntimeRecord {
	record := vmRuntimeCache.data[name]
	if record != nil {
		return record
	}

	record = &vmRuntimeRecord{VMName: name}
	vmRuntimeCache.data[name] = record
	return record
}

func applyVMRuntimeObservation(record *vmRuntimeRecord, status string, observedAt time.Time) bool {
	if record == nil {
		return false
	}

	normalized := normalizeVMRuntimeState(status)
	active := isVMRuntimeActiveState(normalized)
	changed := false

	if active {
		if record.CurrentRunStartedAt == nil {
			startedAt := observedAt
			record.CurrentRunStartedAt = &startedAt
			changed = true
		}
	} else if record.CurrentRunStartedAt != nil {
		record.CurrentRunStartedAt = nil
		changed = true
	}

	if record.LastState != normalized {
		record.LastState = normalized
		changed = true
	}

	return changed
}

func buildVMRuntimeInfo(record *vmRuntimeRecord, status string, now time.Time) VMRuntimeInfo {
	if !isVMRuntimeActiveState(status) || record == nil || record.CurrentRunStartedAt == nil {
		return VMRuntimeInfo{}
	}

	startedAt := *record.CurrentRunStartedAt
	seconds := int64(now.Sub(startedAt).Seconds())
	if seconds < 0 {
		seconds = 0
	}

	return VMRuntimeInfo{
		ContinuousRuntimeSeconds: seconds,
		ContinuousRunningSince:   startedAt.Format("2006-01-02 15:04:05"),
	}
}

func normalizeVMRuntimeState(status string) string {
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "" {
		return "shut off"
	}
	return status
}

func isVMRuntimeActiveState(status string) bool {
	switch normalizeVMRuntimeState(status) {
	case "running", "paused":
		return true
	default:
		return false
	}
}

func cloneVMRuntimeRecord(record *vmRuntimeRecord) *vmRuntimeRecord {
	if record == nil {
		return nil
	}

	copyRecord := *record
	if record.CurrentRunStartedAt != nil {
		startedAt := *record.CurrentRunStartedAt
		copyRecord.CurrentRunStartedAt = &startedAt
	}
	return &copyRecord
}
