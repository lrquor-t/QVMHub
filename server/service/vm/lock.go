package vm

import (
	"qvmhub/model"
)

// LockVM 锁定虚拟机
func LockVM(vmName, username string) error {
	return model.UpsertVMLock(vmName, true, username)
}

// UnlockVM 解锁虚拟机
func UnlockVM(vmName, username string) error {
	return model.UpsertVMLock(vmName, false, username)
}

// IsVMLocked 检查虚拟机是否已锁定
func IsVMLocked(vmName string) bool {
	lock := model.GetVMLockOrNil(vmName)
	return lock != nil && lock.Locked
}

// GetVMLockInfo 获取虚拟机锁定详情
func GetVMLockInfo(vmName string) *model.VMLock {
	return model.GetVMLockOrNil(vmName)
}
