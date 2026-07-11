package service

import rescuepkg "qvmhub/service/rescue"

// init wires rescue package hook variables to service root implementations.
// This breaks the circular dependency: rescue package cannot import service,
// so it exposes hook variables that we set here.
func init() {
	rescuepkg.HookEnsureVMNotMigrating = EnsureVMNotMigrating
	rescuepkg.HookDestroyVM = DestroyVM
	rescuepkg.HookSetVMBootOrder = SetVMBootOrder
	rescuepkg.HookStartVM = StartVM
	rescuepkg.HookSetVMNicModel = SetVMNicModel
}

// ── Exported delegates ──

func StartRescue(vmName, rescueISO string, progress func(int, string)) error {
	return rescuepkg.StartRescue(vmName, rescueISO, progress)
}

func StopRescue(vmName string, progress func(int, string)) error {
	return rescuepkg.StopRescue(vmName, progress)
}

func IsInRescueMode(vmName string) bool {
	return rescuepkg.IsInRescueMode(vmName)
}
