package service

import (
	lwpkg "qvmhub/service/lightweight"
	vpcpkg "qvmhub/service/network/vpc"
	traffpkg "qvmhub/service/traffic"
)

// init wires traffic package hook variables to service root implementations,
// and registers traffic utility functions into other subpackage hooks.
// This breaks the circular dependency: traffic package cannot import service,
// so it exposes hook variables that we set here.
func init() {
	// ── 向 vpc / lightweight 子包注册 traffic 工具函数 ──
	vpcpkg.HookFormatTrafficBytes = traffpkg.FormatTrafficBytes
	lwpkg.HookFormatTrafficBytes = traffpkg.FormatTrafficBytes
}

// ── Type aliases（向后兼容，让 service 根包和外部调用方可直接使用类型名） ──

type TrafficUsageInfo = traffpkg.TrafficUsageInfo

// ── Exported delegates ──

func AggregateUserDailyTraffic(username string) (downBytes, upBytes int64) {
	return traffpkg.AggregateUserDailyTraffic(username)
}

func GetUserTrafficUsage(username string) *TrafficUsageInfo {
	return traffpkg.GetUserTrafficUsage(username)
}

func CheckAndApplyTrafficLimit(username string) {
	traffpkg.CheckAndApplyTrafficLimit(username)
}

func ResetUserTrafficQuota(username string) error {
	return traffpkg.ResetUserTrafficQuota(username)
}

func ResetAllDailyTraffic() {
	traffpkg.ResetAllDailyTraffic()
}

func CheckAllUsersTrafficQuota() {
	traffpkg.CheckAllUsersTrafficQuota()
}

func CheckTrafficAfterQuotaUpdate(username string) {
	traffpkg.CheckTrafficAfterQuotaUpdate(username)
}

func IsUserTrafficLimited(username string) (downLimited, upLimited bool) {
	return traffpkg.IsUserTrafficLimited(username)
}

func CheckUserTrafficQuotaForStart(username string) error {
	return traffpkg.CheckUserTrafficQuotaForStart(username)
}

func StartTrafficQuotaChecker() {
	traffpkg.StartTrafficQuotaChecker()
}

// ── Unexported delegates（供 service 根包其他 register 文件使用） ──

// formatTrafficBytes delegates to traffic.FormatTrafficBytes
func formatTrafficBytes(bytes int64) string {
	return traffpkg.FormatTrafficBytes(bytes)
}
