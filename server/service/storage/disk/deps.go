package disk

// External dependencies injected by service root package at startup.
// This file breaks the circular dependency: disk package must NOT import
// qvmhub/service, so these function variables are set by the caller
// (typically in an init() or explicit wiring step in the service package).

// EnsureNotMigrating checks whether a VM is currently under migration.
// Must be set by service root package before any disk operation is called.
var EnsureNotMigrating func(vmName, action string) error

// ResolveStorageDir resolves the disk directory for a given storage pool ID.
// Returns (dir, poolName, error). Must be set by service root package.
var ResolveStorageDir func(poolID string, isAdmin bool) (string, string, error)

// GetStorageMountPointFn returns the user storage mount point path.
// Must be set by service root package.
var GetStorageMountPointFn func() string

// GetUserDiskDirFn returns the virtual disk directory for a given user.
// Must be set by service root package.
var GetUserDiskDirFn func(username string) string

// CheckSnapshotSafety checks whether a VM has external snapshots that
// would block disk operations. Returns (hasExternal, snapshotNames, error).
// Must be set by service root package.
var CheckSnapshotSafety func(vmName string) (bool, []string, error)
