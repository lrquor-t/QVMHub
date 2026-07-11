package migration

import "qvmhub/service"

func init() {
	service.HookEnsureVMNotMigrating = EnsureVMNotMigrating
	service.HookApplyVMUnderMigrationStatus = ApplyVMUnderMigrationStatus
	service.HookDetectMigrationModeFromState = DetectMigrationModeFromState
	service.HookMigrationModeLive = MigrationModeLive
}