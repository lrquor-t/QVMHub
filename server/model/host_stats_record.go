package model

import "time"

// HostStatsRecord 宿主机资源历史记录
type HostStatsRecord struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	CPUPercent  float64   `gorm:"type:decimal(5,2);not null;comment:CPU使用率" json:"cpu_percent"`
	MemUsed     int64     `gorm:"not null;comment:已用内存(KB)" json:"mem_used"`
	MemTotal    int64     `gorm:"not null;comment:总内存(KB)" json:"mem_total"`
	NetRxBytes  int64     `gorm:"not null;comment:网络接收字节(累加)" json:"net_rx_bytes"`
	NetTxBytes  int64     `gorm:"not null;comment:网络发送字节(累加)" json:"net_tx_bytes"`
	DiskRdBytes int64     `gorm:"not null;comment:磁盘读取字节(累加)" json:"disk_rd_bytes"`
	DiskWrBytes int64     `gorm:"not null;comment:磁盘写入字节(累加)" json:"disk_wr_bytes"`
	RecordedAt  time.Time `gorm:"index;not null;comment:记录时间" json:"recorded_at"`
}
