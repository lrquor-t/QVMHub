package user

import "qvmhub/model"

// VMUserInfo 用户信息（含分配的虚拟机和配额）
type VMUserInfo struct {
	ID                         uint                            `json:"id"`
	Username                   string                          `json:"username"`
	Email                      string                          `json:"email"`
	Role                       string                          `json:"role"`
	CloudType                  string                          `json:"cloud_type"`
	DedicatedVPCSwitchID       uint                            `json:"dedicated_vpc_switch_id"`
	Status                     string                          `json:"status"`
	MaxCPU                     int                             `json:"max_cpu"`
	MaxMemory                  int                             `json:"max_memory"`
	MaxDisk                    int                             `json:"max_disk"`
	MaxVM                      int                             `json:"max_vm"`
	MaxStorage                 int                             `json:"max_storage"`
	MaxRuntimeHours            int                             `json:"max_runtime_hours"`
	EnablePortForward          bool                            `json:"enable_port_forward"`
	MaxPortForwards            int                             `json:"max_port_forwards"`
	MaxSnapshots               int                             `json:"max_snapshots"`
	MaxBandwidthUp             float64                         `json:"max_bandwidth_up"`
	MaxBandwidthDown           float64                         `json:"max_bandwidth_down"`
	MaxTrafficDown             float64                         `json:"max_traffic_down"`
	MaxTrafficUp               float64                         `json:"max_traffic_up"`
	MaxPublicIPs               int                             `json:"max_public_ips"`
	SSHEnabled                 bool                            `json:"ssh_enabled"`
	VMs                        []string                        `json:"vms"`
	Quota                      *QuotaUsage                     `json:"quota"`
	LightweightVMQuotas        []model.LightweightVMQuota      `json:"lightweight_quotas,omitempty"`
	LightweightVMRegistrations []LightweightVMRegistrationView `json:"lightweight_vm_registrations,omitempty"`
}

// UserStatusChangeResult 用户状态变更结果
type UserStatusChangeResult struct {
	Username   string   `json:"username"`
	Status     string   `json:"status"`
	StoppedVMs []string `json:"stopped_vms,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

// QuotaUsage 配额使用情况
type QuotaUsage struct {
	UsedCPU                 int     `json:"used_cpu"`
	UsedMemory              int     `json:"used_memory"`
	UsedDisk                int     `json:"used_disk"`
	UsedVM                  int     `json:"used_vm"`
	UsedStorage             int64   `json:"used_storage"`
	UsedStorageGB           string  `json:"used_storage_gb"`
	UsedRuntimeSeconds      int64   `json:"used_runtime_seconds"`
	UsedRuntimeDisplay      string  `json:"used_runtime_display"`
	UsedPortForwards        int     `json:"used_port_forwards"`
	UsedSnapshots           int     `json:"used_snapshots"`
	EnablePortForward       bool    `json:"enable_port_forward"`
	MaxCPU                  int     `json:"max_cpu"`
	MaxMemory               int     `json:"max_memory"`
	MaxDisk                 int     `json:"max_disk"`
	MaxVM                   int     `json:"max_vm"`
	MaxStorage              int     `json:"max_storage"`
	MaxRuntimeHours         int     `json:"max_runtime_hours"`
	MaxPortForwards         int     `json:"max_port_forwards"`
	MaxSnapshots            int     `json:"max_snapshots"`
	MaxBandwidthUp          float64 `json:"max_bandwidth_up"`
	MaxBandwidthDown        float64 `json:"max_bandwidth_down"`
	MaxTrafficDown          float64 `json:"max_traffic_down"`
	MaxTrafficUp            float64 `json:"max_traffic_up"`
	MaxPublicIPs            int     `json:"max_public_ips"`
	UsedPublicIPs           int     `json:"used_public_ips"`
	UsedTrafficDown         int64   `json:"used_traffic_down"`
	UsedTrafficUp           int64   `json:"used_traffic_up"`
	UsedTrafficDownGB       string  `json:"used_traffic_down_gb"`
	UsedTrafficUpGB         string  `json:"used_traffic_up_gb"`
	IsLimitedDown           bool    `json:"is_limited_down"`
	IsLimitedUp             bool    `json:"is_limited_up"`
	RemainingRuntimeSeconds int64   `json:"remaining_runtime_seconds"`
	RemainingRuntimeDisplay string  `json:"remaining_runtime_display"`
	RuntimeQuotaReached     bool    `json:"runtime_quota_reached"`
}

// UserStorageInfo 用户存储池信息
type UserStorageInfo struct {
	Initialized bool   `json:"initialized"`
	UsedBytes   int64  `json:"used_bytes"`
	UsedDisplay string `json:"used_display"`
	MaxStorage  int    `json:"max_storage"`
	MaxBytes    int64  `json:"max_bytes"`
	Readonly    bool   `json:"readonly"`
	ISODir      string `json:"iso_dir"`
	ShareDir    string `json:"share_dir"`
	DiskDir     string `json:"disk_dir"`
}

// UserFileInfo 用户文件信息
type UserFileInfo struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	SizeText  string `json:"size_text"`
	ModTime   string `json:"mod_time"`
	Path      string `json:"path"`
	OSType    string `json:"os_type,omitempty"`
	OSVariant string `json:"os_variant,omitempty"`
}

// UserRuntimeQuotaSnapshot 用户运行时长配额快照
type UserRuntimeQuotaSnapshot struct {
	UsedSeconds      int64
	RemainingSeconds int64
	QuotaReached     bool
}

// RuntimeQuotaShutdownResult 运行时长配额触发的关机结果
type RuntimeQuotaShutdownResult struct {
	Username   string   `json:"username"`
	StoppedVMs []string `json:"stopped_vms,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

// 存储池类别常量
const (
	StorageCategoryISO   = "iso"
	StorageCategoryShare = "share"
	StorageCategoryDisk  = "disk"
)
