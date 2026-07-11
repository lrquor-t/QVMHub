package probe

import "qvmhub/model"

// ── Probe status constants ──

const (
	PortForwardProbeStatusNotApplicable       = "not_applicable"
	PortForwardProbeStatusPending             = "pending"
	PortForwardProbeStatusClear               = "clear"
	PortForwardProbeStatusHTTPBanned          = "http_banned"
	PortForwardProbeStatusHTTPWhitelisted     = "http_whitelisted"
	PortForwardProbeStatusRestoredByWhitelist = "restored_by_whitelist"
	PortForwardProbeStatusError               = "error"
	PortForwardWhitelistScopeAdmin            = "admin"
	PortForwardWhitelistScopeNone             = ""
)

// ── Internal constants ──

const (
	portForwardProbeSchedulerGroup = "网络安全"
	portForwardProbeSchedulerKey   = "port_forward_http_probe"
	portForwardProbeSchedulerName  = "端口转发 HTTP 探测"
	portForwardProbeBanReason      = "检测到存在建站或HTTP访问且未报备，当前转发已封禁，请联系管理员"
)

// ── Exported types ──

// PortForwardWhitelistSummary 白名单摘要
type PortForwardWhitelistSummary struct {
	VMName               string `json:"vm_name"`
	Username             string `json:"username"`
	UserWhitelisted      bool   `json:"user_whitelisted"`
	VMWhitelisted        bool   `json:"vm_whitelisted"`
	EffectiveWhitelisted bool   `json:"effective_whitelisted"`
	EffectiveScope       string `json:"effective_scope"`
}

// PortForwardWhitelistList 白名单列表
type PortForwardWhitelistList struct {
	Users []model.PortForwardWhitelist `json:"users"`
	VMs   []model.PortForwardWhitelist `json:"vms"`
}

// PortForwardHTTPProbeTaskParams 手动探测任务参数
type PortForwardHTTPProbeTaskParams struct {
	VMName string `json:"vm_name"`
}

// PortForwardHTTPProbeRunResult 探测运行结果
type PortForwardHTTPProbeRunResult struct {
	Scanned      int      `json:"scanned"`
	Banned       int      `json:"banned"`
	Whitelisted  int      `json:"whitelisted"`
	Clear        int      `json:"clear"`
	Skipped      int      `json:"skipped"`
	Errors       int      `json:"errors"`
	MatchedVM    string   `json:"matched_vm"`
	ErrorDetails []string `json:"error_details,omitempty"`
}

// portForwardWhitelistSet 内存白名单集合（private）
type portForwardWhitelistSet struct {
	user map[string]bool
	vm   map[string]bool
}
