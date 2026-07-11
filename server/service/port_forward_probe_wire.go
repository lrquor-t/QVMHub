package service

import (
	"context"

	"qvmhub/model"
	probe "qvmhub/service/network/probe"
)

// init wires network/probe package function variables to service root implementations.
// This breaks the circular dependency: probe package cannot import service,
// so it exposes function variables that we set here.
func init() {
	probe.HookVMExists = func(vmName string) error {
		_, err := GetVM(vmName)
		return err
	}
	probe.HookFindVMOwner = FindVMOwner
	probe.HookIsMaintenanceModeEnabled = IsMaintenanceModeEnabled
}

// ── Type aliases ──

type PortForwardWhitelistSummary = probe.PortForwardWhitelistSummary
type PortForwardWhitelistList = probe.PortForwardWhitelistList
type PortForwardHTTPProbeTaskParams = probe.PortForwardHTTPProbeTaskParams
type PortForwardHTTPProbeRunResult = probe.PortForwardHTTPProbeRunResult

// ── Constant aliases ──

const (
	PortForwardProbeStatusNotApplicable       = probe.PortForwardProbeStatusNotApplicable
	PortForwardProbeStatusPending             = probe.PortForwardProbeStatusPending
	PortForwardProbeStatusClear               = probe.PortForwardProbeStatusClear
	PortForwardProbeStatusHTTPBanned          = probe.PortForwardProbeStatusHTTPBanned
	PortForwardProbeStatusHTTPWhitelisted     = probe.PortForwardProbeStatusHTTPWhitelisted
	PortForwardProbeStatusRestoredByWhitelist = probe.PortForwardProbeStatusRestoredByWhitelist
	PortForwardProbeStatusError               = probe.PortForwardProbeStatusError
	PortForwardWhitelistScopeAdmin            = probe.PortForwardWhitelistScopeAdmin
	PortForwardWhitelistScopeNone             = probe.PortForwardWhitelistScopeNone
)

// ── Exported delegates (used by handler and other service files) ──

// StartPortForwardHTTPProbeScheduler delegates to probe.StartPortForwardHTTPProbeScheduler
func StartPortForwardHTTPProbeScheduler() {
	probe.StartPortForwardHTTPProbeScheduler()
}

// ListPortForwardWhitelists delegates to probe.ListPortForwardWhitelists
func ListPortForwardWhitelists() (*PortForwardWhitelistList, error) {
	return probe.ListPortForwardWhitelists()
}

// AddPortForwardWhitelist delegates to probe.AddPortForwardWhitelist
func AddPortForwardWhitelist(scopeType, scopeValue, createdBy string) (*model.PortForwardWhitelist, []string, error) {
	return probe.AddPortForwardWhitelist(scopeType, scopeValue, createdBy)
}

// DeletePortForwardWhitelist delegates to probe.DeletePortForwardWhitelist
func DeletePortForwardWhitelist(scopeType, scopeValue string) error {
	return probe.DeletePortForwardWhitelist(scopeType, scopeValue)
}

// GetPortForwardWhitelistSummary delegates to probe.GetPortForwardWhitelistSummary
func GetPortForwardWhitelistSummary(vmName, username, role string) (*PortForwardWhitelistSummary, error) {
	return probe.GetPortForwardWhitelistSummary(vmName, username, role)
}

// RunPortForwardHTTPProbeScan delegates to probe.RunPortForwardHTTPProbeScan
func RunPortForwardHTTPProbeScan(ctx context.Context, vmName, trigger string, progress func(int, string)) (*PortForwardHTTPProbeRunResult, error) {
	return probe.RunPortForwardHTTPProbeScan(ctx, vmName, trigger, progress)
}

// ExecuteManualPortForwardHTTPProbe delegates to probe.ExecuteManualPortForwardHTTPProbe
func ExecuteManualPortForwardHTTPProbe(ctx context.Context, task *PortForwardHTTPProbeTaskParams, createdBy string, progress func(int, string)) (string, error) {
	return probe.ExecuteManualPortForwardHTTPProbe(ctx, task, createdBy, progress)
}

// DeleteBannedPortForwardByRuleKey delegates to probe.DeleteBannedPortForwardByRuleKey
func DeleteBannedPortForwardByRuleKey(ruleKey string) error {
	return probe.DeleteBannedPortForwardByRuleKey(ruleKey)
}

// SyncPortForwardProbeStateOnAdd delegates to probe.SyncPortForwardProbeStateOnAdd
func SyncPortForwardProbeStateOnAdd(params *PortForwardAddParams, protocol string, ownerUsername string) {
	probe.SyncPortForwardProbeStateOnAdd(params, protocol, ownerUsername)
}

// SyncPortForwardProbeStateOnDelete delegates to probe.SyncPortForwardProbeStateOnDelete
func SyncPortForwardProbeStateOnDelete(ruleKey string, deletedByBan bool) {
	probe.SyncPortForwardProbeStateOnDelete(ruleKey, deletedByBan)
}

// MergePortForwardProbeState delegates to probe.MergePortForwardProbeState
func MergePortForwardProbeState(rules []PortForwardRule) []PortForwardRule {
	return probe.MergePortForwardProbeState(rules)
}
