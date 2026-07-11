package model

import (
	"time"
)

// LXCSchedule LXC 容器定时任务配置。
type LXCSchedule struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	Name            string     `gorm:"size:255;not null;index:idx_lxc_schedule_name" json:"name"`
	EventType       string     `gorm:"size:32;not null;index:idx_lxc_schedule_event" json:"event_type"`
	Action          string     `gorm:"size:32;not null" json:"action"`
	ScheduleType    string     `gorm:"size:32;not null" json:"schedule_type"`
	RunAt           *time.Time `json:"run_at"`
	Timezone        string     `gorm:"size:64;not null" json:"timezone"`
	TimeOfDay       string     `gorm:"size:16" json:"time_of_day"`
	Weekdays        string     `gorm:"size:64" json:"weekdays"`
	Enabled         bool       `gorm:"not null;default:true;index:idx_lxc_schedule_enabled_next" json:"enabled"`
	NextRunAt       *time.Time `gorm:"index:idx_lxc_schedule_enabled_next" json:"next_run_at"`
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
func (LXCSchedule) TableName() string {
	return "lxc_schedules"
}

// CreateLXCSchedule 创建定时任务。
func CreateLXCSchedule(schedule *LXCSchedule) error {
	return DB.Create(schedule).Error
}

// UpdateLXCSchedule 保存定时任务。
func UpdateLXCSchedule(schedule *LXCSchedule) error {
	return DB.Save(schedule).Error
}

// UpdateLXCScheduleFields 按字段更新定时任务。
func UpdateLXCScheduleFields(id uint, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now()
	return DB.Model(&LXCSchedule{}).Where("id = ?", id).Updates(fields).Error
}

// GetLXCScheduleByIDAndContainer 获取指定容器的定时任务。
func GetLXCScheduleByIDAndContainer(id uint, name string) (*LXCSchedule, error) {
	var schedule LXCSchedule
	if err := DB.Where("id = ? AND name = ?", id, name).First(&schedule).Error; err != nil {
		return nil, err
	}
	return &schedule, nil
}

// GetLXCScheduleByID 获取定时任务。
func GetLXCScheduleByID(id uint) (*LXCSchedule, error) {
	var schedule LXCSchedule
	if err := DB.Where("id = ?", id).First(&schedule).Error; err != nil {
		return nil, err
	}
	return &schedule, nil
}

// ListLXCSchedulesByContainer 获取容器的全部定时任务。
func ListLXCSchedulesByContainer(name string) ([]LXCSchedule, error) {
	var list []LXCSchedule
	if err := DB.Where("name = ?", name).
		Order("enabled DESC").
		Order("CASE WHEN next_run_at IS NULL THEN 1 ELSE 0 END").
		Order("next_run_at ASC").
		Order("id DESC").
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ListDueLXCSchedules 获取当前到期的定时任务。
func ListDueLXCSchedules(now time.Time, limit int) ([]LXCSchedule, error) {
	if limit <= 0 {
		limit = 20
	}
	var list []LXCSchedule
	if err := DB.Where("enabled = ? AND next_run_at IS NOT NULL AND next_run_at <= ?", true, now).
		Order("next_run_at ASC").
		Limit(limit).
		Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// DeleteLXCSchedule 删除单个定时任务。
func DeleteLXCSchedule(id uint) error {
	return DB.Delete(&LXCSchedule{}, id).Error
}

// DeleteLXCSchedulesByContainer 删除容器关联的全部定时任务。
func DeleteLXCSchedulesByContainer(name string) error {
	return DB.Where("name = ?", name).Delete(&LXCSchedule{}).Error
}
