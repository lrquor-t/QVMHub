package lightweight

import (
	"time"

	"qvmhub/model"
)

// TemplateMeta mirrors clonepkg.TemplateMeta for use within the lightweight package.
type TemplateMeta struct {
	Type          string
	BootType      string
	RootPassword  string // 已废弃
	TemplateUser  string
	CloudInitMode string // 初始化模式: "nocloud"/"configdrive"/"fnos"/"none"
	NVRAMPath     string
	DefaultConfig *TemplateDefaultConfig
}

// TemplateDefaultConfig mirrors clonepkg.TemplateDefaultConfig.
type TemplateDefaultConfig struct {
	DiskBus             string `json:"disk_bus,omitempty"`
	VideoModel          string `json:"video_model,omitempty"`
	CPUTopologyMode     string `json:"cpu_topology_mode,omitempty"`
	FirstBootRebootMode string `json:"first_boot_reboot_mode,omitempty"`
}

// ── Constants (与 service 根包一致，避免循环 import) ──

const (
	BridgeModeNAT    = "nat"
	UserStatusActive = "active"
)

// RuntimeQuotaWarningThreshold mirrors service.RuntimeQuotaWarningThreshold.
const RuntimeQuotaWarningThreshold = 2 * time.Hour

// ── VM / User hooks ──

var (
	HookGetUserVMList                       func(username string) []string
	HookRefreshVMCacheByNameAsync           func(name string)
	HookCleanupVMVPCBinding                 func(vmName string)
	HookEnsureDefaultSecurityGroup          func(username string) (*model.VPCSecurityGroup, error)
	HookEnsureDefaultVPCSwitch              func(username string) (*model.VPCSwitch, error)
	HookSwitchUsesDirectBridge              func(sw model.VPCSwitch) bool
	HookBindVMToVPCAsAdmin                  func(vmName string, switchID, securityGroupID uint) error
	HookApplyVPCACLRules                    func() error
	HookClearVMBandwidth                    func(vmName string) error
	HookApplyVMNICBandwidth                 func(vmName string, downAvg, downPeak, downBurst, upAvg, upPeak, upBurst int) error
	HookCurrentTrafficMonth                 func() string
	HookTrafficQuotaBytes                   func(gb float64) float64
	HookClampTrafficBytes                   func(value int64) int64
	HookNormalizeVMCPUTopologyMode          func(mode string) string
	HookNormalizeVMCPULimitPercent          func(percent int) int
	HookValidateVMCPULimitPercent           func(percent int) error
	HookNormalizeVMFirstBootRebootMode      func(mode string) string
	HookGenerateRandomCloneHostname         func() string
	HookValidateCloneCredentialsForTemplate func(templateType, hostname, username, password string, requireCredentials bool) error
	HookNormalizeCloneUsernameForTemplate   func(templateType, username string) string
	HookGetTemplateMeta                     func(templateName string) *TemplateMeta
	HookResolveVMAPICEnabled                func(enabled *bool) bool
	HookRemoveVMFromUser                    func(username, vmName string) error
	HookAddVMToUser                         func(username, vmName string) error
	HookSaveVMCredential                    func(vmName, username, password, source, operator string, markReset bool) error
	HookStartVM                             func(name string) error
	HookFixOnReboot                         func(name string)
	HookIsSMTPConfigured                    func() bool
	HookSendEmail                           func(to, subject, body string) error
	HookFormatRuntimeQuotaDuration          func(seconds int64) string
	HookShutdownVM                          func(name string) error
	HookDestroyVM                           func(name string) error
	HookFirstNonEmpty                       func(values ...string) string

	// Unexported function hooks
	HookGetVMDiskPath                 func(vmName string) string
	HookWaitVMShutdownForDisable      func(vmName string, timeout time.Duration) bool
	HookGetRuntimeActiveVMSetFromHost func() (map[string]struct{}, error)
	HookFormatTrafficBytes            func(bytes int64) string
)
