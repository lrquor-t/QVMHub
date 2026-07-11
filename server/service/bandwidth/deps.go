package bandwidth

import "qvmhub/model"

// OVSStaticHost mirrors service.OVSStaticHost for use within the bandwidth package.
type OVSStaticHost struct {
	VMName string
	MAC    string
	IP     string
}

var (
	// ── OVS / Network hooks ──
	HookGetOVSStaticHostByVMName func(vmName string) (OVSStaticHost, bool)
	HookListAllVPCStaticHosts    func() ([]OVSStaticHost, error)
	HookGetOVSStaticIPByMAC      func(mac string) string
	HookGetOVSLeaseIPByMAC       func(mac string) string
	HookUseOVSNetwork            func() bool
	HookOvsBridgeName            func() string
	HookOvsSubnetCIDR            func() string
	HookVPCGatewayPortName       func(id uint) string

	// ── VM / User hooks ──
	HookGetUserVMList             func(username string) []string
	HookIsLightweightCloudVM      func(vmName string) bool
	HookIsUserTrafficLimited      func(username string) (bool, bool)
	HookInferVPCSwitchForVM       func(vmName string) (*model.VPCSwitch, bool)
	HookApplyVPCSwitchBandwidth   func(sw model.VPCSwitch) error
	HookRefreshVMCacheByNameAsync func(name string)
)
