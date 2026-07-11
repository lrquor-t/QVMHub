package vnc

// VncInfo VNC 状态信息
type VncInfo struct {
	Enabled  bool   `json:"enabled"`
	Port     string `json:"port"`
	Auth     string `json:"auth"`
	Password bool   `json:"has_password"`
	Socket   string `json:"socket,omitempty"`
	Exposed  bool   `json:"exposed"` // 是否对外暴露（监听 0.0.0.0）
}

// VncConnInfo VNC 连接信息（供 WebSocket 代理使用）
type VncConnInfo struct {
	Network string // "unix" 或 "tcp"
	Address string // socket 路径 或 host:port
}
