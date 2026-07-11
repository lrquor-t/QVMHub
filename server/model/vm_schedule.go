package model

import (
	"time"
)

// VMSchedule 虚拟机定时任务配置。
type VMSchedule struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	VMName          string     `gorm:"size:255;not null;index:idx_vm_schedule_vm" json:"vm_name"`
	EventType       string     `gorm:"size:32;not null;index:idx_vm_schedule_event" json:"event_type"`
	Action          string     `gorm:"size:32;not null" json:"action"`
	ScheduleType    string     `gorm:"size:32;not null" json:"schedule_type"`
	RunAt           *time.Time `json:"run_at"`
	Timezone        string     `gorm:"size:64;not null" json:"timezone"`
	TimeOfDay       string     `gorm:"size:16" json:"time_of_day"`
	Weekdays        string     `gorm:"size:64" json:"weekdays"`
	Enabled         bool       `gorm:"not null;default:true;index:idx_vm_schedule_enabled_next" json:"enabled"`
	NextRunAt       *time.Time `gorm:"index:idx_vm_schedule_enabled_next" json:"next_run_at"`
	LastTaskID      *uint      `json:"last_task_id"`
	LastTriggeredAt *time.Time `json:"last_triggered_at"`
	LastFinishedAt  *time.Time `json:"last_finished_at"`
	LastStatus      string     `gorm:"size:32" json:"last_status"`
	LastMessage     string     `gorm:"type:text" json:"last_message"`
	CreatedBy       string     `gorm:"size:100;not null" json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// TableName 指定表名。
func (VMSchedule) TableName() string {
	return "vm_schedules"
}

// CreateVMSchedule 创建定时任务。
func CreateVMSchedule(schedule *VMSchedule) error {
	return DB.Create(schedule).Error
}

// UpdateVMSchedule 保存定时任务。
func UpdateVMSchedule(schedule *VMSchedule) error {
	return DB.Save(schedule).Error
}

// UpdateVMScheduleFields 按字段更新定时任务。
func UpdateVMScheduleFields(id uint, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now()
	return DB.Model(&VMSchedule{}).Where("id = ?", id).Updates(fields).Error
}

// GetVMScheduleByIDAndVM 获取指定虚拟机的定时任务。
func GetVMScheduleByIDAndVM(id uint, vmName string) (*VMSchedule, error) {
	var schedule VMSchedule
	if err := DB.Where("id = ? AND vm_name = ?", id, vmName).First(&schedule).Error; err != nil {
		return nil, err
	}
	return &schedule, nil
}

// GetVMScheduleByID 获取定时任务。
func GetVMScheduleByID(id uint) (*VMSchedule, error) {
	var schedule VMSchedule
	if err := DB.Where("id = ?", id).First(&schedule).Error; err != nil {
		return nil, err
	}
	return &schedule, nil
}

// ListVMSchedulesByVM 获取虚拟机的全部定时任务。
func ListVMSchedulesByVM(vmName string) ([]VMSchedule, error) {
	var list []VMSchedule
	if err := DB.Where("vm_name = ?", vmName).
		Order("enabled DESC").
		Order("CASE WHEN next_run_at IS NULL THEN 1 ELSE 0 END").
		Order("next_run_at ASC").
		Order("id DESC").
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListDueVMSchedules 获取当前到期的定时任务。
func ListDueVMSchedules(now time.Time, limit int) ([]VMSchedule, error) {
	if limit <= 0 {
		limit = 20
	}
	var list []VMSchedule
	if err := DB.Where("enabled = ? AND next_run_at IS NOT NULL AND next_run_at <= ?", true, now).
		Order("next_run_at ASC").
		Limit(limit).
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// DeleteVMSchedule 删除单个定时任务。
func DeleteVMSchedule(id uint) error {
	return DB.Delete(&VMSchedule{}, id).Error
}

// DeleteVMSchedulesByVM 删除虚拟机关联的全部定时任务。
func DeleteVMSchedulesByVM(vmName string) error {
	return DB.Where("vm_name = ?", vmName).Delete(&VMSchedule{}).Error
}
