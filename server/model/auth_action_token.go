package model

import "time"

// AuthActionToken 邮件链接类令牌
type AuthActionToken struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	UserID     uint       `json:"user_id" gorm:"index;not null"`
	Purpose    string     `json:"purpose" gorm:"size:64;index;not null"`
	TokenHash  string     `json:"-" gorm:"size:128;uniqueIndex;not null"`
	ExpiresAt  time.Time  `json:"expires_at" gorm:"index;not null"`
	ConsumedAt *time.Time `json:"consumed_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (AuthActionToken) TableName() string {
	return "auth_action_tokens"
}
