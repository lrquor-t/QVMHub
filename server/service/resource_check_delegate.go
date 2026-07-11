package service

import hostpkg "qvmhub/service/host"

// VMCreateCheckpoint 类型别名，保持 handler 层引用 service.VMCreateCheckpoint 不变
type VMCreateCheckpoint = hostpkg.VMCreateCheckpoint

func CheckStorageSpace(dir string, requiredMB int64) error {
	return hostpkg.CheckStorageSpace(dir, requiredMB)
}

func CheckDirWritable(dir string) error {
	return hostpkg.CheckDirWritable(dir)
}

func ValidateDiskBackingChain(diskPath string) error {
	return hostpkg.ValidateDiskBackingChain(diskPath)
}

func EnsureOVSBridgeExists(bridge string) error {
	return hostpkg.EnsureOVSBridgeExists(bridge)
}
