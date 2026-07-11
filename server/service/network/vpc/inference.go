package vpc

import (
	"fmt"
	"strconv"
	"strings"

	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/service/ip_resolver"
	"qvmhub/utils"
)

func InferVPCSwitchForVM(vmName string) (*model.VPCSwitch, bool) {
	if strings.TrimSpace(vmName) == "" || model.DB == nil {
		return nil, false
	}
	if sw, found := inferDirectBridgeSwitchForVM(vmName); found {
		return repairMissingVPCBindingFromRuntime(vmName, sw)
	}
	if vlanID, ok := inferVPCVLANTagFromVMXML(vmName); ok {
		if sw, found := getVPCSwitchByVLANID(vlanID); found {
			return repairMissingVPCBindingFromRuntime(vmName, sw)
		}
	}
	if vnetIF := ip_resolver.GetVMVnetIF(vmName); vnetIF != "" {
		if tag, ok := getOVSPortTag(vnetIF); ok {
			if vlanID, err := strconv.Atoi(tag); err == nil && vlanID > 0 {
				if sw, found := getVPCSwitchByVLANID(vlanID); found {
					return repairMissingVPCBindingFromRuntime(vmName, sw)
				}
			}
		}
	}
	if mac := ip_resolver.GetFirstVMMAC(vmName); mac != "" {
		if sw, found := getVPCSwitchByOVSStaticHost(vmName, mac); found {
			return repairMissingVPCBindingFromRuntime(vmName, sw)
		}
		if sw, found := getVPCSwitchByMACRuntimeState(mac); found {
			return repairMissingVPCBindingFromRuntime(vmName, sw)
		}
	} else if sw, found := getVPCSwitchByOVSStaticHost(vmName, ""); found {
		return repairMissingVPCBindingFromRuntime(vmName, sw)
	}
	return nil, false
}

func inferDirectBridgeSwitchForVM(vmName string) (*model.VPCSwitch, bool) {
	for _, iface := range HookParseVirshDomiflist(utils.ExecCommand("virsh", "domiflist", vmName).Stdout) {
		if iface.Type == "bridge" && iface.Source != "" && iface.Source != HookOvsBridgeName() {
			if sw, found := getVPCSwitchByDirectBridge(iface.Source); found {
				return sw, true
			}
		}
	}
	for _, args := range [][]string{{"dumpxml", vmName, "--inactive"}, {"dumpxml", vmName}} {
		result := utils.ExecCommand("virsh", args...)
		if result.Error != nil {
			continue
		}
		if bridge, ok := extractFirstOVSInterfaceBridge(result.Stdout); ok && bridge != HookOvsBridgeName() {
			if sw, found := getVPCSwitchByDirectBridge(bridge); found {
				return sw, true
			}
		}
	}
	return nil, false
}

func inferVPCVLANTagFromVMXML(vmName string) (int, bool) {
	for _, args := range [][]string{
		{"dumpxml", vmName, "--inactive"},
		{"dumpxml", vmName},
	} {
		result := utils.ExecCommand("virsh", args...)
		if result.Error != nil {
			continue
		}
		if vlanID, ok := extractFirstOVSInterfaceVLANTag(result.Stdout); ok {
			return vlanID, true
		}
	}
	return 0, false
}

func getVPCSwitchByDirectBridge(bridgeName string) (*model.VPCSwitch, bool) {
	if strings.TrimSpace(bridgeName) == "" || model.DB == nil {
		return nil, false
	}
	var sw model.VPCSwitch
	if err := model.DB.Where("bridge_mode = ? AND bridge_name = ?", BridgeModeDirect, strings.TrimSpace(bridgeName)).Order("id ASC").First(&sw).Error; err != nil {
		return nil, false
	}
	return &sw, true
}

func getVPCSwitchByVLANID(vlanID int) (*model.VPCSwitch, bool) {
	if vlanID <= 0 || model.DB == nil {
		return nil, false
	}
	var sw model.VPCSwitch
	if err := model.DB.Where("vlan_id = ?", vlanID).First(&sw).Error; err != nil {
		return nil, false
	}
	return &sw, true
}

func getVPCSwitchByIP(ipText string) (*model.VPCSwitch, bool) {
	ipText = strings.TrimSpace(ipText)
	if ipText == "" || model.DB == nil {
		return nil, false
	}
	var switches []model.VPCSwitch
	if err := model.DB.Order("id ASC").Find(&switches).Error; err != nil {
		return nil, false
	}
	for _, sw := range switches {
		if IPInCIDR(ipText, sw.CIDR) {
			return &sw, true
		}
	}
	return nil, false
}

func getVPCSwitchByOVSStaticHost(vmName, mac string) (*model.VPCSwitch, bool) {
	vmName = strings.TrimSpace(vmName)
	mac = strings.ToLower(strings.TrimSpace(mac))
	hosts := []StaticHost{}
	if vmName != "" {
		if host, ok := HookGetOVSStaticHostByVMName(vmName); ok {
			hosts = append(hosts, host)
		}
	}
	if mac != "" {
		if ip := HookGetOVSStaticIPByMAC(mac); ip != "" {
			hosts = append(hosts, StaticHost{VMName: vmName, MAC: mac, IP: ip})
		}
	}
	for _, host := range hosts {
		if sw, ok := getVPCSwitchByIP(host.IP); ok {
			return sw, true
		}
	}
	return nil, false
}

func getVPCSwitchByMACRuntimeState(mac string) (*model.VPCSwitch, bool) {
	mac = strings.ToLower(strings.TrimSpace(mac))
	if mac == "" || model.DB == nil {
		return nil, false
	}
	var switches []model.VPCSwitch
	if err := model.DB.Order("id ASC").Find(&switches).Error; err != nil {
		return nil, false
	}
	for _, sw := range switches {
		if ip := HookGetVPCStaticIPByMAC(sw.ID, mac); ip != "" {
			return &sw, true
		}
		leases, err := HookListVPCDHCPLeases(sw.ID)
		if err != nil {
			continue
		}
		for _, lease := range leases {
			if strings.EqualFold(lease.MAC, mac) {
				return &sw, true
			}
		}
	}
	return nil, false
}

func repairMissingVPCBindingFromRuntime(vmName string, sw *model.VPCSwitch) (*model.VPCSwitch, bool) {
	if sw == nil || strings.TrimSpace(vmName) == "" || model.DB == nil {
		return sw, sw != nil
	}
	migrateOVSStaticHostToVPCIfNeeded(vmName, *sw)
	group, err := EnsureDefaultSecurityGroup(sw.Username)
	if err != nil {
		logger.App.Warn("VM 运行态属于 VPC 交换机但补默认安全组失败", "vm", vmName, "switch", sw.Name, "error", err)
		return sw, true
	}
	var existing model.VPCVMBinding
	if err := model.DB.Where("vm_name = ?", vmName).First(&existing).Error; err == nil {
		if existing.SwitchID == sw.ID && existing.SecurityGroupID == group.ID && existing.Username == sw.Username {
			return sw, true
		}
		existing.Username = sw.Username
		existing.SwitchID = sw.ID
		existing.SecurityGroupID = group.ID
		if saveErr := model.DB.Save(&existing).Error; saveErr != nil {
			logger.App.Warn("VM VPC 绑定记录修复失败", "vm", vmName, "error", saveErr)
			return sw, true
		}
		logger.App.Warn(fmt.Sprintf("已根据运行态修复 VM %s 的 VPC 绑定记录", vmName))
		return sw, true
	}
	binding := model.VPCVMBinding{
		VMName:          vmName,
		Username:        sw.Username,
		SwitchID:        sw.ID,
		SecurityGroupID: group.ID,
	}
	if createErr := model.DB.Create(&binding).Error; createErr != nil {
		logger.App.Warn("VM VPC 绑定记录补建失败", "vm", vmName, "error", createErr)
		return sw, true
	}
	logger.App.Warn(fmt.Sprintf("已根据运行态补建 VM %s 的 VPC 绑定记录", vmName))
	return sw, true
}

func migrateOVSStaticHostToVPCIfNeeded(vmName string, sw model.VPCSwitch) {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" || sw.ID == 0 {
		return
	}
	mac := strings.ToLower(strings.TrimSpace(ip_resolver.GetFirstVMMAC(vmName)))
	host, ok := HookGetOVSStaticHostByVMName(vmName)
	if !ok && mac != "" {
		if ip := HookGetOVSStaticIPByMAC(mac); ip != "" {
			host = StaticHost{VMName: vmName, MAC: mac, IP: ip}
			ok = true
		}
	}
	if !ok || strings.TrimSpace(host.IP) == "" || !IPInCIDR(host.IP, sw.CIDR) {
		return
	}
	host.VMName = vmName
	host.MAC = strings.ToLower(strings.TrimSpace(host.MAC))
	if host.MAC == "" {
		host.MAC = mac
	}
	if host.MAC == "" {
		return
	}
	if err := HookUpsertVPCStaticHost(sw, host.VMName, host.MAC, host.IP); err != nil {
		logger.App.Warn("迁移 VM VPC 静态 IP 绑定失败", "vm", vmName, "error", err)
		return
	}
	_, _ = HookRemoveOVSStaticHost(host.VMName, host.MAC)
	logger.App.Warn(fmt.Sprintf("已将 VM %s 的静态 IP %s 迁移到 VPC 交换机 %s", vmName, host.IP, sw.Name))
}
