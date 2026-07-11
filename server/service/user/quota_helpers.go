package user

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"qvmhub/service/vm/memory"
	"qvmhub/utils"
)

// GetVMCPUAndMemory 获取VM的CPU核心数和内存（MB）
func GetVMCPUAndMemory(vmName string) (cpu int, memMB int) {
	infoResult := utils.ExecCommand("virsh", "dominfo", vmName)
	if infoResult.Error != nil {
		return 0, 0
	}

	cpu = parseInfoInt(infoResult.Stdout, "CPU(s):")
	maxMem := parseInfoInt(infoResult.Stdout, "Max memory:")
	memMB = maxMem / 1024 // KiB -> MB
	if meta, err := memory.ReadVMMemoryMetadata(vmName); err == nil && meta != nil && meta.DynamicEnabled && meta.MemoryInitialMB > 0 {
		memMB = meta.MemoryInitialMB
	}

	return cpu, memMB
}

// getVMDiskCapacityGB 获取VM的所有磁盘配置容量之和（GB）
func getVMDiskCapacityGB(vmName string) int {
	// 获取磁盘列表
	blkResult := utils.ExecCommand("virsh", "domblklist", vmName)
	if blkResult.Error != nil {
		return 0
	}

	totalGB := 0
	lines := strings.Split(blkResult.Stdout, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] != "-" && fields[1] != "Source" && !strings.HasPrefix(line, "-") {
			diskPath := fields[1]
			if strings.HasSuffix(diskPath, ".iso") {
				continue
			}

			// 累加每个非ISO磁盘的配置容量
			totalGB += getDiskFileCapacityGB(diskPath)
		}
	}

	return totalGB
}

// GetVMDiskDevCapacityGB 获取指定VM的指定磁盘设备的虚拟容量（GB）
// dev 为磁盘设备名，如 vda、vdb、sda 等
func GetVMDiskDevCapacityGB(vmName, dev string) int {
	blkResult := utils.ExecCommand("virsh", "domblklist", vmName)
	if blkResult.Error != nil {
		return 0
	}

	lines := strings.Split(blkResult.Stdout, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == dev {
			diskPath := fields[1]
			if diskPath == "-" || strings.HasSuffix(diskPath, ".iso") {
				return 0
			}
			return getDiskFileCapacityGB(diskPath)
		}
	}
	return 0
}

// getDiskFileCapacityGB 获取磁盘文件的虚拟容量（GB）
func getDiskFileCapacityGB(diskPath string) int {
	imgResult := utils.ExecShell(fmt.Sprintf(
		"qemu-img info -U %s 2>/dev/null | grep 'virtual size'", utils.ShellSingleQuote(diskPath)))
	if imgResult.Error == nil && imgResult.Stdout != "" {
		re := regexp.MustCompile(`virtual size:\s*([\d.]+)\s*(GiB|GB|MiB|MB|TiB|TB)`)
		if matches := re.FindStringSubmatch(imgResult.Stdout); len(matches) > 2 {
			sizeFloat, _ := strconv.ParseFloat(matches[1], 64)
			switch matches[2] {
			case "TiB", "TB":
				return int(sizeFloat * 1024)
			case "GiB", "GB":
				return int(sizeFloat)
			case "MiB", "MB":
				return int(sizeFloat / 1024)
			}
		}
	}
	return 0
}

// parseInfoInt 辅助函数：从 virsh dominfo 输出中解析整数值
func parseInfoInt(stdout, prefix string) int {
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				val := strings.TrimSpace(parts[1])
				// 移除单位，如 "2048 KiB"
				val = strings.Fields(val)[0]
				if n, err := strconv.Atoi(val); err == nil {
					return n
				}
			}
		}
	}
	return 0
}
