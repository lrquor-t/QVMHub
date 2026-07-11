package arch

import (
	"strings"
	"sync"

	"qvmhub/utils"
)

// ==================== 宿主机架构检测 ====================

var (
	cachedHostArch string
	cacheOnce      sync.Once
)

// detectAndCache 执行 uname -m 并缓存
func detectAndCache() {
	result := utils.ExecCommand("uname", "-m")
	if result.Error != nil {
		// 降级：默认假定为 x86_64
		cachedHostArch = ArchX8664
		return
	}
	raw := strings.TrimSpace(result.Stdout)
	switch raw {
	case "x86_64":
		cachedHostArch = ArchX8664
	case "aarch64", "arm64":
		cachedHostArch = ArchAarch64
	case "riscv64":
		cachedHostArch = ArchRiscv64
	default:
		cachedHostArch = ArchX8664
	}
}

// DetectHostArch 检测宿主机架构，结果缓存
func DetectHostArch() string {
	cacheOnce.Do(detectAndCache)
	return cachedHostArch
}

// GetHostArchDisplayName 返回人类可读的架构名称
func GetHostArchDisplayName() string {
	arch := DetectHostArch()
	if p := GetProfile(arch); p != nil {
		return p.DisplayName()
	}
	return arch
}

// IsHostArch 判断宿主机是否为指定架构
func IsHostArch(arch string) bool {
	return DetectHostArch() == arch
}
