package spice

// types.go — SPICE 显示协议相关类型。

// SpiceInfo SPICE 状态信息（对外只读）
type SpiceInfo struct {
	Enabled  bool   `json:"enabled"`        // domain XML 是否含 graphics type='spice'
	Port     string `json:"port,omitempty"` // info spice 读回的实际端口
	Password bool   `json:"has_password"`   // 是否设置了密码
	Exposed  bool   `json:"exposed"`        // 是否对外暴露（监听 0.0.0.0）
}

// SpiceConnInfo SPICE 连接信息（供 .vv 文件 / 接口返回使用）
type SpiceConnInfo struct {
	Host     string // 对外连接地址（公网 IP）
	Port     string // SPICE 端口
	TLSPort  string // TLS 端口（若启用，当前未启用 TLS）
	Password string // 明文密码（若设置，用于 .vv 文件）
	Exposed  bool   // 是否对外暴露
}
