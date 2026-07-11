package model

import "time"

// HostStoragePool 保存宿主机硬盘在面板中的管理配置。
// 硬盘容量、挂载状态等运行时信息始终通过系统命令读取，数据库只保存展示和调度策略。
type HostStoragePool struct {
	DeviceID    string    `gorm:"primaryKey;size:200" json:"device_id"`
	DisplayName string    `gorm:"size:100" json:"display_name"`
	Enabled     bool      `json:"enabled"`
	IsDefault   bool      `json:"is_default"`
	MountPath   string    `gorm:"size:500" json:"mount_path"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
