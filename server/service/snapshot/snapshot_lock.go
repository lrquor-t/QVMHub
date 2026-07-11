package snapshot

import (
	"encoding/json"
	"fmt"
	"strings"

	"qvmhub/model"
	"qvmhub/taskqueue"
	"qvmhub/utils"
)

// snapshotTaskParamsForMatch 用于从任务参数 JSON 中匹配 VM 名称。
type snapshotTaskParamsForMatch struct {
	VmName string `json:"vm_name"`
}

// IsSnapshotInProgress 判断 VM 是否正在进行快照操作。
// 同时检查任务队列和 VM 的 libvirt 状态（paused + saving reason），
// 双重保障避免遗漏。
func IsSnapshotInProgress(vmName string) bool {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" {
		return false
	}

	// 方案A: 查询任务队列中该 VM 是否有未完成的快照任务（pending / running）
	if taskqueue.HasActiveTask(model.TaskTypeSnapshot, func(raw string) bool {
		var params snapshotTaskParamsForMatch
		if err := json.Unmarshal([]byte(raw), &params); err != nil {
			return false
		}
		return strings.TrimSpace(params.VmName) == vmName
	}) {
		return true
	}

	// 方案B: 通过 libvirt 检查 VM 是否处于 paused (saving) 状态
	// QEMU 保存内存快照时，VM 必然进入 paused (saving) 状态，
	// 即使任务队列尚未记录或已丢失，此检查也能兜底。
	stateResult := utils.ExecCommandQuiet("virsh", "domstate", vmName, "--reason")
	if stateResult.Error == nil {
		state := strings.TrimSpace(stateResult.Stdout)
		// "paused (saving)" 表示正在保存内存快照
		if strings.Contains(state, "paused") && strings.Contains(state, "saving") {
			return true
		}
	}

	return false
}

// EnsureVMNotSnapshotting 在执行 VM 操作前阻止快照中的 VM 被修改。
func EnsureVMNotSnapshotting(vmName, action string) error {
	if !IsSnapshotInProgress(vmName) {
		return nil
	}
	action = strings.TrimSpace(action)
	if action == "" {
		action = "该操作"
	}
	return fmt.Errorf("虚拟机正在执行快照操作，暂不能执行%s", action)
}
