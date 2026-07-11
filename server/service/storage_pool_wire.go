package service

import (
	"context"

	pool "qvmhub/service/storage/pool"
)

// ── Type aliases（向后兼容，让 service 根包和外部调用方可直接使用类型名） ──

type HostStoragePoolInfo = pool.HostStoragePoolInfo
type VMStorageTarget = pool.VMStorageTarget
type UpdateHostStoragePoolConfigRequest = pool.UpdateHostStoragePoolConfigRequest
type ISOFileInfo = pool.ISOFileInfo
type LVMVolumeRequest = pool.LVMVolumeRequest
type VGInfo = pool.VGInfo
type LVInfo = pool.LVInfo
type PVInfo = pool.PVInfo
type ZFSPoolRequest = pool.ZFSPoolRequest
type ZPoolInfo = pool.ZPoolInfo

// ── Exported delegates (used by handler and other service files) ──

// ListStoragePools delegates to pool.ListStoragePools
func ListStoragePools() ([]HostStoragePoolInfo, error) {
	return pool.ListStoragePools()
}

// GetStoragePool delegates to pool.GetStoragePool
func GetStoragePool(id string) (*HostStoragePoolInfo, error) {
	return pool.GetStoragePool(id)
}

// UpdateHostStoragePoolConfig delegates to pool.UpdateHostStoragePoolConfig
func UpdateHostStoragePoolConfig(id string, req UpdateHostStoragePoolConfigRequest) error {
	return pool.UpdateHostStoragePoolConfig(id, req)
}

// SetDefaultHostStoragePool delegates to pool.SetDefaultHostStoragePool
func SetDefaultHostStoragePool(id string) error {
	return pool.SetDefaultHostStoragePool(id)
}

// ListVMStorageTargets delegates to pool.ListVMStorageTargets
func ListVMStorageTargets(isAdmin bool) ([]VMStorageTarget, error) {
	return pool.ListVMStorageTargets(isAdmin)
}

// ResolveVMStorageDir delegates to pool.ResolveVMStorageDir
func ResolveVMStorageDir(poolID string, isAdmin bool) (string, string, error) {
	return pool.ResolveVMStorageDir(poolID, isAdmin)
}

// FormatAndMountStoragePool delegates to pool.FormatAndMountStoragePool
func FormatAndMountStoragePool(ctx context.Context, id string, fstype string, progress func(int, string)) error {
	return pool.FormatAndMountStoragePool(ctx, id, fstype, progress)
}

// CreatePartitionOnDisk delegates to pool.CreatePartitionOnDisk
func CreatePartitionOnDisk(ctx context.Context, deviceID string, sizeGB int, progress func(int, string)) error {
	return pool.CreatePartitionOnDisk(ctx, deviceID, sizeGB, progress)
}

// DeleteAllPartitionsOnDisk delegates to pool.DeleteAllPartitionsOnDisk
func DeleteAllPartitionsOnDisk(ctx context.Context, deviceID string, progress func(int, string)) error {
	return pool.DeleteAllPartitionsOnDisk(ctx, deviceID, progress)
}

// GetAllISOs delegates to pool.GetAllISOs
func GetAllISOs() ([]ISOFileInfo, error) {
	return pool.GetAllISOs()
}

// CreateLVMVolume delegates to pool.CreateLVMVolume
func CreateLVMVolume(ctx context.Context, req LVMVolumeRequest, progress func(int, string)) error {
	return pool.CreateLVMVolume(ctx, req, progress)
}

// GetAvailablePVTargets delegates to pool.GetAvailablePVTargets
func GetAvailablePVTargets() ([]HostStoragePoolInfo, error) {
	return pool.GetAvailablePVTargets()
}

// ListVGs delegates to pool.ListVGs
func ListVGs() ([]VGInfo, []LVInfo, []PVInfo, error) {
	return pool.ListVGs()
}

// DeleteLVMVolume delegates to pool.DeleteLVMVolume
func DeleteLVMVolume(ctx context.Context, vgName string, progress func(int, string)) error {
	return pool.DeleteLVMVolume(ctx, vgName, progress)
}

// CreateZFSPool delegates to pool.CreateZFSPool
func CreateZFSPool(ctx context.Context, req ZFSPoolRequest, progress func(int, string)) error {
	return pool.CreateZFSPool(ctx, req, progress)
}

// DeleteZFSPool delegates to pool.DeleteZFSPool
func DeleteZFSPool(ctx context.Context, poolName string, progress func(int, string)) error {
	return pool.DeleteZFSPool(ctx, poolName, progress)
}

// AddZFSVdevs delegates to pool.AddZFSVdevs
func AddZFSVdevs(poolName, vdevType string, deviceIDs []string) error {
	return pool.AddZFSVdevs(poolName, vdevType, deviceIDs)
}

// CreateZFSDataset delegates to pool.CreateZFSDataset
func CreateZFSDataset(poolName, dsName string) error {
	return pool.CreateZFSDataset(poolName, dsName)
}

// DeleteZFSDataset delegates to pool.DeleteZFSDataset
func DeleteZFSDataset(fullName string) error {
	return pool.DeleteZFSDataset(fullName)
}

// GetZFSProperties delegates to pool.GetZFSProperties
func GetZFSProperties(dataset string) (pool.ZFSPropertyInfo, error) {
	return pool.GetZFSProperties(dataset)
}

// SetZFSProperty delegates to pool.SetZFSProperty
func SetZFSProperty(dataset, prop, value string) error {
	return pool.SetZFSProperty(dataset, prop, value)
}

// ValidateZFSProperty delegates to pool.ValidateZFSProperty
func ValidateZFSProperty(prop, value string) error {
	return pool.ValidateZFSProperty(prop, value)
}

// GetZFSScrubStatus delegates to pool.GetZFSScrubStatus
func GetZFSScrubStatus(poolName string) (pool.ZFSScrubStatus, error) {
	return pool.GetZFSScrubStatus(poolName)
}

// StartZFSScrub delegates to pool.StartZFSScrub
func StartZFSScrub(poolName string) error {
	return pool.StartZFSScrub(poolName)
}

// StopZFSScrub delegates to pool.StopZFSScrub
func StopZFSScrub(poolName string) error {
	return pool.StopZFSScrub(poolName)
}

// ClearZFSErrors delegates to pool.ClearZFSErrors
func ClearZFSErrors(poolName string) error {
	return pool.ClearZFSErrors(poolName)
}

// GetZFSErrors delegates to pool.GetZFSErrors
func GetZFSErrors(poolName string) (pool.ZFSErrorList, error) {
	return pool.GetZFSErrors(poolName)
}

// ZFSAvailable delegates to pool.ZFSAvailable
func ZFSAvailable() bool {
	return pool.ZFSAvailable()
}

// ListZPools delegates to pool.ListZPools
func ListZPools() ([]ZPoolInfo, error) {
	return pool.ListZPools()
}

// ── Unexported delegates (used internally by service root package) ──

// inferOSFromISO delegates to pool.InferOSFromISO
// Kept unexported for backward compatibility with user_pkg_register.go
func inferOSFromISO(nameLower string) (osType, osVariant string) {
	return pool.InferOSFromISO(nameLower)
}

// buildISOInfo delegates to pool.BuildISOInfo
// Kept unexported for backward compatibility with user_pkg_register.go
func buildISOInfo(filePath, poolName string) ISOFileInfo {
	return pool.BuildISOInfo(filePath, poolName)
}
