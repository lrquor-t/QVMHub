package model

import "time"

// LXCCache 保存用于列表类接口的 LXC 容器缓存投影（与 VMCache 同步模式一致）。
type LXCCache struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	Name           string    `json:"name" gorm:"uniqueIndex;size:128;not null"`
	OwnerUsername  string    `json:"owner_username" gorm:"index;size:64;not null;default:'admin'"`
	Status         string    `json:"status" gorm:"index;size:64"` // running/stopped/frozen/...
	Template       string    `json:"template" gorm:"size:255"`    // 来源模板名
	CPUShares      int       `json:"cpu_shares"`                  // cgroup cpu.weight（展示）
	MemoryMB       int       `json:"memory_mb"`                   // cgroup memory.max（展示）
	RootfsSizeText string    `json:"rootfs_size_text" gorm:"size:64"`
	Backing        string    `json:"backing" gorm:"size:32;default:'overlay'"` // overlay/dir/btrfs/zfs/lvm
	SnapshotCount  int       `json:"snapshot_count"`
	Autostart      bool      `json:"autostart"`
	Remark         string    `json:"remark" gorm:"size:255"`
	GroupName      string    `json:"group_name" gorm:"index;size:128"`
	MacAddress     string    `json:"mac_address" gorm:"size:64"`
	VethName       string    `json:"veth_name" gorm:"size:64"` // host 侧 veth（运行态解析回填）
	CachedIP       string    `json:"cached_ip" gorm:"size:64"`
	BandwidthIn    int       `json:"bandwidth_in"`
	BandwidthOut   int       `json:"bandwidth_out"`
	Present        bool      `json:"present" gorm:"index;not null;default:true"`
	LastSyncedAt   time.Time `json:"last_synced_at" gorm:"index"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (LXCCache) TableName() string { return "lxc_containers" }
