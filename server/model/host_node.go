package model

import "time"

// HostNode 保存可迁移目标节点的连接信息。
type HostNode struct {
	ID               uint       `json:"id" gorm:"primaryKey"`
	Name             string     `json:"name" gorm:"index;size:80;not null"`
	APIBaseURL       string     `json:"api_base_url" gorm:"size:255;not null"`
	APIKeyID         string     `json:"api_key_id" gorm:"size:120"`
	APIKeyEnc        string     `json:"-" gorm:"type:text"`
	SSHHost          string     `json:"ssh_host" gorm:"size:255;not null"`
	SSHPort          int        `json:"ssh_port" gorm:"default:22"`
	SSHUser          string     `json:"ssh_user" gorm:"size:64;not null;default:'root'"`
	SSHPasswordEnc   string     `json:"-" gorm:"type:text"`
	Enabled          bool       `json:"enabled" gorm:"index;default:true"`
	Status           string     `json:"status" gorm:"index;size:32;default:'unknown'"`
	LastProbeMessage string     `json:"last_probe_message" gorm:"type:text"`
	CapabilitiesJSON string     `json:"capabilities_json" gorm:"type:text"`
	LastProbedAt     *time.Time `json:"last_probed_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (HostNode) TableName() string {
	return "host_nodes"
}
