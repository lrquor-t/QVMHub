package service

import (
	"qvmhub/model"
	bwpkg "qvmhub/service/bandwidth"
	ovspkg "qvmhub/service/ovs"
)

// init wires bandwidth package function variables to service root implementations.
// This breaks the circular dependency: bandwidth package cannot import service,
// so it exposes function variables that we set here.
func init() {
	// ── OVS / Network hooks — now delegate to ovs subpackage ──
	bwpkg.HookGetOVSStaticHostByVMName = func(vmName string) (bwpkg.OVSStaticHost, bool) {
		host, ok := ovspkg.GetOVSStaticHostByVMName(vmName)
		return bwpkg.OVSStaticHost{VMName: host.VMName, MAC: host.MAC, IP: host.IP}, ok
	}
	bwpkg.HookListAllVPCStaticHosts = func() ([]bwpkg.OVSStaticHost, error) {
		hosts, err := ovspkg.ListAllVPCStaticHosts()
		if err != nil {
			return nil, err
		}
		result := make([]bwpkg.OVSStaticHost, len(hosts))
		for i, h := range hosts {
			result[i] = bwpkg.OVSStaticHost{VMName: h.VMName, MAC: h.MAC, IP: h.IP}
		}
		return result, nil
	}
	bwpkg.HookGetOVSStaticIPByMAC = ovspkg.GetOVSStaticIPByMAC
	bwpkg.HookGetOVSLeaseIPByMAC = ovspkg.GetOVSLeaseIPByMAC
	bwpkg.HookUseOVSNetwork = ovspkg.UseOVSNetwork
	bwpkg.HookOvsBridgeName = ovspkg.OvsBridgeName
	bwpkg.HookOvsSubnetCIDR = ovspkg.OvsSubnetCIDR
	bwpkg.HookVPCGatewayPortName = VPCGatewayPortName

	// ── VM / User hooks ──
	bwpkg.HookGetUserVMList = GetUserVMList
	bwpkg.HookIsLightweightCloudVM = IsLightweightCloudVM
	bwpkg.HookIsUserTrafficLimited = IsUserTrafficLimited
	bwpkg.HookInferVPCSwitchForVM = InferVPCSwitchForVM
	bwpkg.HookApplyVPCSwitchBandwidth = ApplyVPCSwitchBandwidth
	bwpkg.HookRefreshVMCacheByNameAsync = RefreshVMCacheByNameAsync
}

// ── Exported delegates (used by handler and other service files) ──

// ApplyVMBandwidth delegates to bandwidth.ApplyVMBandwidth
func ApplyVMBandwidth(vmName string, downAvg, downPeak, downBurst, upAvg, upPeak, upBurst int) error {
	return bwpkg.ApplyVMBandwidth(vmName, downAvg, downPeak, downBurst, upAvg, upPeak, upBurst)
}

// ApplyVMNICBandwidth delegates to bandwidth.ApplyVMNICBandwidth
func ApplyVMNICBandwidth(vmName string, downAvg, downPeak, downBurst, upAvg, upPeak, upBurst int) error {
	return bwpkg.ApplyVMNICBandwidth(vmName, downAvg, downPeak, downBurst, upAvg, upPeak, upBurst)
}

// ClearVMBandwidth delegates to bandwidth.ClearVMBandwidth
func ClearVMBandwidth(vmName string) error {
	return bwpkg.ClearVMBandwidth(vmName)
}

// GetVMBandwidth delegates to bandwidth.GetVMBandwidth
func GetVMBandwidth(vmName string) (*bwpkg.BandwidthDetail, error) {
	return bwpkg.GetVMBandwidth(vmName)
}

// ReapplyConfiguredVMBandwidth delegates to bandwidth.ReapplyConfiguredVMBandwidth
func ReapplyConfiguredVMBandwidth(vmName string) error {
	return bwpkg.ReapplyConfiguredVMBandwidth(vmName)
}

// SetVMCustomAverage delegates to bandwidth.SetVMCustomAverage
func SetVMCustomAverage(username, vmName string, inAvgMbps, outAvgMbps int) error {
	return bwpkg.SetVMCustomAverage(username, vmName, inAvgMbps, outAvgMbps)
}

// GetVMBandwidthMbps delegates to bandwidth.GetVMBandwidthMbps
func GetVMBandwidthMbps(vmName string) (int, int) {
	return bwpkg.GetVMBandwidthMbps(vmName)
}

// IsVPCBoundVM delegates to bandwidth.IsVPCBoundVM
func IsVPCBoundVM(vmName string) bool {
	return bwpkg.IsVPCBoundVM(vmName)
}

// RebalanceUserBandwidth delegates to bandwidth.RebalanceUserBandwidth
func RebalanceUserBandwidth(username string) error {
	return bwpkg.RebalanceUserBandwidth(username)
}

// ApplyGlobalBandwidthLimit delegates to bandwidth.ApplyGlobalBandwidthLimit
func ApplyGlobalBandwidthLimit() error {
	return bwpkg.ApplyGlobalBandwidthLimit()
}

// ClearGlobalBandwidthLimit delegates to bandwidth.ClearGlobalBandwidthLimit
func ClearGlobalBandwidthLimit() error {
	return bwpkg.ClearGlobalBandwidthLimit()
}

// ── Unexported delegates (used by other register files within service root) ──

// applyTCVPCSwitchDownlinkLimit delegates to bandwidth.ApplyTCVPCSwitchDownlinkLimit
func applyTCVPCSwitchDownlinkLimit(gwPort string, downMbps int) {
	bwpkg.ApplyTCVPCSwitchDownlinkLimit(gwPort, downMbps)
}

// clearTCVPCSwitchDownlinkLimit delegates to bandwidth.ClearTCVPCSwitchDownlinkLimit
func clearTCVPCSwitchDownlinkLimit(gwPort string) {
	bwpkg.ClearTCVPCSwitchDownlinkLimit(gwPort)
}

// addOVSBandwidthMeter delegates to bandwidth.AddOVSBandwidthMeter
func addOVSBandwidthMeter(bridge string, meterID uint32, rateKbit int) error {
	return bwpkg.AddOVSBandwidthMeter(bridge, meterID, rateKbit)
}

// getOVSInterfaceOfPort delegates to bandwidth.GetOVSInterfaceOfPort
func getOVSInterfaceOfPort(vnetIF string) string {
	return bwpkg.GetOVSInterfaceOfPort(vnetIF)
}

// getVPCSwitchForVM delegates to bandwidth.GetVPCSwitchForVM
func getVPCSwitchForVM(vmName string) (*model.VPCSwitch, bool) {
	return bwpkg.GetVPCSwitchForVM(vmName)
}

// getGlobalEffectiveBandwidth delegates to bandwidth.GetGlobalEffectiveBandwidth
func getGlobalEffectiveBandwidth() (int, int) {
	return bwpkg.GetGlobalEffectiveBandwidth()
}
