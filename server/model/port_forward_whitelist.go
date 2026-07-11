package model

import "time"

const (
	PortForwardWhitelistScopeUser = "user"
	PortForwardWhitelistScopeVM   = "vm"
)

// PortForwardWhitelist 端口转发 HTTP 探测白名单。
type PortForwardWhitelist struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	ScopeType  string    `json:"scope_type" gorm:"size:16;not null;uniqueIndex:idx_port_forward_whitelist_scope"`
	ScopeValue string    `json:"scope_value" gorm:"size:128;not null;uniqueIndex:idx_port_forward_whitelist_scope"`
	CreatedBy  string    `json:"created_by" gorm:"size:64;not null"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (PortForwardWhitelist) TableName() string {
	return "port_forward_whitelists"
}
