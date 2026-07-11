package user

import (
	"fmt"
	"strings"

	"qvmhub/utils"
)

// MountStorageToVM 挂载用户存储池目录到虚拟机
func MountStorageToVM(username, vmName, category string, readonly bool) error {
	var hostPath string
	var tag string

	switch category {
	case StorageCategoryISO:
		hostPath = GetUserISODir(username)
		tag = fmt.Sprintf("user_%s_iso", username)
	case StorageCategoryShare:
		hostPath = GetUserShareDir(username)
		tag = fmt.Sprintf("user_%s_share", username)
	default:
		return fmt.Errorf("未知的存储类别: %s", category)
	}

	// 检查目录是否存在
	checkResult := utils.ExecShell(fmt.Sprintf("test -d %s && echo yes || echo no", utils.ShellSingleQuote(hostPath)))
	if strings.TrimSpace(checkResult.Stdout) != "yes" {
		return fmt.Errorf("存储池目录不存在，请先初始化存储池")
	}

	// 调用已有的共享目录服务
	securityModel := "mapped"
	return HookAddShare(vmName, hostPath, tag, securityModel, readonly)
}

// UnmountStorageFromVM 卸载用户存储池目录
func UnmountStorageFromVM(vmName, tag string) error {
	return HookRemoveShare(vmName, tag)
}

// FormatBytesPublic 格式化字节数为人类可读字符串（公开方法）
func FormatBytesPublic(bytes int64) string {
	return formatBytes(bytes)
}
