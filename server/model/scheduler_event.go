package model

import (
	"fmt"
	"strings"
	"time"
)

// SchedulerEvent 调度事件记录。
type SchedulerEvent struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	SchedulerKey   string     `gorm:"size:100;not null;index:idx_scheduler_event_key_time" json:"scheduler_key"`
	SchedulerName  string     `gorm:"size:100;not null" json:"scheduler_name"`
	SchedulerGroup string     `gorm:"size:100;not null" json:"scheduler_group"`
	VMName         string     `gorm:"size:255;not null;index:idx_scheduler_event_vm" json:"vm_name"`
	VMBackend      string     `gorm:"size:50;not null" json:"vm_backend"`
	Status         string     `gorm:"size:20;not null;index:idx_scheduler_event_status" json:"status"`
	TriggerReason  string     `gorm:"type:text;not null" json:"trigger_reason"`
	ResultMessage  string     `gorm:"type:text" json:"result_message"`
	ErrorMessage   string     `gorm:"type:text" json:"error_message"`
	StartedAt      time.Time  `gorm:"not null;index:idx_scheduler_event_started" json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	CreatedAt      time.Time  `gorm:"index:idx_scheduler_event_created" json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// SchedulerEventFilter 调度事件列表筛选参数。
type SchedulerEventFilter struct {
	Page         int
	PageSize     int
	SchedulerKey string
	Status       string
	VMName       string
	Start        *time.Time
	End          *time.Time
}

// CreateSchedulerEvent 写入调度事件。
func CreateSchedulerEvent(event *SchedulerEvent) error {
	return DB.Create(event).Error
}

// UpdateSchedulerEvent 更新调度事件。
func UpdateSchedulerEvent(event *SchedulerEvent) error {
	return DB.Save(event).Error
}

// ListSchedulerEvents 获取调度事件列表。
func ListSchedulerEvents(filter SchedulerEventFilter) ([]SchedulerEvent, int64, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	query := DB.Model(&SchedulerEvent{})
	if filter.SchedulerKey != "" {
		query = query.Where("scheduler_key = ?", filter.SchedulerKey)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.VMName != "" {
		query = query.Where("vm_name LIKE ?", "%"+filter.VMName+"%")
	}
	if filter.Start != nil {
		query = query.Where("created_at >= ?", filter.Start)
	}
	if filter.End != nil {
		query = query.Where("created_at <= ?", filter.End)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []SchedulerEvent
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// GetSchedulerLatestEventTimes 获取各调度器最近一次事件时间。
func GetSchedulerLatestEventTimes() (map[string]time.Time, error) {
	rows, err := DB.Model(&SchedulerEvent{}).
		Select("scheduler_key, MAX(created_at) AS last_event_at").
		Group("scheduler_key").
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]time.Time)
	for rows.Next() {
		var schedulerKey string
		var rawValue interface{}
		if err := rows.Scan(&schedulerKey, &rawValue); err != nil {
			return nil, err
		}
		parsed, err := parseSchedulerEventDBTime(rawValue)
		if err != nil {
			return nil, err
		}
		result[schedulerKey] = parsed
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// DeleteSchedulerEventsBefore 清理指定时间之前的调度事件。
func DeleteSchedulerEventsBefore(cutoff time.Time) (int64, error) {
	result := DB.Where("created_at < ?", cutoff).Delete(&SchedulerEvent{})
	return result.RowsAffected, result.Error
}

func parseSchedulerEventDBTime(value interface{}) (time.Time, error) {
	switch v := value.(type) {
	case time.Time:
		return v, nil
	case string:
		return parseSchedulerEventTimeString(v)
	case []byte:
		return parseSchedulerEventTimeString(string(v))
	default:
		return time.Time{}, fmt.Errorf("不支持的调度事件时间类型: %T", value)
	}
}

func parseSchedulerEventTimeString(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("调度事件时间为空")
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("无法解析调度事件时间: %s", value)
}
