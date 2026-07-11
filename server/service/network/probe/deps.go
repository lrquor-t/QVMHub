package probe

// deps.go — probe 子包通过 Hook 变量调用 service 根包函数，避免循环 import。
// service 根包在 port_forward_probe_register.go 的 init() 中为这些变量赋值。
//
// probe 可以直接 import 以下包（无循环依赖）：
//   - qvmhub/service/network  （类型 + 导出函数）
//   - qvmhub/service/network/vpc
//   - qvmhub/service/firewall
//   - qvmhub/service/scheduler
//   - qvmhub/model / config / logger

var (
	// HookVMExists 检查虚拟机是否存在（替代 service.GetVM）
	HookVMExists func(vmName string) error
	// HookFindVMOwner 查找虚拟机归属用户名（替代 service.FindVMOwner）
	HookFindVMOwner func(vmName string) string
	// HookIsMaintenanceModeEnabled 维护模式是否开启（替代 service.IsMaintenanceModeEnabled）
	HookIsMaintenanceModeEnabled func() bool
)
