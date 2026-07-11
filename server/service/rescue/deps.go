package rescue

// deps.go — 声明 Hook 变量，避免反向 import service 根包。

var (
	// VM lifecycle hooks
	HookEnsureVMNotMigrating func(vmName, action string) error
	HookDestroyVM            func(name string) error
	HookSetVMBootOrder       func(name string, bootOrder []string) error
	HookStartVM              func(name string) error
	HookSetVMNicModel        func(name, nicModel string) error
)
