package service

import (
	"time"

	"qvmhub/model"
	userpkg "qvmhub/service/user"
)

// ── Type aliases ──

type VMUserInfo = userpkg.VMUserInfo
type UserStatusChangeResult = userpkg.UserStatusChangeResult
type QuotaUsage = userpkg.QuotaUsage
type UserStorageInfo = userpkg.UserStorageInfo
type UserFileInfo = userpkg.UserFileInfo
type UserRuntimeQuotaSnapshot = userpkg.UserRuntimeQuotaSnapshot
type RuntimeQuotaShutdownResult = userpkg.RuntimeQuotaShutdownResult

// ── Constant aliases ──

const (
	StorageCategoryISO   = userpkg.StorageCategoryISO
	StorageCategoryShare = userpkg.StorageCategoryShare
	StorageCategoryDisk  = userpkg.StorageCategoryDisk
)

// init wires user package function variables to service root implementations.
// This breaks the circular dependency: user package cannot import service,
// so it exposes function variables that we set here.
func init() {
	// ── Cloud / Lightweight VM hooks ──
	userpkg.HookNormalizeCloudType = NormalizeCloudType
	userpkg.HookIsLightweightCloudType = IsLightweightCloudType
	userpkg.HookIsLightweightCloudUser = IsLightweightCloudUser
	userpkg.HookIsLightweightCloudVM = IsLightweightCloudVM
	userpkg.HookNormalizeLightweightVMQuotaRequest = func(req userpkg.LightweightVMQuotaRequest) userpkg.LightweightVMQuotaRequest {
		serviceReq := LightweightVMQuotaRequest{
			VMName:            req.VMName,
			TrafficDownGB:     req.TrafficDownGB,
			TrafficUpGB:       req.TrafficUpGB,
			BandwidthDownMbps: req.BandwidthDownMbps,
			BandwidthUpMbps:   req.BandwidthUpMbps,
			MaxPortForwards:   req.MaxPortForwards,
			MaxSnapshots:      req.MaxSnapshots,
			MaxRuntimeHours:   req.MaxRuntimeHours,
		}
		result := NormalizeLightweightVMQuotaRequest(serviceReq)
		return userpkg.LightweightVMQuotaRequest{
			VMName:            result.VMName,
			TrafficDownGB:     result.TrafficDownGB,
			TrafficUpGB:       result.TrafficUpGB,
			BandwidthDownMbps: result.BandwidthDownMbps,
			BandwidthUpMbps:   result.BandwidthUpMbps,
			MaxPortForwards:   result.MaxPortForwards,
			MaxSnapshots:      result.MaxSnapshots,
			MaxRuntimeHours:   result.MaxRuntimeHours,
		}
	}
	userpkg.HookDefaultLightweightVMQuota = func(vmName string) userpkg.LightweightVMQuotaRequest {
		result := defaultLightweightVMQuota(vmName)
		return userpkg.LightweightVMQuotaRequest{
			VMName:            result.VMName,
			TrafficDownGB:     result.TrafficDownGB,
			TrafficUpGB:       result.TrafficUpGB,
			BandwidthDownMbps: result.BandwidthDownMbps,
			BandwidthUpMbps:   result.BandwidthUpMbps,
			MaxPortForwards:   result.MaxPortForwards,
			MaxSnapshots:      result.MaxSnapshots,
			MaxRuntimeHours:   result.MaxRuntimeHours,
		}
	}
	userpkg.HookFillLightweightVMQuotaRuntime = fillLightweightVMQuotaRuntime
	userpkg.HookListLightweightVMRegistrations = func(username string, includeActive bool) ([]userpkg.LightweightVMRegistrationView, error) {
		regs, err := ListLightweightVMRegistrations(username, includeActive)
		if err != nil {
			return nil, err
		}
		result := make([]userpkg.LightweightVMRegistrationView, len(regs))
		for i, r := range regs {
			result[i] = userpkg.LightweightVMRegistrationView{
				ID:                   r.ID,
				Username:             r.Username,
				VMName:               r.VMName,
				Template:             r.Template,
				TemplateType:         r.TemplateType,
				VCPU:                 r.VCPU,
				RAM:                  r.RAM,
				DiskSize:             r.DiskSize,
				DiskBus:              r.DiskBus,
				Hostname:             r.Hostname,
				Autostart:            r.Autostart,
				Freeze:               r.Freeze,
				APIC:                 r.APIC,
				PAE:                  r.PAE,
				RTCOffset:            r.RTCOffset,
				RTCStartDate:         r.RTCStartDate,
				VideoModel:           r.VideoModel,
				CPUTopologyMode:      r.CPUTopologyMode,
				CPULimitPercent:      r.CPULimitPercent,
				CPUAffinity:          r.CPUAffinity,
				FirstBootRebootMode:  r.FirstBootRebootMode,
				NicModel:             r.NicModel,
				StoragePoolID:        r.StoragePoolID,
				PreserveFnOSDeviceID: r.PreserveFnOSDeviceID,
				FnOSDeviceID:         r.FnOSDeviceID,
				SwitchID:             r.SwitchID,
				SwitchName:           r.SwitchName,
			}
		}
		return result, nil
	}
	userpkg.HookUpsertLightweightVMQuota = func(username string, req userpkg.LightweightVMQuotaRequest) (*model.LightweightVMQuota, error) {
		serviceReq := LightweightVMQuotaRequest{
			VMName:            req.VMName,
			TrafficDownGB:     req.TrafficDownGB,
			TrafficUpGB:       req.TrafficUpGB,
			BandwidthDownMbps: req.BandwidthDownMbps,
			BandwidthUpMbps:   req.BandwidthUpMbps,
			MaxPortForwards:   req.MaxPortForwards,
			MaxSnapshots:      req.MaxSnapshots,
			MaxRuntimeHours:   req.MaxRuntimeHours,
		}
		return UpsertLightweightVMQuota(username, serviceReq)
	}
	userpkg.HookEnsureLightweightVMNetwork = EnsureLightweightVMNetwork
	userpkg.HookCleanupLightweightVMResources = CleanupLightweightVMResources
	userpkg.HookCleanupVMVPCBinding = CleanupVMVPCBinding

	// ── Network / Security hooks ──
	userpkg.HookEnsureDefaultSecurityGroup = EnsureDefaultSecurityGroup
	userpkg.HookEnsureDefaultVPCSwitch = EnsureDefaultVPCSwitch
	userpkg.HookCleanupUserNetworkResources = CleanupUserNetworkResources

	// ── VM cache hooks ──
	userpkg.HookSyncVMCacheOwnersForAssignment = SyncVMCacheOwnersForAssignment
	userpkg.HookUpdateVMCacheOwner = UpdateVMCacheOwner
	userpkg.HookSyncVMCacheOwner = SyncVMCacheOwner

	// ── VM lifecycle hooks ──
	userpkg.HookShutdownVM = ShutdownVM
	userpkg.HookDestroyVM = DestroyVM

	// ── Storage hooks ──
	userpkg.HookGetStorageMountPoint = GetStorageMountPoint
	userpkg.HookEnsureStorageFilesystem = EnsureStorageFilesystem
	userpkg.HookSetupUserProject = SetupUserProject
	userpkg.HookGetProjectID = GetProjectID
	userpkg.HookSetUserStorageQuota = SetUserStorageQuota
	userpkg.HookRemoveUserStorageQuota = RemoveUserStorageQuota
	userpkg.HookGetUserStorageUsage = func(username string) (*userpkg.StorageQuotaInfo, error) {
		info, err := GetUserStorageUsage(username)
		if err != nil {
			return nil, err
		}
		if info == nil {
			return nil, nil
		}
		return &userpkg.StorageQuotaInfo{
			UsedBytes:  info.UsedBytes,
			LimitBytes: info.LimitBytes,
		}, nil
	}
	userpkg.HookInferOSFromISO = inferOSFromISO
	userpkg.HookBuildISOInfo = func(filePath, poolName string) userpkg.ISOFileInfo {
		info := buildISOInfo(filePath, poolName)
		return userpkg.ISOFileInfo{
			Name:      info.Name,
			Path:      info.Path,
			Size:      info.Size,
			SizeBytes: info.SizeBytes,
			Pool:      info.Pool,
			OSType:    info.OSType,
			OSVariant: info.OSVariant,
			MinDisk:   info.MinDisk,
		}
	}
	userpkg.HookAddShare = AddShare
	userpkg.HookRemoveShare = RemoveShare

	// ── Network / Traffic / Bandwidth hooks ──
	userpkg.HookGetUserPublicIPUsage = GetUserPublicIPUsage
	userpkg.HookGetUserPortForwardUsage = GetUserPortForwardUsage
	userpkg.HookGetUserTrafficUsage = func(username string) *userpkg.TrafficUsageInfo {
		info := GetUserTrafficUsage(username)
		if info == nil {
			return nil
		}
		return &userpkg.TrafficUsageInfo{
			MaxTrafficDown:    info.MaxTrafficDown,
			MaxTrafficUp:      info.MaxTrafficUp,
			UsedTrafficDown:   info.UsedTrafficDown,
			UsedTrafficUp:     info.UsedTrafficUp,
			UsedTrafficDownGB: info.UsedTrafficDownGB,
			UsedTrafficUpGB:   info.UsedTrafficUpGB,
			IsLimitedDown:     info.IsLimitedDown,
			IsLimitedUp:       info.IsLimitedUp,
		}
	}
	userpkg.HookCheckTrafficAfterQuotaUpdate = CheckTrafficAfterQuotaUpdate
	userpkg.HookRebalanceUserBandwidth = RebalanceUserBandwidth

	// ── Maintenance / Email hooks ──
	userpkg.HookIsMaintenanceModeEnabled = IsMaintenanceModeEnabled
	userpkg.HookIsLibvirtUnavailableText = isLibvirtUnavailableText
	userpkg.HookIsLibvirtUnavailableError = IsLibvirtUnavailableError
	userpkg.HookSendEmail = SendEmail
}

// ── Core user delegates ──

func ListUsers() ([]VMUserInfo, error) {
	return userpkg.ListUsers()
}

func CreateSystemUser(username, password, role string, maxCPU, maxMemory, maxDisk, maxVM, maxStorage, maxRuntimeHours int, enablePortForward bool, maxPortForwards, maxSnapshots int, maxBandwidthUp, maxBandwidthDown, maxTrafficDown, maxTrafficUp float64) error {
	return userpkg.CreateSystemUser(username, password, role, maxCPU, maxMemory, maxDisk, maxVM, maxStorage, maxRuntimeHours, enablePortForward, maxPortForwards, maxSnapshots, maxBandwidthUp, maxBandwidthDown, maxTrafficDown, maxTrafficUp)
}

func ProvisionSystemUserResources(user *model.User, password string) error {
	return userpkg.ProvisionSystemUserResources(user, password)
}

func FindVMOwner(vmName string) string {
	return userpkg.FindVMOwner(vmName)
}

func UpdateUserStatus(username, targetStatus string) error {
	return userpkg.UpdateUserStatus(username, targetStatus)
}

func DisableUserAccount(username string, progressFn func(int, string)) (*UserStatusChangeResult, error) {
	return userpkg.DisableUserAccount(username, progressFn)
}

func DeleteSystemUser(username string, progressFn func(int, string)) error {
	return userpkg.DeleteSystemUser(username, progressFn)
}

// ── VM assignment delegates ──

func AssignVMsToUser(username string, vmNames []string) error {
	return userpkg.AssignVMsToUser(username, vmNames)
}

func AssignVMsToUserWithQuotas(username string, vmNames []string, lightweightQuotas []LightweightVMQuotaRequest) error {
	userQuotas := make([]userpkg.LightweightVMQuotaRequest, len(lightweightQuotas))
	for i, q := range lightweightQuotas {
		userQuotas[i] = userpkg.LightweightVMQuotaRequest{
			VMName:            q.VMName,
			TrafficDownGB:     q.TrafficDownGB,
			TrafficUpGB:       q.TrafficUpGB,
			BandwidthDownMbps: q.BandwidthDownMbps,
			BandwidthUpMbps:   q.BandwidthUpMbps,
			MaxPortForwards:   q.MaxPortForwards,
			MaxSnapshots:      q.MaxSnapshots,
			MaxRuntimeHours:   q.MaxRuntimeHours,
		}
	}
	return userpkg.AssignVMsToUserWithQuotas(username, vmNames, userQuotas)
}

// ── Quota delegates ──

func GetUserVMList(username string) []string {
	return userpkg.GetUserVMList(username)
}

func GetUserQuotaUsage(username string) (*QuotaUsage, error) {
	return userpkg.GetUserQuotaUsage(username)
}

func CheckQuota(username string, reqCPU, reqMemoryGB, reqDiskGB int) error {
	return userpkg.CheckQuota(username, reqCPU, reqMemoryGB, reqDiskGB)
}

func CheckQuotaForEdit(username string, deltaCPU, deltaMemoryGB, deltaDiskGB int) error {
	return userpkg.CheckQuotaForEdit(username, deltaCPU, deltaMemoryGB, deltaDiskGB)
}

func CheckQuotaForStart(username string, vmName string) error {
	return userpkg.CheckQuotaForStart(username, vmName)
}

func AddVMToUser(username, vmName string) error {
	return userpkg.AddVMToUser(username, vmName)
}

func RemoveVMFromUser(username, vmName string) error {
	return userpkg.RemoveVMFromUser(username, vmName)
}

func UserOwnsVM(username, vmName string) bool {
	return userpkg.UserOwnsVM(username, vmName)
}

func UserOwnsVMorLXC(username, name string) bool {
	return userpkg.UserOwnsVMorLXC(username, name)
}

func GetUserVMandContainerList(username string) []string {
	return userpkg.GetUserVMandContainerList(username)
}

func UpdateUserQuota(username string, maxCPU, maxMemory, maxDisk, maxVM, maxStorage, maxRuntimeHours int, enablePortForward bool, maxPortForwards, maxSnapshots int, maxBandwidthUp, maxBandwidthDown, maxTrafficDown, maxTrafficUp float64, maxPublicIPs int) error {
	return userpkg.UpdateUserQuota(username, maxCPU, maxMemory, maxDisk, maxVM, maxStorage, maxRuntimeHours, enablePortForward, maxPortForwards, maxSnapshots, maxBandwidthUp, maxBandwidthDown, maxTrafficDown, maxTrafficUp, maxPublicIPs)
}

func GetRunningVMsResourceUsage(username string) (runningCPU int, runningMemoryMB int, err error) {
	return userpkg.GetRunningVMsResourceUsage(username)
}

func GetVMCPUAndMemory(vmName string) (cpu int, memMB int) {
	return userpkg.GetVMCPUAndMemory(vmName)
}

func GetVMDiskDevCapacityGB(vmName, dev string) int {
	return userpkg.GetVMDiskDevCapacityGB(vmName, dev)
}

// ── Storage delegates ──

func GetUserISODir(username string) string {
	return userpkg.GetUserISODir(username)
}

func GetUserShareDir(username string) string {
	return userpkg.GetUserShareDir(username)
}

func GetUserDiskDir(username string) string {
	return userpkg.GetUserDiskDir(username)
}

func InitUserStorage(username string) error {
	return userpkg.InitUserStorage(username)
}

func IsStorageInitialized(username string) bool {
	return userpkg.IsStorageInitialized(username)
}

func GetUserStorageInfo(username string) (*UserStorageInfo, error) {
	return userpkg.GetUserStorageInfo(username)
}

func CheckStorageQuota(username string, additionalBytes int64) error {
	return userpkg.CheckStorageQuota(username, additionalBytes)
}

func IsStorageReadonly(username string) bool {
	return userpkg.IsStorageReadonly(username)
}

func ListUserFiles(username, category string) ([]UserFileInfo, error) {
	return userpkg.ListUserFiles(username, category)
}

func DeleteUserFile(username, category, filename string) error {
	return userpkg.DeleteUserFile(username, category, filename)
}

func GetUserFilePath(username, category, filename string) (string, error) {
	return userpkg.GetUserFilePath(username, category, filename)
}

func GetUserISOs(username string) []ISOFileInfo {
	userResult := userpkg.GetUserISOs(username)
	result := make([]ISOFileInfo, len(userResult))
	for i, iso := range userResult {
		result[i] = ISOFileInfo{
			Name:      iso.Name,
			Path:      iso.Path,
			Size:      iso.Size,
			SizeBytes: iso.SizeBytes,
			Pool:      iso.Pool,
			OSType:    iso.OSType,
			OSVariant: iso.OSVariant,
			MinDisk:   iso.MinDisk,
		}
	}
	return result
}

func MountStorageToVM(username, vmName, category string, readonly bool) error {
	return userpkg.MountStorageToVM(username, vmName, category, readonly)
}

func UnmountStorageFromVM(vmName, tag string) error {
	return userpkg.UnmountStorageFromVM(vmName, tag)
}

func FormatBytesPublic(bytes int64) string {
	return userpkg.FormatBytesPublic(bytes)
}

// ── SSH delegates ──

func SetUserSSH(username string, enabled bool) error {
	return userpkg.SetUserSSH(username, enabled)
}

func GetUserSSHStatus(username string) (bool, error) {
	return userpkg.GetUserSSHStatus(username)
}

func SyncSSHDenyConfig() {
	userpkg.SyncSSHDenyConfig()
}

// ── Runtime quota delegates ──

func InitializeUserRuntimeQuotaTracker() {
	userpkg.InitializeUserRuntimeQuotaTracker()
}

func BuildUserRuntimeQuotaSnapshot(user *model.User, observedAt time.Time) UserRuntimeQuotaSnapshot {
	return userpkg.BuildUserRuntimeQuotaSnapshot(user, observedAt)
}

func FormatRuntimeQuotaDuration(seconds int64) string {
	return userpkg.FormatRuntimeQuotaDuration(seconds)
}

func CheckRuntimeQuotaAvailable(username string) error {
	return userpkg.CheckRuntimeQuotaAvailable(username)
}

func CheckRuntimeQuotaAvailableForUser(user *model.User, observedAt time.Time) error {
	return userpkg.CheckRuntimeQuotaAvailableForUser(user, observedAt)
}

func SyncAllUserRuntimeQuotaStates(observedAt time.Time) {
	userpkg.SyncAllUserRuntimeQuotaStates(observedAt)
}

func SyncAllUserRuntimeQuotaStatesWithActiveVMs(activeVMs map[string]struct{}, observedAt time.Time) {
	userpkg.SyncAllUserRuntimeQuotaStatesWithActiveVMs(activeVMs, observedAt)
}

func SyncUserRuntimeQuotaState(username string, observedAt time.Time) {
	userpkg.SyncUserRuntimeQuotaState(username, observedAt)
}

func EnforceUserRuntimeQuotaShutdown(username string, progressFn func(int, string)) (*RuntimeQuotaShutdownResult, error) {
	return userpkg.EnforceUserRuntimeQuotaShutdown(username, progressFn)
}

// ── Unexported function delegates (used by register files) ──

func waitVMShutdownForDisable(vmName string, timeout time.Duration) bool {
	return userpkg.WaitVMShutdownForDisable(vmName, timeout)
}

func getRuntimeActiveVMSetFromHost() (map[string]struct{}, error) {
	return userpkg.GetRuntimeActiveVMSetFromHost()
}
