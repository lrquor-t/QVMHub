// Package nodereg implements QVMHub's node registry health layer: HTTP-only
// probing of registered QVM nodes, an in-memory health cache, and a background
// probe scheduler. It reuses model.HostNode + service.CallNodeAPI and never
// SSHes to a node (design §5.1–5.3).
package nodereg

import "time"

// 节点状态枚举(与 model.HostNode.Status 文本一致)。
const (
	StatusUnknown  = "unknown"
	StatusOnline   = "online"
	StatusOffline  = "offline"
	StatusDisabled = "disabled"
)

// NodeStats 是 /api/host/stats 的子集(与 QVMConsole service/vm/types.go HostStats 对齐)。
type NodeStats struct {
	CPUPercent float64 `json:"cpu_percent"`
	MemTotal   int64   `json:"mem_total"`  // KB
	MemUsed    int64   `json:"mem_used"`   // KB
	MemAvail   int64   `json:"mem_available"`
	DiskTotal  int64   `json:"disk_total"` // KB
	DiskUsed   int64   `json:"disk_used"`  // KB
	VMRunning  int     `json:"vm_running"`
	VMTotal    int     `json:"vm_total"`
	Hostname   string  `json:"hostname"`
	Uptime     string  `json:"uptime"`
	Arch       string  `json:"arch"`
}

// NodeHealth 是某节点在缓存中的健康快照(总览页只读此结构,零请求时扇出)。
type NodeHealth struct {
	NodeID     uint       `json:"node_id"`
	Name       string     `json:"name"`
	APIBaseURL string     `json:"api_base_url"`
	Enabled    bool       `json:"enabled"`
	Online     bool       `json:"online"`
	Status     string     `json:"status"` // online / offline / unknown
	Version    string     `json:"version,omitempty"`
	Stats      *NodeStats `json:"stats,omitempty"`
	LastSeen   *time.Time `json:"last_seen"`
	LastError  string     `json:"last_error,omitempty"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
