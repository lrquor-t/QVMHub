package model

import "time"

// LightweightVMQuota 轻量云单台虚拟机配额配置。
type LightweightVMQuota struct {
	ID                      uint       `json:"id" gorm:"primaryKey"`
	Username                string     `json:"username" gorm:"index;not null;size:64"`
	VMName                  string     `json:"vm_name" gorm:"uniqueIndex;not null;size:128"`
	TrafficDownGB           float64    `json:"traffic_down_gb" gorm:"default:0"`
	TrafficUpGB             float64    `json:"traffic_up_gb" gorm:"default:0"`
	BandwidthDownMbps       int        `json:"bandwidth_down_mbps" gorm:"default:0"`
	BandwidthUpMbps         int        `json:"bandwidth_up_mbps" gorm:"default:0"`
	MaxPortForwards         int        `json:"max_port_forwards" gorm:"default:10"`
	MaxSnapshots            int        `json:"max_snapshots" gorm:"default:2"`
	MaxRuntimeHours         int        `json:"max_runtime_hours" gorm:"default:0"`
	UsedRuntimeSeconds      int64      `json:"used_runtime_seconds" gorm:"default:0"`
	RuntimeIsActive         bool       `json:"-" gorm:"default:false"`
	RuntimeLastObservedAt   *time.Time `json:"-"`
	RuntimeWarningSentAt    *time.Time `json:"-"`
	UsedTrafficDown         int64      `json:"used_traffic_down" gorm:"-"`
	UsedTrafficUp           int64      `json:"used_traffic_up" gorm:"-"`
	UsedTrafficDownGB       string     `json:"used_traffic_down_gb" gorm:"-"`
	UsedTrafficUpGB         string     `json:"used_traffic_up_gb" gorm:"-"`
	UsedRuntimeDisplay      string     `json:"used_runtime_display" gorm:"-"`
	RemainingRuntimeSeconds int64      `json:"remaining_runtime_seconds" gorm:"-"`
	RemainingRuntimeDisplay string     `json:"remaining_runtime_display" gorm:"-"`
	RuntimeQuotaReached     bool       `json:"runtime_quota_reached" gorm:"-"`
	CurrentNetRxBytes       int64      `json:"current_net_rx_bytes" gorm:"-"`
	CurrentNetTxBytes       int64      `json:"current_net_tx_bytes" gorm:"-"`
	CurrentNetRxRate        string     `json:"current_net_rx_rate" gorm:"-"`
	CurrentNetTxRate        string     `json:"current_net_tx_rate" gorm:"-"`
	IsLimitedDown           bool       `json:"is_limited_down" gorm:"-"`
	IsLimitedUp             bool       `json:"is_limited_up" gorm:"-"`
	UsedPortForwards        int        `json:"used_port_forwards" gorm:"-"`
	UsedSnapshots           int        `json:"used_snapshots" gorm:"-"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

func (LightweightVMQuota) TableName() string {
	return "lightweight_vm_quotas"
}

// LightweightVMTrafficMonthly 保存轻量云单 VM 的月流量状态。
type LightweightVMTrafficMonthly struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	VMName        string    `json:"vm_name" gorm:"uniqueIndex:idx_light_vm_month;not null;size:128"`
	Username      string    `json:"username" gorm:"index;not null;size:64"`
	Month         string    `json:"month" gorm:"uniqueIndex:idx_light_vm_month;not null;size:7"`
	TrafficDown   int64     `json:"traffic_down"`
	TrafficUp     int64     `json:"traffic_up"`
	OffsetDown    int64     `json:"offset_down"`
	OffsetUp      int64     `json:"offset_up"`
	IsLimitedDown bool      `json:"is_limited_down"`
	IsLimitedUp   bool      `json:"is_limited_up"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (LightweightVMTrafficMonthly) TableName() string {
	return "lightweight_vm_traffic_monthlies"
}
