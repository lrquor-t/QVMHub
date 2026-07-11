package model

import (
	"time"
)

// VmStatsRecord 虚拟机资源使用历史记录（持久化到数据库）
type VmStatsRecord struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	VMName         string    `gorm:"index;not null" json:"vm_name"`     // 虚拟机名称
	CPUPercent     float64   `json:"cpu_percent"`                       // CPU 使用率 (%)
	MemUsed        int64     `json:"mem_used"`                          // 已用内存 (KB)
	MemTotal       int64     `json:"mem_total"`                         // 总内存 (KB)
	NetRxBytes     int64     `json:"net_rx_bytes"`                      // 网络接收字节数
	NetTxBytes     int64     `json:"net_tx_bytes"`                      // 网络发送字节数
	DiskRdBytes    int64     `json:"disk_rd_bytes"`                     // 磁盘读取字节数
	DiskWrBytes    int64     `json:"disk_wr_bytes"`                     // 磁盘写入字节数
	DiskRdOps      int64     `json:"disk_rd_ops"`                       // 磁盘累计读操作次数
	DiskWrOps      int64     `json:"disk_wr_ops"`                       // 磁盘累计写操作次数
	DiskUsedBytes  int64     `json:"disk_used_bytes"`                   // LXC rootfs 已用（字节）
	DiskTotalBytes int64     `json:"disk_total_bytes"`                  // LXC rootfs 总量（字节）
	RecordedAt     time.Time `gorm:"index;not null" json:"recorded_at"` // 记录时间
}
