package firewall

// deps.go — firewall 子包通过 Hook 变量调用 service 根包函数，避免循环 import。
// service 根包在 firewall_register.go 的 init() 中为这些变量赋值。

var (
	// HookOvsBridgeName returns the OVS bridge name (e.g. "br0").
	HookOvsBridgeName func() string

	// HookUseOVSNetwork returns whether OVS network mode is enabled.
	HookUseOVSNetwork func() bool

	// HookVPCGatewayPortName returns the gateway port name for a VPC switch.
	HookVPCGatewayPortName func(id uint) string

	// HookListLivePortForwardsFromIPTables returns the current live port forward rules.
	HookListLivePortForwardsFromIPTables func() ([]PortForwardRule, error)
)
