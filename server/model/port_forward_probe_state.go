package model

import "time"

// PortForwardProbeState 记录端口转发 HTTP 探测状态和封禁快照。
type PortForwardProbeState struct {
	ID                 uint       `json:"id" gorm:"primaryKey"`
	RuleKey            string     `json:"rule_key" gorm:"size:255;not null;uniqueIndex"`
	Protocol           string     `json:"protocol" gorm:"size:16;not null"`
	HostPort           string     `json:"host_port" gorm:"size:32;not null"`
	DestIP             string     `json:"dest_ip" gorm:"size:64;not null"`
	DestPort           string     `json:"dest_port" gorm:"size:32;not null"`
	VMName             string     `json:"vm_name" gorm:"index;size:128"`
	OwnerUsername      string     `json:"owner_username" gorm:"index;size:64"`
	CreatedBy          string     `json:"created_by" gorm:"size:64"`
	CreatedByAdmin     bool       `json:"created_by_admin"`
	Live               bool       `json:"live"`
	Banned             bool       `json:"banned" gorm:"index"`
	WhitelistScope     string     `json:"whitelist_scope" gorm:"size:32"`
	LastCheckedAt      *time.Time `json:"last_checked_at"`
	LastHTTPStatusCode int        `json:"last_http_status_code"`
	LastResult         string     `json:"last_result" gorm:"size:64"`
	LastError          string     `json:"last_error" gorm:"type:text"`
	BanReason          string     `json:"ban_reason" gorm:"type:text"`
	BannedAt           *time.Time `json:"banned_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (PortForwardProbeState) TableName() string {
	return "port_forward_probe_states"
}
