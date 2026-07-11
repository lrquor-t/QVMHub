package service

import (
	"context"

	"qvmhub/model"
	publicippkg "qvmhub/service/public_ip"
	ovspkg "qvmhub/service/ovs"
)

// init wires public_ip package function variables to service root implementations.
// This breaks the circular dependency: public_ip package cannot import service,
// so it exposes function variables that we set here.
func init() {
	// ── VM / User hooks ──
	publicippkg.HookFindVMOwner = FindVMOwner

	// ── OVS / Network hooks ──
	publicippkg.HookGetVPCLeaseIPForVM = ovspkg.GetVPCLeaseIPForVM
	publicippkg.HookGetOVSStaticHostByVMName = func(vmName string) (publicippkg.OVSStaticHost, bool) {
		host, ok := ovspkg.GetOVSStaticHostByVMName(vmName)
		return publicippkg.OVSStaticHost{VMName: host.VMName, MAC: host.MAC, IP: host.IP}, ok
	}
	publicippkg.HookGetVMNetworkRuntimeStatus = func(vmName string) (*publicippkg.VMNetworkRuntimeStatus, error) {
		status, err := GetVMNetworkRuntimeStatus(vmName)
		if err != nil {
			return nil, err
		}
		if status == nil {
			return nil, nil
		}
		ifaces := make([]publicippkg.VMNetworkInterface, len(status.Interfaces))
		for i, iface := range status.Interfaces {
			ifaces[i] = publicippkg.VMNetworkInterface{IP: iface.IP}
		}
		return &publicippkg.VMNetworkRuntimeStatus{Interfaces: ifaces}, nil
	}
	publicippkg.HookIsVPCManagedIP = IsVPCManagedIP
	publicippkg.HookApplyVPCACLRules = ApplyVPCACLRules
	publicippkg.HookOvsUplink = ovspkg.OvsUplink
	publicippkg.HookOvsBridgeName = ovspkg.OvsBridgeName
	publicippkg.HookOvsGatewayIP = ovspkg.OvsGatewayIP
	publicippkg.HookGetOVSInterfaceOfPort = getOVSInterfaceOfPort
	publicippkg.HookParseVirshDomiflistOutput = func(text string) []publicippkg.OVSRuntimeInterface {
		rows := parseVirshDomiflistOutput(text)
		result := make([]publicippkg.OVSRuntimeInterface, len(rows))
		for i, r := range rows {
			result[i] = publicippkg.OVSRuntimeInterface{
				Name:   r.Name,
				Type:   r.Type,
				Source: r.Source,
				Model:  r.Model,
				MAC:    r.MAC,
			}
		}
		return result
	}
	publicippkg.HookWriteFileIfChanged = ovspkg.WriteFileIfChanged
}

// ── Type aliases ──

type PublicIPRequest = publicippkg.PublicIPRequest
type PublicIPBindRequest = publicippkg.PublicIPBindRequest
type PublicIPOperationParams = publicippkg.PublicIPOperationParams
type PublicIPInfo = publicippkg.PublicIPInfo
type PublicIPPreview = publicippkg.PublicIPPreview
type PublicIPAttachment = publicippkg.PublicIPAttachment

// ── Exported delegates (used by handler and other service files) ──

func ListPublicIPs() ([]PublicIPInfo, error) {
	return publicippkg.ListPublicIPs()
}

func CreatePublicIP(req PublicIPRequest) (*model.PublicIP, error) {
	return publicippkg.CreatePublicIP(req)
}

func UpdatePublicIP(id uint, req PublicIPRequest) (*model.PublicIP, error) {
	return publicippkg.UpdatePublicIP(id, req)
}

func DeletePublicIP(id uint) error {
	return publicippkg.DeletePublicIP(id)
}

func PreviewPublicIPBinding(id uint, req PublicIPBindRequest) (*PublicIPPreview, error) {
	return publicippkg.PreviewPublicIPBinding(id, req)
}

func ExecutePublicIPOperation(ctx context.Context, params PublicIPOperationParams, progress func(int, string)) (string, error) {
	return publicippkg.ExecutePublicIPOperation(ctx, params, progress)
}

func ApplyPublicIPRules() error {
	return publicippkg.ApplyPublicIPRules()
}

func RestorePublicIPRules() error {
	return publicippkg.RestorePublicIPRules()
}

func BuildPublicIPRulesScript() (string, error) {
	return publicippkg.BuildPublicIPRulesScript()
}

func ResolvePublicIPVMPrivateIP(vmName string) string {
	return publicippkg.ResolvePublicIPVMPrivateIP(vmName)
}

func PublicIPNATPrivateIPsForVM(vmName string) []string {
	return publicippkg.PublicIPNATPrivateIPsForVM(vmName)
}

func GetUserPublicIPUsage(username string) int {
	return publicippkg.GetUserPublicIPUsage(username)
}

func NormalizePublicIPMode(mode string) string {
	return publicippkg.NormalizePublicIPMode(mode)
}

func PublicIPModeLabel(mode string) string {
	return publicippkg.PublicIPModeLabel(mode)
}

func ListPublicIPAttachmentsForVM(vmName string) []PublicIPAttachment {
	return publicippkg.ListPublicIPAttachmentsForVM(vmName)
}

func ParsePublicIPOperationParams(raw string) (PublicIPOperationParams, error) {
	return publicippkg.ParsePublicIPOperationParams(raw)
}

func ParsePublicIPID(raw string) (uint, error) {
	return publicippkg.ParsePublicIPID(raw)
}
