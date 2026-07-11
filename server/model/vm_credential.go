package model

import "time"

// VMCredential 虚拟机登录凭据
type VMCredential struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	VMName      string     `json:"vm_name" gorm:"uniqueIndex;size:128;not null"`
	Username    string     `json:"username" gorm:"index;size:64;not null"`
	PasswordEnc string     `json:"-" gorm:"type:text;not null"`
	Source      string     `json:"source" gorm:"size:32;not null;default:'create'"`
	Operator    string     `json:"operator" gorm:"size:64"`
	LastResetAt *time.Time `json:"last_reset_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (VMCredential) TableName() string {
	return "vm_credentials"
}
