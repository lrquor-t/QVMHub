package service

import sharepkg "qvmhub/service/share"

// init wires share package hook variables to service root implementations.
// This breaks the circular dependency: share package cannot import service,
// so it exposes hook variables that we set here.
// 当前 share 子包不依赖 service 根包函数，保留空 init 以遵循架构模式。
func init() {
	_ = sharepkg.ShareInfo{}
}

// ── Type aliases（向后兼容，让 service 根包和外部调用方可直接使用类型名） ──

type ShareInfo = sharepkg.ShareInfo

// ── Exported delegates ──

func ListShares(vmName string) ([]ShareInfo, error) {
	return sharepkg.ListShares(vmName)
}

func ListSharesInactive(vmName string) ([]ShareInfo, error) {
	return sharepkg.ListSharesInactive(vmName)
}

func AddShare(vmName, hostPath, tag, securityModel string, readonly bool) error {
	return sharepkg.AddShare(vmName, hostPath, tag, securityModel, readonly)
}

func RemoveShare(vmName, tag string) error {
	return sharepkg.RemoveShare(vmName, tag)
}
