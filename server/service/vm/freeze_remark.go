package vm

// ==================== 虚拟机冻结和备注操作 ====================

// GetVMFreeze 获取虚拟机是否启用启动冻结
func GetVMFreeze(name string) (bool, error) {
	metadata, err := readVMConfigMetadata(name)
	if err != nil {
		return false, err
	}
	return metadataFreezeEnabled(metadata), nil
}

// SetVMFreeze 设置虚拟机启动时冻结 CPU
func SetVMFreeze(name string, freeze bool) error {
	if err := D.HookEnsureVMNotMigrating(name, "设置启动冻结"); err != nil {
		return err
	}
	metadata, err := readVMConfigMetadata(name)
	if err != nil {
		return err
	}
	if freeze {
		metadata.Freeze = "yes"
	} else {
		metadata.Freeze = ""
	}
	if err := writeVMConfigMetadata(name, metadata); err != nil {
		return err
	}
	RefreshVMCacheByNameAsync(name)
	return nil
}
