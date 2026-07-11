package service

import (
	"qvmhub/service/vm/memory"
)

// hooks_init.go - 将 service 根包的函数注入到 memory 子包的反向依赖 Hook 变量中，
// 供子包通过 Hook 间接调用根包函数，避免循环 import。

func init() {
	// Register memory package functions into service hooks
	// (moved from memory/register.go to avoid import cycle)
	HookApplyPendingVMMemoryConfig = memory.ApplyPendingVMMemoryConfig
	HookGetVMMemoryDynamicInfo = func(name, xmlStr, state string) any {
		return memory.GetVMMemoryDynamicInfo(name, xmlStr, state)
	}

	// Register memory hooks (service root → memory sub-package)
	memory.HookMemoryGetCachedStats = func(name string) *memory.VmStats {
		// memory 包仅用 VmStats 判非 nil，不需要实际字段
		if GetCachedStats(name) != nil {
			return &memory.VmStats{}
		}
		return nil
	}
	memory.HookMemoryGetHostStats = func() (*memory.HostStats, error) {
		stats, err := GetHostStats()
		if err != nil || stats == nil {
			return nil, err
		}
		return &memory.HostStats{
			MemTotal: stats.MemTotal,
			MemFree:  stats.MemFree,
		}, nil
	}
	memory.HookMemoryIsMaintenanceModeEnabled = func() bool {
		return IsMaintenanceModeEnabled()
	}
	memory.HookMemoryInjectMemballoonConfig = func(xmlStr string, enableFPR bool) string {
		return InjectMemballoonConfig(xmlStr, enableFPR)
	}
}
