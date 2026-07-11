package model

import (
	"time"
)

// VMLock 虚拟机锁定状态
type VMLock struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	VMName    string    `gorm:"uniqueIndex;size:255;not null" json:"vm_name"`
	Locked    bool      `gorm:"index;not null;default:false" json:"locked"`
	LockedAt  time.Time `json:"locked_at"`
	LockedBy  string    `gorm:"index;size:100;not null;default:''" json:"locked_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (VMLock) TableName() string {
	return "vm_locks"
}

// GetVMLock 获取虚拟机锁定状态
func GetVMLock(vmName string) (*VMLock, error) {
	var lock VMLock
	err := DB.Where("vm_name = ?", vmName).First(&lock).Error
	if err != nil {
		return nil, err
	}
	return &lock, nil
}

// GetVMLockOrNil 获取锁定状态，不存在时返回 nil 而非错误
func GetVMLockOrNil(vmName string) *VMLock {
	lock, err := GetVMLock(vmName)
	if err != nil {
		return nil
	}
	return lock
}

// UpsertVMLock 创建或更新锁定状态
func UpsertVMLock(vmName string, locked bool, lockedBy string) error {
	now := time.Now()
	lock := VMLock{
		VMName:   vmName,
		Locked:   locked,
		LockedAt: now,
		LockedBy: lockedBy,
	}
	// NOTE: GORM Where struct ignores zero-value fields
	return DB.Where(VMLock{VMName: vmName}).
		Assign(map[string]interface{}{
			"locked":    locked,
			"locked_at": now,
			"locked_by": lockedBy,
		}).
		FirstOrCreate(&lock).Error
}

// DeleteVMLock 删除虚拟机锁定记录（虚拟机删除时调用）
func DeleteVMLock(vmName string) error {
	return DB.Where("vm_name = ?", vmName).Delete(&VMLock{}).Error
}

// BatchIsVMLocked 批量查询虚拟机锁定状态，返回 map[vmName]bool
func BatchIsVMLocked(vmNames []string) map[string]bool {
	result := make(map[string]bool, len(vmNames))
	if len(vmNames) == 0 {
		return result
	}

	var locks []VMLock
	if err := DB.Where("vm_name IN ? AND locked = ?", vmNames, true).Find(&locks).Error; err != nil {
		return result
	}
	for _, lock := range locks {
		result[lock.VMName] = true
	}
	return result
}
