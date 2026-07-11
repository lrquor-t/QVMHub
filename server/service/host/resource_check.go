package host

import (
	"encoding/json"
	"fmt"
	"os"

	"qvmhub/logger"
	"qvmhub/utils"
)

// CheckStorageSpace 检查指定目录的可用空间是否满足要求
// dir: 目标目录路径
// requiredMB: 所需空间大小（单位：MB）
func CheckStorageSpace(dir string, requiredMB int64) error {
	// 使用 syscall 获取可用空间（KB）
	_, _, availableKB, err := utils.GetDiskSpace(dir)
	if err != nil {
		return fmt.Errorf("无法获取目录 %s 的磁盘空间信息: %w", dir, err)
	}
	requiredKB := requiredMB * 1024
	if availableKB < requiredKB {
		return fmt.Errorf("目录 %s 可用空间不足，需要 %d MB，当前可用 %d MB", dir, requiredMB, availableKB/1024)
	}
	return nil
}

// CheckDirWritable 检查目录是否存在且可写
// dir: 目标目录路径
func CheckDirWritable(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("目录 %s 不存在", dir)
		}
		return fmt.Errorf("无法访问目录 %s: %w", dir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("路径 %s 不是目录", dir)
	}

	// 通过创建临时文件测试可写性
	tmpFile, err := os.CreateTemp(dir, "write-test-*")
	if err != nil {
		return fmt.Errorf("目录 %s 不可写: %w", dir, err)
	}

	tmpPath := tmpFile.Name()
	tmpFile.Close()
	_ = os.Remove(tmpPath)

	return nil
}

// ValidateDiskBackingChain 校验磁盘镜像的 backing chain 完整性
// diskPath: 磁盘镜像路径
func ValidateDiskBackingChain(diskPath string) error {
	result := utils.ExecCommand("qemu-img", "info", "--backing-chain", "--output=json", diskPath)
	if result.Error != nil {
		return fmt.Errorf("获取磁盘 %s 的 backing chain 失败: %s", diskPath, result.Stderr)
	}

	// qemu-img info --backing-chain --output=json 返回一个 JSON 数组
	var chain []map[string]interface{}
	if err := json.Unmarshal([]byte(result.Stdout), &chain); err != nil {
		// 某些旧版本可能返回单个对象而非数组
		var single map[string]interface{}
		if err2 := json.Unmarshal([]byte(result.Stdout), &single); err2 != nil {
			return fmt.Errorf("解析磁盘 %s 的 backing chain JSON 失败: %w", diskPath, err)
		}
		chain = []map[string]interface{}{single}
	}

	for _, item := range chain {
		filename, ok := item["filename"].(string)
		if !ok || filename == "" {
			continue
		}

		if _, err := os.Stat(filename); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("磁盘链文件缺失: %s", filename)
			}
			return fmt.Errorf("无法访问磁盘链文件 %s: %w", filename, err)
		}
	}

	return nil
}

// EnsureOVSBridgeExists 检查指定的 OVS 网桥是否存在
// bridge: 网桥名称
func EnsureOVSBridgeExists(bridge string) error {
	if bridge == "" {
		return fmt.Errorf("网桥名称不能为空")
	}

	result := utils.ExecCommand("ovs-vsctl", "br-exists", bridge)
	if result.Error != nil {
		return fmt.Errorf("OVS 网桥 %s 不存在", bridge)
	}

	return nil
}

// VMCreateCheckpoint 用于记录虚拟机创建过程中的关键状态，支持出错时回滚
type VMCreateCheckpoint struct {
	VMName    string   // 虚拟机名称
	DiskPaths []string // 已创建的磁盘文件路径
	VMDefined bool     // 是否已执行 virsh define
	VMStarted bool     // 是否已执行 virsh start
	DBRecords []string // 记录已创建的数据库记录类型
}

// Rollback 执行尽力而为的清理回滚
// 每步失败仅记录日志，不中断后续清理流程
func (c *VMCreateCheckpoint) Rollback() {
	if c.VMName == "" {
		return
	}

	// 1. 如果虚拟机已启动，则强制销毁
	if c.VMStarted {
		if result := utils.ExecCommand("virsh", "destroy", c.VMName); result.Error != nil {
			logger.App.Error("销毁虚拟机失败", "vm", c.VMName, "stderr", result.Stderr)
		} else {
			logger.App.Info("已销毁虚拟机", "vm", c.VMName)
		}
	}

	// 2. 如果虚拟机已定义，则取消定义（包含 NVRAM 和快照元数据）
	if c.VMDefined {
		if result := utils.ExecCommand("virsh", "undefine", "--nvram", "--snapshots-metadata", c.VMName); result.Error != nil {
			logger.App.Error("取消定义虚拟机失败", "vm", c.VMName, "stderr", result.Stderr)
		} else {
			logger.App.Info("已取消定义虚拟机", "vm", c.VMName)
		}
	}

	// 3. 删除所有已创建的磁盘文件
	for _, diskPath := range c.DiskPaths {
		if diskPath == "" {
			continue
		}
		if err := os.Remove(diskPath); err != nil {
			logger.App.Warn("删除磁盘文件失败", "path", diskPath, "error", err)
		} else {
			logger.App.Info("已删除磁盘文件", "path", diskPath)
		}
	}
}
