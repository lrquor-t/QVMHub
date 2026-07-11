package service

// Clone function delegates - forward to service/clone subpackage
// Maintains backward compatibility for callers using service.XXX()
import (
	"context"
	"fmt"

	"qvmhub/config"
	clonepkg "qvmhub/service/clone"
	securitypkg "qvmhub/service/security"
)

// DeleteVM delegates to clone.DeleteVM
func DeleteVM(name string) error {
	return clonepkg.DeleteVM(name)
}

// DeleteVMWithDisks delegates to clone.DeleteVMWithDisks
func DeleteVMWithDisks(name string, deleteDisks []string, transferDisks []string, transferUser string) error {
	return clonepkg.DeleteVMWithDisks(name, deleteDisks, transferDisks, transferUser)
}

// ForceDeleteVM delegates to clone.ForceDeleteVM
func ForceDeleteVM(name string) error {
	return clonepkg.ForceDeleteVM(name)
}

// CloneVM delegates to clone.CloneVM
func CloneVM(ctx context.Context, params *CloneParams, progressFn func(int, string)) (*CloneResult, error) {
	return clonepkg.CloneVM(ctx, params, progressFn)
}

// BatchCloneVM delegates to clone.BatchCloneVM
func BatchCloneVM(ctx context.Context, params *BatchCloneParams, progressFn func(int, string)) ([]CloneResult, error) {
	return clonepkg.BatchCloneVM(ctx, params, progressFn)
}

// BatchVMName delegates to clone.BatchVMName（批量克隆命名单一来源，handler 预检与创建共用，杜绝格式漂移）
func BatchVMName(prefix string, n int) string {
	return clonepkg.BatchVMName(prefix, n)
}

// ReinstallVM delegates to clone.ReinstallVM
func ReinstallVM(ctx context.Context, params *ReinstallParams, progressFn func(int, string)) error {
	return clonepkg.ReinstallVM(ctx, params, progressFn)
}

// ParseCloneParams delegates to clone.ParseCloneParams
func ParseCloneParams(jsonStr string) (*CloneParams, error) {
	return clonepkg.ParseCloneParams(jsonStr)
}

// ParseBatchCloneParams delegates to clone.ParseBatchCloneParams
func ParseBatchCloneParams(jsonStr string) (*BatchCloneParams, error) {
	return clonepkg.ParseBatchCloneParams(jsonStr)
}

// ParseReinstallParams delegates to clone.ParseReinstallParams
func ParseReinstallParams(jsonStr string) (*ReinstallParams, error) {
	return clonepkg.ParseReinstallParams(jsonStr)
}

// CheckDiskTransferQuota delegates to clone.CheckDiskTransferQuota
func CheckDiskTransferQuota(username string, diskPaths []string) (int64, error) {
	return clonepkg.CheckDiskTransferQuota(username, diskPaths)
}

// ValidateCloneCredentials delegates to clone.ValidateCloneCredentials
func ValidateCloneCredentials(hostname, username, password string, requireCredentials bool) error {
	return clonepkg.ValidateCloneCredentials(hostname, username, password, requireCredentials)
}

// ValidateCloneCredentialsForTemplate delegates to clone.ValidateCloneCredentialsForTemplate
func ValidateCloneCredentialsForTemplate(templateType, hostname, username, password string, requireCredentials bool) error {
	return clonepkg.ValidateCloneCredentialsForTemplate(templateType, hostname, username, password, requireCredentials)
}

// ValidateStrongPassword 统一密码强度校验
// 当 PasswordBreachCheckEnabled 为 false 时跳过所有校验
// 当为 true 时检查泄露密码（HIBP API + 本地常见弱密码列表）
func ValidateStrongPassword(password string) error {
	if !config.GlobalConfig.PasswordBreachCheckEnabled {
		return nil
	}
	// 基础校验（clone 包内的规则校验）
	if err := clonepkg.ValidateStrongPassword(password); err != nil {
		return err
	}
	// 泄露密码检测
	breached, fallback, err := securitypkg.CheckPasswordBreached(password)
	if err != nil {
		// API 不可用时仅依赖本地兜底结果
		if !fallback {
			return nil
		}
	}
	if breached {
		return fmt.Errorf("该密码已在已知泄露数据库中发现，请更换为更安全的密码")
	}
	return nil
}

// NormalizeCloneUsernameForTemplate delegates to clone.NormalizeCloneUsernameForTemplate
func NormalizeCloneUsernameForTemplate(templateType, username string) string {
	return clonepkg.NormalizeCloneUsernameForTemplate(templateType, username)
}

// ValidateFnOSDeviceID delegates to clone.ValidateFnOSDeviceID
func ValidateFnOSDeviceID(deviceID string) error {
	return clonepkg.ValidateFnOSDeviceID(deviceID)
}

// IsReinstallBootFamilyCompatible delegates to clone.IsReinstallBootFamilyCompatible
func IsReinstallBootFamilyCompatible(currentBootType, templateBootType string) bool {
	return clonepkg.IsReinstallBootFamilyCompatible(currentBootType, templateBootType)
}

// ResolveReinstallDiskSizeGB delegates to clone.ResolveReinstallDiskSizeGB
func ResolveReinstallDiskSizeGB(vmName, templateName string, requestedDiskSize int) (int, error) {
	return clonepkg.ResolveReinstallDiskSizeGB(vmName, templateName, requestedDiskSize)
}

// NormalizeReinstallDiskSizeGB delegates to clone.NormalizeReinstallDiskSizeGB
func NormalizeReinstallDiskSizeGB(requestedDiskSize, currentDiskSize, minDiskSize int) int {
	return clonepkg.NormalizeReinstallDiskSizeGB(requestedDiskSize, currentDiskSize, minDiskSize)
}

// CheckCanceled delegates to clone.CheckCanceled
func CheckCanceled(ctx context.Context, vmName, diskPath string) error {
	return clonepkg.CheckCanceled(ctx, vmName, diskPath)
}

// WaitForIPWithContext delegates to clone.WaitForIPWithContext
func WaitForIPWithContext(ctx context.Context, vmName string, maxWaitSeconds int) string {
	return clonepkg.WaitForIPWithContext(ctx, vmName, maxWaitSeconds)
}

// PrepareLinuxCloneFirstBootIdentity 通过 virt-customize 离线完成 Linux 克隆初始化
// 包括 machine-id 清理、cloud-init NoCloud seed 写入、密码和用户名修改
func PrepareLinuxCloneFirstBootIdentity(params *CloneParams, cloneDisk string) error {
	return clonepkg.PrepareLinuxCloneFirstBootIdentityExported(params, cloneDisk)
}

// InjectMemballoonConfig delegates to clone.InjectMemballoonConfig
func InjectMemballoonConfig(xmlStr string, enableFPR bool) string {
	return clonepkg.InjectMemballoonConfig(xmlStr, enableFPR)
}

// GenerateRandomCloneHostname delegates to clone.GenerateRandomCloneHostname
func GenerateRandomCloneHostname() string {
	return clonepkg.GenerateRandomCloneHostname()
}

// LinkedCloneVM delegates to clone.LinkedCloneVM
func LinkedCloneVM(ctx context.Context, params *LinkedCloneParams, progressFn func(int, string)) (*LinkedCloneResult, error) {
	return clonepkg.LinkedCloneVM(ctx, params, progressFn)
}

// ParseLinkedCloneParams delegates to clone.ParseLinkedCloneParams
func ParseLinkedCloneParams(jsonStr string) (*LinkedCloneParams, error) {
	return clonepkg.ParseLinkedCloneParams(jsonStr)
}

// NormalizeFnOSDeviceID delegates to clone.NormalizeFnOSDeviceID
func NormalizeFnOSDeviceID(deviceID string) (string, string, error) {
	return clonepkg.NormalizeFnOSDeviceID(deviceID)
}

// RandomStringFromCharset delegates to clone.RandomStringFromCharset
func RandomStringFromCharset(charset string, length int) string {
	return clonepkg.RandomStringFromCharset(charset, length)
}

// CloneUsernameRegexp re-exports clone.CloneUsernameRegexp
var CloneUsernameRegexp = clonepkg.CloneUsernameRegexp

// LinuxCloneIPWaitSeconds re-exports clone.LinuxCloneIPWaitSeconds
const LinuxCloneIPWaitSeconds = clonepkg.LinuxCloneIPWaitSeconds
