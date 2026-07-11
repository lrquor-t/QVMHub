package pool

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"qvmhub/config"
	"qvmhub/model"
)

// UpdateHostStoragePoolConfig 更新显示名与启用状态。
func UpdateHostStoragePoolConfig(id string, req UpdateHostStoragePoolConfigRequest) error {
	pool, err := GetStoragePool(id)
	if err != nil {
		return err
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = defaultStorageDisplayName(*pool)
	}
	mountPath := pool.MountPath
	if mountPath == "" {
		mountPath = defaultStorageMountPath(id)
	}
	if req.Enabled && !pool.CanUseForVM {
		return fmt.Errorf("该硬盘当前不能启用为虚拟机存储位置: %s", pool.StatusReason)
	}
	cfg := model.HostStoragePool{DeviceID: id}
	return model.DB.Where("device_id = ?", id).Assign(map[string]interface{}{
		"display_name": displayName,
		"enabled":      req.Enabled,
		"is_default":   pool.IsDefault && req.Enabled,
		"mount_path":   mountPath,
	}).FirstOrCreate(&cfg).Error
}

// SetDefaultHostStoragePool 将指定硬盘设为默认存储位置。
func SetDefaultHostStoragePool(id string) error {
	pool, err := GetStoragePool(id)
	if err != nil {
		return err
	}
	if !pool.CanUseForVM {
		return fmt.Errorf("该硬盘当前不能设为默认存储位置: %s", pool.StatusReason)
	}
	displayName := strings.TrimSpace(pool.DisplayName)
	if displayName == "" {
		displayName = defaultStorageDisplayName(*pool)
	}
	mountPath := pool.MountPath
	if mountPath == "" {
		mountPath = defaultStorageMountPath(id)
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.HostStoragePool{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return err
		}
		cfg := model.HostStoragePool{DeviceID: id}
		return tx.Where("device_id = ?", id).Assign(map[string]interface{}{
			"display_name": displayName,
			"enabled":      true,
			"is_default":   true,
			"mount_path":   mountPath,
		}).FirstOrCreate(&cfg).Error
	})
}

// ResolveVMStorageDir 解析创建虚拟机时使用的磁盘目录。
func ResolveVMStorageDir(poolID string, isAdmin bool) (string, string, error) {
	poolID = strings.TrimSpace(poolID)
	if poolID == "" {
		if cfg, ok := getDefaultHostStoragePoolConfig(); ok {
			poolID = cfg.DeviceID
		} else {
			return config.GlobalConfig.CloneDir, "", nil
		}
	}

	pool, err := GetStoragePool(poolID)
	if err != nil {
		return "", "", err
	}
	if !pool.CanUseForVM {
		return "", "", fmt.Errorf("存储池不可用于创建虚拟机: %s", pool.StatusReason)
	}
	if !isAdmin && !pool.Enabled {
		return "", "", fmt.Errorf("该存储池未启用，普通用户不能使用")
	}
	if err := ensureVMStorageDir(pool.VMDir); err != nil {
		return "", "", err
	}
	return pool.VMDir, pool.ID, nil
}

func configuredCloneDir() string {
	if config.GlobalConfig == nil || strings.TrimSpace(config.GlobalConfig.CloneDir) == "" {
		return ""
	}
	return config.GlobalConfig.CloneDir
}
