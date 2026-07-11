package pool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"qvmhub/config"
	"qvmhub/utils"
)

// GetAllISOs 扫描系统设置中配置的全局 ISO 目录。
func GetAllISOs() ([]ISOFileInfo, error) {
	paths := collectISOScanPaths()
	seen := make(map[string]bool)
	var all []ISOFileInfo
	for label, dir := range paths {
		result := utils.ExecShell(fmt.Sprintf("find %s -maxdepth 1 -name '*.iso' -type f 2>/dev/null", utils.ShellSingleQuote(dir)))
		if result.Error != nil || strings.TrimSpace(result.Stdout) == "" {
			continue
		}
		for _, line := range strings.Split(result.Stdout, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || seen[line] {
				continue
			}
			seen[line] = true
			all = append(all, BuildISOInfo(line, label))
		}
	}
	return all, nil
}

// InferOSFromISO 根据文件名推断 ISO 的操作系统类型和变体。
func InferOSFromISO(nameLower string) (osType, osVariant string) {
	return inferOSFromISO(nameLower)
}

// BuildISOInfo 构建 ISO 文件信息（导出供 root package delegate 使用）。
func BuildISOInfo(filePath, poolName string) ISOFileInfo {
	return buildISOInfo(filePath, poolName)
}

func collectISOScanPaths() map[string]string {
	return map[string]string{
		"系统设置 ISO": configuredISODir(),
	}
}

func configuredISODir() string {
	if config.GlobalConfig == nil {
		return config.DefaultISODir
	}
	isoDir := strings.TrimSpace(config.GlobalConfig.ISODir)
	if isoDir == "" {
		return config.DefaultISODir
	}
	return isoDir
}

func buildISOInfo(filePath, poolName string) ISOFileInfo {
	name := filepath.Base(filePath)
	nameLower := strings.ToLower(name)
	iso := ISOFileInfo{Name: name, Path: filePath, Pool: poolName}
	sizeResult := utils.ExecShell(fmt.Sprintf("du -h %s | awk '{print $1}'", utils.ShellSingleQuote(filePath)))
	if sizeResult.Error == nil {
		iso.Size = strings.TrimSpace(sizeResult.Stdout)
	}
	if fi, err := os.Stat(filePath); err == nil {
		iso.SizeBytes = fi.Size()
	}
	iso.OSType, iso.OSVariant = inferOSFromISO(nameLower)
	if iso.OSType == "windows" {
		iso.MinDisk = 20
	} else {
		iso.MinDisk = 10
	}
	return iso
}

func inferOSFromISO(nameLower string) (osType, osVariant string) {
	if strings.Contains(nameLower, "win11") || strings.Contains(nameLower, "windows11") || strings.Contains(nameLower, "windows_11") || strings.Contains(nameLower, "win_11") {
		return "windows", "win11"
	}
	if strings.Contains(nameLower, "win10") || strings.Contains(nameLower, "windows10") || strings.Contains(nameLower, "windows_10") || strings.Contains(nameLower, "win_10") {
		return "windows", "win10"
	}
	if strings.Contains(nameLower, "win2k25") || strings.Contains(nameLower, "server2025") || strings.Contains(nameLower, "2025") && strings.Contains(nameLower, "server") {
		return "windows", "win2k25"
	}
	if strings.Contains(nameLower, "win2k22") || strings.Contains(nameLower, "server2022") || strings.Contains(nameLower, "2022") && strings.Contains(nameLower, "server") {
		return "windows", "win2k22"
	}
	if strings.Contains(nameLower, "win2k19") || strings.Contains(nameLower, "server2019") || strings.Contains(nameLower, "2019") && strings.Contains(nameLower, "server") {
		return "windows", "win2k19"
	}
	if strings.Contains(nameLower, "win2k16") || strings.Contains(nameLower, "server2016") {
		return "windows", "win2k16"
	}
	if strings.Contains(nameLower, "win") || strings.Contains(nameLower, "windows") {
		return "windows", ""
	}
	if strings.Contains(nameLower, "ubuntu") {
		if strings.Contains(nameLower, "24.04") || strings.Contains(nameLower, "noble") {
			return "linux", "ubuntu24.04"
		}
		if strings.Contains(nameLower, "22.04") || strings.Contains(nameLower, "jammy") {
			return "linux", "ubuntu22.04"
		}
		if strings.Contains(nameLower, "20.04") || strings.Contains(nameLower, "focal") {
			return "linux", "ubuntu20.04"
		}
		return "linux", "ubuntu24.04"
	}
	if strings.Contains(nameLower, "debian") {
		if strings.Contains(nameLower, "12") || strings.Contains(nameLower, "bookworm") {
			return "linux", "debian12"
		}
		if strings.Contains(nameLower, "11") || strings.Contains(nameLower, "bullseye") {
			return "linux", "debian11"
		}
		return "linux", "debian12"
	}
	if strings.Contains(nameLower, "centos") {
		return "linux", "centos-stream9"
	}
	if strings.Contains(nameLower, "rocky") {
		return "linux", "rocky9"
	}
	if strings.Contains(nameLower, "alma") {
		return "linux", "almalinux9"
	}
	if strings.Contains(nameLower, "rhel") || strings.Contains(nameLower, "redhat") {
		return "linux", "rhel9-unknown"
	}
	if strings.Contains(nameLower, "fedora") {
		return "linux", "fedora-unknown"
	}
	if strings.Contains(nameLower, "arch") {
		return "linux", "archlinux"
	}
	if strings.Contains(nameLower, "alpine") {
		return "linux", "alpinelinux3.21"
	}
	if strings.Contains(nameLower, "opensuse") || strings.Contains(nameLower, "suse") {
		return "linux", "opensuse-unknown"
	}
	return "linux", "generic"
}
