package snapshot

import (
	"fmt"
	"strings"

	"qvmhub/model"
	"qvmhub/utils"
)

// CountVMSnapshots 统计单台虚拟机的快照数量。
func CountVMSnapshots(vmName string) int {
	result := utils.ExecCommandQuiet("virsh", "snapshot-list", vmName, "--name")
	if result.Error != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(result.Stdout, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

// CountUserSnapshots 统计用户名下所有虚拟机的快照总数。
func CountUserSnapshots(username string) int {
	total := 0
	for _, vmName := range HookGetUserVMList(username) {
		total += CountVMSnapshots(vmName)
	}
	return total
}

func BuildVMSnapshotQuotaInfo(username, role, vmName string, currentCount int) *SnapshotQuotaInfo {
	username = strings.TrimSpace(username)
	if strings.TrimSpace(role) == "admin" || username == "" || strings.TrimSpace(vmName) == "" {
		return nil
	}
	if currentCount < 0 {
		currentCount = CountVMSnapshots(vmName)
	}
	if quota, err := HookGetLightweightVMQuota(vmName); err == nil && quota != nil {
		info := &SnapshotQuotaInfo{
			Scope:         CloudTypeLightweight,
			UsedSnapshots: currentCount,
			MaxSnapshots:  quota.MaxSnapshots,
		}
		if info.MaxSnapshots > 0 {
			info.RemainingSnapshots = info.MaxSnapshots - info.UsedSnapshots
			if info.RemainingSnapshots < 0 {
				info.RemainingSnapshots = 0
			}
		}
		return info
	}

	var user model.User
	if err := model.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return nil
	}
	info := &SnapshotQuotaInfo{
		Scope:         CloudTypeElastic,
		UsedSnapshots: CountUserSnapshots(username),
		MaxSnapshots:  user.MaxSnapshots,
	}
	if info.MaxSnapshots > 0 {
		info.RemainingSnapshots = info.MaxSnapshots - info.UsedSnapshots
		if info.RemainingSnapshots < 0 {
			info.RemainingSnapshots = 0
		}
	}
	return info
}

func CheckUserSnapshotQuota(username string, delta int) error {
	if delta <= 0 {
		return nil
	}
	var user model.User
	if err := model.DB.Where("username = ?", strings.TrimSpace(username)).First(&user).Error; err != nil {
		return fmt.Errorf("用户不存在: %w", err)
	}
	if user.Role == "admin" || user.MaxSnapshots <= 0 {
		return nil
	}
	used := CountUserSnapshots(username)
	if used+delta > user.MaxSnapshots {
		return fmt.Errorf("当前快照数量超出配额限制（已用 %d / 上限 %d）", used, user.MaxSnapshots)
	}
	return nil
}

func CheckVMSnapshotQuota(username, role, vmName string, delta int) error {
	if delta <= 0 || strings.TrimSpace(role) == "admin" {
		return nil
	}
	if HookIsLightweightCloudVM(vmName) || HookIsLightweightCloudUser(username) {
		return HookCheckLightweightVMSnapshotQuota(username, vmName, delta)
	}
	return CheckUserSnapshotQuota(username, delta)
}
