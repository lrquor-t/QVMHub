package pool

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"qvmhub/model"
	"qvmhub/utils"
)

// DeleteAllPartitionsOnDisk 删除指定磁盘上所有分区（先卸载，再清除分区表）。
// 也支持直接格式化挂载的无分区磁盘：卸载 → 清除文件系统签名。
func DeleteAllPartitionsOnDisk(ctx context.Context, deviceID string, progress func(int, string)) error {
	pool, err := GetStoragePool(deviceID)
	if err != nil {
		return err
	}
	if err := validateDeletePartitionTarget(*pool); err != nil {
		return err
	}

	devicePath := pool.DevicePath
	hasChildren := len(pool.Children) > 0

	// 收集所有需要卸载的挂载点（子分区 + 磁盘自身直接挂载）
	progress(10, "正在收集挂载信息...")
	mountpoints := collectAllMountpoints(pool.Children)
	if !hasChildren && len(pool.Mountpoints) > 0 {
		mountpoints = append(mountpoints, pool.Mountpoints...)
	}

	// 卸载
	if len(mountpoints) > 0 {
		progress(20, fmt.Sprintf("正在卸载 %d 个挂载点...", len(mountpoints)))
		for _, mp := range mountpoints {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			progress(20, fmt.Sprintf("正在卸载 %s ...", mp))
			result := utils.ExecCommandContextWithTimeout(ctx, "umount", 2*time.Minute, mp)
			if result.Error != nil {
				// 尝试 lazy unmount
				lazyResult := utils.ExecCommandContextWithTimeout(ctx, "umount", 2*time.Minute, "-l", mp)
				if lazyResult.Error != nil {
					return fmt.Errorf("卸载 %s 失败: %s", mp, result.Stderr)
				}
			}
		}
	}

	// 移除 fstab 中的相关条目
	progress(40, "正在清理开机自动挂载配置...")
	if err := removeFstabEntriesForDisk(pool, devicePath); err != nil {
		return fmt.Errorf("清理 fstab 失败: %w", err)
	}

	// 清除数据库中的存储池配置
	progress(50, "正在清除存储池配置...")
	if err := clearStoragePoolConfigs(pool); err != nil {
		return fmt.Errorf("清除存储池配置失败: %w", err)
	}

	// 清除分区表 / 文件系统签名
	if hasChildren {
		progress(70, "正在清除分区表...")
	} else {
		progress(70, "正在清除文件系统签名...")
	}
	wipeResult := utils.ExecCommandContextWithTimeout(ctx, "wipefs", 2*time.Minute, "-a", devicePath)
	if wipeResult.Error != nil {
		return fmt.Errorf("清除失败: %s", wipeResult.Stderr)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// 刷新内核分区表
	progress(90, "正在刷新分区表...")
	probeResult := utils.ExecCommandContextWithTimeout(ctx, "partprobe", 30*time.Second, devicePath)
	if probeResult.Error != nil {
		utils.ExecShellQuiet(fmt.Sprintf("partprobe %s 2>/dev/null || true", utils.ShellSingleQuote(devicePath)))
	}

	if hasChildren {
		progress(100, "所有分区已删除")
	} else {
		progress(100, "磁盘已清除")
	}
	return nil
}

// validateDeletePartitionTarget 验证磁盘是否可清除（删除分区或清除直接挂载）。
func validateDeletePartitionTarget(pool HostStoragePoolInfo) error {
	if pool.Type != "disk" {
		return fmt.Errorf("只能对整块磁盘操作，当前设备类型为 %s", pool.Type)
	}
	if pool.Readonly {
		return fmt.Errorf("设备为只读状态")
	}
	if pool.SystemDisk {
		return fmt.Errorf("系统关键磁盘禁止此操作")
	}
	// 有子分区：允许（会递归卸载子分区）
	if len(pool.Children) > 0 {
		return nil
	}
	// 无子分区但整盘直接挂载：允许（会卸载自身后清除）
	if len(pool.Mountpoints) > 0 {
		return nil
	}
	// 既无分区又无挂载：无需操作
	return fmt.Errorf("磁盘上没有分区或挂载，无需清除")
}

// collectAllMountpoints 递归收集所有子节点的挂载点。
func collectAllMountpoints(children []HostStoragePoolInfo) []string {
	var mps []string
	for _, child := range children {
		if len(child.Mountpoints) > 0 {
			mps = append(mps, child.Mountpoints...)
		}
		if len(child.MountPath) > 0 && !containsStr(mps, child.MountPath) {
			mps = append(mps, child.MountPath)
		}
		mps = append(mps, collectAllMountpoints(child.Children)...)
	}
	return mps
}

// removeFstabEntriesForDisk 从 /etc/fstab 中移除与指定磁盘及其分区相关的条目。
func removeFstabEntriesForDisk(pool *HostStoragePoolInfo, devicePath string) error {
	data, err := os.ReadFile("/etc/fstab")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取 /etc/fstab 失败: %w", err)
	}

	// 收集需要移除的设备路径
	devicePaths := []string{devicePath}
	for _, child := range pool.Children {
		if child.DevicePath != "" {
			devicePaths = append(devicePaths, child.DevicePath)
		}
	}

	// 收集所有挂载点（子分区 + 磁盘自身）
	allMountpoints := collectAllMountpoints(pool.Children)
	if len(pool.Mountpoints) > 0 {
		allMountpoints = append(allMountpoints, pool.Mountpoints...)
	}

	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			kept = append(kept, line)
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			kept = append(kept, line)
			continue
		}
		shouldRemove := false
		// 检查设备路径匹配
		for _, dp := range devicePaths {
			if fields[0] == dp || (strings.HasPrefix(fields[0], "UUID=") && isDeviceUUID(dp)) {
				shouldRemove = true
				break
			}
		}
		// 检查挂载点匹配（处理 UUID 引用等间接情况）
		if !shouldRemove {
			for _, mp := range allMountpoints {
				if fields[1] == mp {
					shouldRemove = true
					break
				}
			}
		}
		if !shouldRemove {
			kept = append(kept, line)
		}
	}

	content := strings.Join(kept, "\n") + "\n"
	if err := os.WriteFile("/etc/fstab", []byte(content), 0644); err != nil {
		return fmt.Errorf("写入 /etc/fstab 失败: %w", err)
	}
	return nil
}

// isDeviceUUID 检查设备路径是否为 UUID 引用。
func isDeviceUUID(path string) bool {
	return strings.HasPrefix(path, "/dev/disk/by-uuid/")
}

// clearStoragePoolConfigs 清除数据库中与磁盘及其分区相关的存储池配置。
func clearStoragePoolConfigs(pool *HostStoragePoolInfo) error {
	if model.DB == nil {
		return nil
	}

	// 收集所有需要清除的 device_id
	ids := []string{pool.ID}
	for _, child := range pool.Children {
		if child.ID != "" {
			ids = append(ids, child.ID)
		}
	}

	for _, id := range ids {
		if err := model.DB.Where("device_id = ?", id).Delete(&model.HostStoragePool{}).Error; err != nil {
			return fmt.Errorf("删除存储池配置 %s 失败: %w", id, err)
		}
	}
	return nil
}

func containsStr(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
