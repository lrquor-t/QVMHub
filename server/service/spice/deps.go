package spice

// deps.go — 声明 Hook 变量，避免反向 import service 根包（与 vnc/deps.go 同构）。

var (
	// VM lifecycle hooks（由 service 根包的 *_wire.go 注入）
	HookStartVM        func(name string) error
	HookDetectVMOSType func(templateName, xmlStr string) string

	// 网络相关 hooks（避免 spice 反向 import service 根包）
	// GetHostIP 返回宿主机外网 IP（公网暴露时写进 .vv）
	HookGetHostIP func() string
	// ManageUFWRule 放行/收回防火墙端口：action ∈ {allow,delete}
	HookManageUFWRule func(action, rule string) error
)
