package model

import "time"

// SecurityChallenge 邮箱验证码与短时挑战
type SecurityChallenge struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	UserID     uint       `json:"user_id" gorm:"index;not null"`
	Purpose    string     `json:"purpose" gorm:"size:64;index;not null"`
	Method     string     `json:"method" gorm:"size:32;not null"`
	Target     string     `json:"target" gorm:"size:255"`
	CodeHash   string     `json:"-" gorm:"size:128;not null"`
	ExpiresAt  time.Time  `json:"expires_at" gorm:"index;not null"`
	ConsumedAt *time.Time `json:"consumed_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (SecurityChallenge) TableName() string {
	return "security_challenges"
}
