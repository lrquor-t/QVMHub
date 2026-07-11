package pool

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"qvmhub/logger"
	"qvmhub/utils"
)

// CreatePartitionOnDisk 在指定磁盘上创建新分区。
// sizeGB 为 0 时使用所有剩余空间。
func CreatePartitionOnDisk(ctx context.Context, deviceID string, sizeGB int, progress func(int, string)) error {
	pool, err := GetStoragePool(deviceID)
	if err != nil {
		return err
	}
	if err := validatePartitionTarget(*pool); err != nil {
		return err
	}

	devicePath := pool.DevicePath

	progress(10, "正在检测分区表...")
	hasTable, err := diskHasPartitionTable(devicePath)
	if err != nil {
		return fmt.Errorf("检测分区表失败: %w", err)
	}

	if !hasTable {
		progress(20, "正在创建 GPT 分区表...")
		result := utils.ExecCommandContextWithTimeout(ctx, "parted", 30*time.Second, "-s", devicePath, "mklabel", "gpt")
		if result.Error != nil {
			return fmt.Errorf("创建 GPT 分区表失败: %s", result.Stderr)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	progress(30, "正在计算分区位置...")
	startBytes, err := findPartitionStart(ctx, devicePath)
	if err != nil {
		return fmt.Errorf("计算分区起始位置失败: %w", err)
	}

	// 将起始位置（字节）转换为 parted 可接受的格式
	startMB := (startBytes + 1024*1024 - 1) / (1024 * 1024) // 向上取整到 MB
	if startMB < 1 {
		startMB = 1
	}
	startSpec := fmt.Sprintf("%dMiB", startMB)

	var endSpec string
	if sizeGB > 0 {
		endSpec = fmt.Sprintf("%dMiB", startMB+int64(sizeGB)*1024)
	} else {
		endSpec = "100%"
	}

	progress(50, fmt.Sprintf("正在创建分区 (%s → %s)...", startSpec, endSpec))
	createResult := utils.ExecCommandContextWithTimeout(ctx, "parted", 2*time.Minute,
		"-s", devicePath, "mkpart", "primary", "ext4", startSpec, endSpec)
	if createResult.Error != nil {
		return fmt.Errorf("创建分区失败: %s", createResult.Stderr)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	progress(80, "正在刷新分区表...")
	probeResult := utils.ExecCommandContextWithTimeout(ctx, "partprobe", 30*time.Second, devicePath)
	if probeResult.Error != nil {
		// partprobe 失败不致命，内核可能会自动检测
		logger.App.Warn("刷新分区表警告", "stderr", probeResult.Stderr)
	}

	progress(100, "分区创建完成")
	return nil
}

// validatePartitionTarget 验证磁盘是否可用于创建分区。
func validatePartitionTarget(pool HostStoragePoolInfo) error {
	if pool.Type != "disk" {
		return fmt.Errorf("只能对整块磁盘创建分区，当前设备类型为 %s", pool.Type)
	}
	if pool.Readonly {
		return fmt.Errorf("设备为只读状态，无法创建分区")
	}
	if pool.SystemDisk {
		return fmt.Errorf("系统关键磁盘禁止创建分区")
	}
	if len(pool.Mountpoints) > 0 || hasMountedChild(pool) {
		return fmt.Errorf("设备或其分区当前已挂载，无法创建分区")
	}
	// 排除 LVM/raid/zfs 成员
	nameLower := strings.ToLower(pool.Name)
	if strings.HasPrefix(nameLower, "ram") || strings.HasPrefix(nameLower, "zram") || pool.Type == "dm" {
		return fmt.Errorf("不支持在此类设备上创建分区")
	}
	fstype := strings.ToLower(pool.FSType)
	if fstype == "lvm2_member" || fstype == "linux_raid_member" || fstype == "zfs_member" {
		return fmt.Errorf("该设备已加入 LVM/RAID/ZFS，无法创建分区")
	}
	if hasChildWithFSType(pool, "lvm2_member", "linux_raid_member", "zfs_member") {
		return fmt.Errorf("该设备的子分区已加入 LVM/RAID/ZFS，无法创建分区")
	}
	return nil
}

// diskHasPartitionTable 检查磁盘是否已有分区表。
func diskHasPartitionTable(devicePath string) (bool, error) {
	result := utils.ExecCommand("parted", "-s", devicePath, "print")
	if result.Error != nil {
		stderr := strings.ToLower(result.Stderr)
		// "unrecognised disk label" 表示没有分区表
		if strings.Contains(stderr, "unrecognised disk label") {
			return false, nil
		}
		// 其他错误需要返回
		return false, fmt.Errorf("读取分区表失败: %s", result.Stderr)
	}
	// 成功执行说明有分区表
	return true, nil
}

// findPartitionStart 找到新分区应该开始的位置（最后一个分区的末尾 + 1MiB 对齐）。
func findPartitionStart(ctx context.Context, devicePath string) (int64, error) {
	result := utils.ExecCommandContextWithTimeout(ctx, "parted", 30*time.Second,
		"-s", devicePath, "unit", "B", "print")
	if result.Error != nil {
		// 如果是 GPT 空表（没有分区），从 1MiB 开始
		return 1024 * 1024, nil
	}

	lines := strings.Split(result.Stdout, "\n")
	var maxEnd int64

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Model:") ||
			strings.HasPrefix(line, "Disk") || strings.HasPrefix(line, "Sector") ||
			strings.HasPrefix(line, "Partition Table:") || strings.HasPrefix(line, "Number") {
			continue
		}
		// parted unit B print 输出格式：Number  Start        End          Size     File system  Name  Flags
		// 例如： 1      1048576B    10737418239B  10736369664B  ext4
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		// 第一列应该是数字（分区号）
		if _, err := strconv.Atoi(fields[0]); err != nil {
			continue
		}
		// 第二列是 Start，第三列是 End
		endStr := strings.TrimSuffix(fields[2], "B")
		endVal, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil {
			continue
		}
		if endVal > maxEnd {
			maxEnd = endVal
		}
	}

	// 对齐到 1MiB 边界
	align := int64(1024 * 1024)
	start := ((maxEnd + align - 1) / align) * align
	if start < align {
		start = align
	}
	return start, nil
}
