package service

import (
	"context"
	"os"

	ovspkg "qvmhub/service/ovs"
)

// ── Type aliases ──

type OVSStaticHost = ovspkg.OVSStaticHost
type OVSDHCPLease = ovspkg.OVSDHCPLease
type ovsRuntimeInterface = ovspkg.OvsRuntimeInterface

// ── Diagnostic type aliases ──

type OVSStatus = ovspkg.OVSStatus
type OVSServiceStatus = ovspkg.OVSServiceStatus
type OVSRuleStatus = ovspkg.OVSRuleStatus
type OVSCommandFailure = ovspkg.OVSCommandFailure
type OVSPortList = ovspkg.OVSPortList
type OVSPortStatus = ovspkg.OVSPortStatus
type OVSLeaseStatus = ovspkg.OVSLeaseStatus
type OVSStaticHostInfo = ovspkg.OVSStaticHostInfo
type OVSDHCPLeaseInfo = ovspkg.OVSDHCPLeaseInfo
type OVSLeaseConflict = ovspkg.OVSLeaseConflict
type OVSCheckResult = ovspkg.OVSCheckResult
type VMNetworkRuntimeStatus = ovspkg.VMNetworkRuntimeStatus
type VMNetworkInterface = ovspkg.VMNetworkInterface
type OVSBandwidthReadStatus = ovspkg.OVSBandwidthReadStatus

// init wires OVS package function variables to service root implementations.
// This breaks the circular dependency: ovs package cannot import service,
// so it exposes function variables that we set here.
func init() {
	ovspkg.HookEnsureOVSBridgeExists = EnsureOVSBridgeExists
	ovspkg.HookBuildOVSVirtInstallNetworkArgForBridge = BuildOVSVirtInstallNetworkArgForBridge
	ovspkg.HookBuildOVSInterfaceXMLForBridge = BuildOVSInterfaceXMLForBridge
}

// ── Exported delegates (used by handler and other service files) ──

// BuildOVSVirtInstallNetworkArg delegates to ovs.BuildOVSVirtInstallNetworkArg
func BuildOVSVirtInstallNetworkArg(model string) string {
	return ovspkg.BuildOVSVirtInstallNetworkArg(model)
}

// BuildOVSInterfaceXML delegates to ovs.BuildOVSInterfaceXML
func BuildOVSInterfaceXML(mac, model string) string {
	return ovspkg.BuildOVSInterfaceXML(mac, model)
}

// BuildOVSInterfaceXMLWithVLAN delegates to ovs.BuildOVSInterfaceXMLWithVLAN
func BuildOVSInterfaceXMLWithVLAN(mac, model string, vlanID int) string {
	return ovspkg.BuildOVSInterfaceXMLWithVLAN(mac, model, vlanID)
}

// EnsureOVSNetworkReady delegates to ovs.EnsureOVSNetworkReady
func EnsureOVSNetworkReady() error {
	return ovspkg.EnsureOVSNetworkReady()
}

// ReloadOVSDNSMasq delegates to ovs.ReloadOVSDNSMasq
func ReloadOVSDNSMasq() {
	ovspkg.ReloadOVSDNSMasq()
}

// ListOVSStaticHosts delegates to ovs.ListOVSStaticHosts
func ListOVSStaticHosts() ([]ovspkg.OVSStaticHost, error) {
	return ovspkg.ListOVSStaticHosts()
}

// ParseOVSStaticHostsText delegates to ovs.ParseOVSStaticHostsText
func ParseOVSStaticHostsText(text string) []ovspkg.OVSStaticHost {
	return ovspkg.ParseOVSStaticHostsText(text)
}

// ListVPCStaticHosts delegates to ovs.ListVPCStaticHosts
func ListVPCStaticHosts(switchID uint) ([]ovspkg.OVSStaticHost, error) {
	return ovspkg.ListVPCStaticHosts(switchID)
}

// ListAllVPCStaticHosts delegates to ovs.ListAllVPCStaticHosts
func ListAllVPCStaticHosts() ([]ovspkg.OVSStaticHost, error) {
	return ovspkg.ListAllVPCStaticHosts()
}

// UpsertOVSStaticHost delegates to ovs.UpsertOVSStaticHost
func UpsertOVSStaticHost(vmName, mac, ipAddr string) error {
	return ovspkg.UpsertOVSStaticHost(vmName, mac, ipAddr)
}

// RemoveOVSStaticHost delegates to ovs.RemoveOVSStaticHost
func RemoveOVSStaticHost(vmName, mac string) (string, error) {
	return ovspkg.RemoveOVSStaticHost(vmName, mac)
}

// ListOVSDHCPLeases delegates to ovs.ListOVSDHCPLeases
func ListOVSDHCPLeases() ([]ovspkg.OVSDHCPLease, error) {
	return ovspkg.ListOVSDHCPLeases()
}

// ListVPCDHCPLeases delegates to ovs.ListVPCDHCPLeases
func ListVPCDHCPLeases() ([]ovspkg.OVSDHCPLease, error) {
	return ovspkg.ListVPCDHCPLeases()
}

// ListVPCDHCPLeasesForSwitch delegates to ovs.ListVPCDHCPLeasesForSwitch
func ListVPCDHCPLeasesForSwitch(switchID uint) ([]ovspkg.OVSDHCPLease, error) {
	return ovspkg.ListVPCDHCPLeasesForSwitch(switchID)
}

// ParseOVSDHCPLeasesText delegates to ovs.ParseOVSDHCPLeasesText
func ParseOVSDHCPLeasesText(text string) []ovspkg.OVSDHCPLease {
	return ovspkg.ParseOVSDHCPLeasesText(text)
}

// CleanOVSDHCPLease delegates to ovs.CleanOVSDHCPLease
func CleanOVSDHCPLease(mac, ipAddr string) {
	ovspkg.CleanOVSDHCPLease(mac, ipAddr)
}

// CleanVPCDHCPLease delegates to ovs.CleanVPCDHCPLease
func CleanVPCDHCPLease(switchID uint, mac, ipAddr string) {
	ovspkg.CleanVPCDHCPLease(switchID, mac, ipAddr)
}

// CleanAllVPCDHCPLeases delegates to ovs.CleanAllVPCDHCPLeases
func CleanAllVPCDHCPLeases(mac, ipAddr string) {
	ovspkg.CleanAllVPCDHCPLeases(mac, ipAddr)
}

// GetOVSStaticIPByMAC delegates to ovs.GetOVSStaticIPByMAC
func GetOVSStaticIPByMAC(mac string) string {
	return ovspkg.GetOVSStaticIPByMAC(mac)
}

// GetOVSStaticHostByVMName delegates to ovs.GetOVSStaticHostByVMName
func GetOVSStaticHostByVMName(vmName string) (ovspkg.OVSStaticHost, bool) {
	return ovspkg.GetOVSStaticHostByVMName(vmName)
}

// GetOVSLeaseIPByMAC delegates to ovs.GetOVSLeaseIPByMAC
func GetOVSLeaseIPByMAC(mac string) string {
	return ovspkg.GetOVSLeaseIPByMAC(mac)
}

// GetVPCLeaseIPForVMByMAC delegates to ovs.GetVPCLeaseIPForVMByMAC
func GetVPCLeaseIPForVMByMAC(vmName, mac string) string {
	return ovspkg.GetVPCLeaseIPForVMByMAC(vmName, mac)
}

// GetVPCLeaseIPForVM delegates to ovs.GetVPCLeaseIPForVM
func GetVPCLeaseIPForVM(vmName string) string {
	return ovspkg.GetVPCLeaseIPForVM(vmName)
}

// normalizeIPForOVS delegates to ovs.NormalizeIPForOVS
func normalizeIPForOVS(ipAddr string) string {
	return ovspkg.NormalizeIPForOVS(ipAddr)
}

// GetOVSStatus delegates to ovs.GetOVSStatus
func GetOVSStatus() (*ovspkg.OVSStatus, error) {
	return ovspkg.GetOVSStatus()
}

// GetOVSPorts delegates to ovs.GetOVSPorts
func GetOVSPorts() (*ovspkg.OVSPortList, error) {
	return ovspkg.GetOVSPorts()
}

// GetOVSLeasesStatus delegates to ovs.GetOVSLeasesStatus
func GetOVSLeasesStatus() (*ovspkg.OVSLeaseStatus, error) {
	return ovspkg.GetOVSLeasesStatus()
}

// CheckOVSNetwork delegates to ovs.CheckOVSNetwork
func CheckOVSNetwork() (*ovspkg.OVSCheckResult, error) {
	return ovspkg.CheckOVSNetwork()
}

// RepairOVSNetwork delegates to ovs.RepairOVSNetwork
func RepairOVSNetwork(ctx context.Context, progress func(int, string)) (string, error) {
	return ovspkg.RepairOVSNetwork(ctx, progress)
}

// GetVMNetworkRuntimeStatus delegates to ovs.GetVMNetworkRuntimeStatus
func GetVMNetworkRuntimeStatus(vmName string) (*ovspkg.VMNetworkRuntimeStatus, error) {
	return ovspkg.GetVMNetworkRuntimeStatus(vmName)
}

// ── Unexported delegates (used by other register files within service root) ──

// useOVSNetwork delegates to ovs.UseOVSNetwork
func useOVSNetwork() bool {
	return ovspkg.UseOVSNetwork()
}

// ovsBridgeName delegates to ovs.OvsBridgeName
func ovsBridgeName() string {
	return ovspkg.OvsBridgeName()
}

// ovsSubnetPrefix delegates to ovs.OvsSubnetPrefix
func ovsSubnetPrefix() string {
	return ovspkg.OvsSubnetPrefix()
}

// ovsSubnetCIDR delegates to ovs.OvsSubnetCIDR
func ovsSubnetCIDR() string {
	return ovspkg.OvsSubnetCIDR()
}

// ovsUplink delegates to ovs.OvsUplink
func ovsUplink() string {
	return ovspkg.OvsUplink()
}

// ensureIPTablesRule delegates to ovs.EnsureIPTablesRule
func ensureIPTablesRule(checkCmd, addCmd, label string) error {
	return ovspkg.EnsureIPTablesRule(checkCmd, addCmd, label)
}

// ensureLocalDNSMasqInputRules delegates to ovs.EnsureLocalDNSMasqInputRules
func ensureLocalDNSMasqInputRules(iface string) error {
	return ovspkg.EnsureLocalDNSMasqInputRules(iface)
}

// removeLocalDNSMasqInputRules delegates to ovs.RemoveLocalDNSMasqInputRules
func removeLocalDNSMasqInputRules(iface string) {
	ovspkg.RemoveLocalDNSMasqInputRules(iface)
}

// writeFileIfChanged delegates to ovs.WriteFileIfChanged
func writeFileIfChanged(path string, content []byte, perm os.FileMode) (bool, error) {
	return ovspkg.WriteFileIfChanged(path, content, perm)
}

// buildOVSStaticHostsForUpsert delegates to ovs.BuildOVSStaticHostsForUpsert
func buildOVSStaticHostsForUpsert(hosts []OVSStaticHost, target OVSStaticHost) ([]OVSStaticHost, error) {
	// Convert service root types to ovs package types
	converted := make([]ovspkg.OVSStaticHost, len(hosts))
	for i, h := range hosts {
		converted[i] = ovspkg.OVSStaticHost{VMName: h.VMName, MAC: h.MAC, IP: h.IP}
	}
	ovsTarget := ovspkg.OVSStaticHost{VMName: target.VMName, MAC: target.MAC, IP: target.IP}
	result, err := ovspkg.BuildOVSStaticHostsForUpsert(converted, ovsTarget)
	if err != nil {
		return nil, err
	}
	// Convert back to service root types
	out := make([]OVSStaticHost, len(result))
	for i, h := range result {
		out[i] = OVSStaticHost{VMName: h.VMName, MAC: h.MAC, IP: h.IP}
	}
	return out, nil
}

// writeOVSStaticHosts delegates to ovs.WriteOVSStaticHosts
func writeOVSStaticHosts(hosts []OVSStaticHost) error {
	converted := make([]ovspkg.OVSStaticHost, len(hosts))
	for i, h := range hosts {
		converted[i] = ovspkg.OVSStaticHost{VMName: h.VMName, MAC: h.MAC, IP: h.IP}
	}
	return ovspkg.WriteOVSStaticHosts(converted)
}

// writeVPCStaticHosts delegates to ovs.WriteVPCStaticHosts
func writeVPCStaticHosts(switchID uint, hosts []OVSStaticHost) error {
	converted := make([]ovspkg.OVSStaticHost, len(hosts))
	for i, h := range hosts {
		converted[i] = ovspkg.OVSStaticHost{VMName: h.VMName, MAC: h.MAC, IP: h.IP}
	}
	return ovspkg.WriteVPCStaticHosts(switchID, converted)
}

// newerOVSDHCPLease delegates to ovs.NewerOVSDHCPLease
func newerOVSDHCPLease(current, candidate OVSDHCPLease) OVSDHCPLease {
	a := ovspkg.OVSDHCPLease{
		ExpiryTime: current.ExpiryTime,
		ExpiryUnix: current.ExpiryUnix,
		MAC:        current.MAC,
		IP:         current.IP,
		Hostname:   current.Hostname,
		ClientID:   current.ClientID,
	}
	b := ovspkg.OVSDHCPLease{
		ExpiryTime: candidate.ExpiryTime,
		ExpiryUnix: candidate.ExpiryUnix,
		MAC:        candidate.MAC,
		IP:         candidate.IP,
		Hostname:   candidate.Hostname,
		ClientID:   candidate.ClientID,
	}
	winner := ovspkg.NewerOVSDHCPLease(a, b)
	return OVSDHCPLease{
		ExpiryTime: winner.ExpiryTime,
		ExpiryUnix: winner.ExpiryUnix,
		MAC:        winner.MAC,
		IP:         winner.IP,
		Hostname:   winner.Hostname,
		ClientID:   winner.ClientID,
	}
}

// parseVirshDomiflistOutput delegates to ovs.ParseVirshDomiflistOutput
func parseVirshDomiflistOutput(text string) []ovsRuntimeInterface {
	ifaceList := ovspkg.ParseVirshDomiflistOutput(text)
	result := make([]ovsRuntimeInterface, len(ifaceList))
	for i, iface := range ifaceList {
		result[i] = ovsRuntimeInterface{
			Name:   iface.Name,
			Type:   iface.Type,
			Source: iface.Source,
			Model:  iface.Model,
			MAC:    iface.MAC,
		}
	}
	return result
}
