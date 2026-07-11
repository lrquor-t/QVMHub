package snapshot

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"qvmhub/utils"
)

// ValidateSnapshotName 校验快照名称，避免 libvirt/QEMU 内部任务 ID 与文件名不兼容。
func ValidateSnapshotName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("请输入快照名称")
	}
	if !snapshotNameRegexp.MatchString(name) {
		return fmt.Errorf("快照名称只能包含英文字母、数字、下划线、点和短横线，且必须以英文字母、数字或下划线开头，最长 64 个字符")
	}
	return nil
}

// GenerateSnapshotName 生成兼容 libvirt/QEMU 内部任务 ID 的快照名称。
func GenerateSnapshotName() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "snap_" + time.Now().Format("20060102_150405")
	}
	return "snap_" + time.Now().Format("20060102_150405") + "_" + hex.EncodeToString(buf)
}

func NormalizeSnapshotName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = GenerateSnapshotName()
	}
	if err := ValidateSnapshotName(name); err != nil {
		return "", err
	}
	return name, nil
}

func CheckInternalSnapshotVirtFSUnsupported(vmName string) (bool, string, error) {
	stateResult := utils.ExecCommand("virsh", "domstate", vmName)
	if stateResult.Error != nil {
		return false, "", fmt.Errorf("获取虚拟机状态失败: %s", stateResult.Stderr)
	}
	if strings.TrimSpace(stateResult.Stdout) != "running" {
		return false, "", nil
	}
	shares, err := HookListShares(vmName)
	if err != nil {
		return false, "", err
	}
	if len(shares) == 0 {
		return false, "", nil
	}
	parts := make([]string, 0, len(shares))
	for _, share := range shares {
		label := strings.TrimSpace(share.Tag)
		if label == "" {
			label = strings.TrimSpace(share.Target)
		}
		if share.Source != "" {
			label += " -> " + share.Source
		}
		parts = append(parts, label)
	}
	return true, "当前虚拟机正在挂载 9p/VirtFS 共享目录（" + strings.Join(parts, "，") + "），libvirt 禁止在这种状态创建包含内存的内部快照。请先在虚拟机内卸载共享目录并关机移除共享挂载，或取消勾选\"保存虚拟机内存状态\"创建仅磁盘快照。", nil
}

// getSnapshotType 获取快照的类型/位置信息
// 返回: state(running/shutoff/disk-snapshot), location(internal/external)
func getSnapshotType(vmName, snapName string) (state string, location string, err error) {
	info, err := getSnapshotInfo(vmName, snapName)
	if err != nil {
		return "", "", err
	}
	return info.State, info.Location, nil
}

func getSnapshotInfo(vmName, snapName string) (snapshotInfoOutput, error) {
	infoResult := utils.ExecCommand("virsh", "snapshot-info", vmName, snapName)
	if infoResult.Error != nil {
		return snapshotInfoOutput{}, fmt.Errorf("获取快照信息失败: %s", infoResult.Stderr)
	}
	return parseSnapshotInfoOutput(infoResult.Stdout), nil
}

func parseSnapshotInfoOutput(output string) snapshotInfoOutput {
	var info snapshotInfoOutput
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "State:") {
			info.State = strings.TrimSpace(strings.TrimPrefix(line, "State:"))
		}
		if strings.HasPrefix(line, "Location:") {
			info.Location = strings.TrimSpace(strings.TrimPrefix(line, "Location:"))
		}
		if strings.HasPrefix(line, "Children:") {
			info.Children = parseSnapshotInfoInt(line, "Children:")
		}
		if strings.HasPrefix(line, "Descendants:") {
			info.Descendants = parseSnapshotInfoInt(line, "Descendants:")
		}
	}
	return info
}

// snapshotExists 检查指定快照是否存在。
func snapshotExists(vmName, snapName string) bool {
	result := utils.ExecCommandQuiet("virsh", "snapshot-list", vmName, "--name")
	if result.Error != nil {
		return false
	}
	for _, name := range strings.Split(result.Stdout, "\n") {
		if strings.TrimSpace(name) == snapName {
			return true
		}
	}
	return false
}

func parseSnapshotInfoInt(line, prefix string) int {
	value := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}
