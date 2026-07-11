package public_ip

import (
	"net"
	"sort"
	"strings"

	"qvmhub/model"
)

func ResolvePublicIPVMPrivateIP(vmName string) string {
	if ip := HookGetVPCLeaseIPForVM(vmName); ip != "" {
		return ip
	}
	if host, ok := HookGetOVSStaticHostByVMName(vmName); ok {
		return host.IP
	}
	status, err := HookGetVMNetworkRuntimeStatus(vmName)
	if err == nil && status != nil {
		for _, iface := range status.Interfaces {
			if iface.IP != "" && net.ParseIP(iface.IP) != nil {
				return iface.IP
			}
		}
	}
	return ""
}

func PublicIPNATPrivateIPsForVM(vmName string) []string {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" || model.DB == nil {
		return []string{}
	}
	var bindings []model.PublicIPBinding
	if err := model.DB.Where("vm_name = ? AND mode = ?", vmName, PublicIPModeNAT).Find(&bindings).Error; err != nil {
		return []string{}
	}
	seen := map[string]bool{}
	var ips []string
	for _, binding := range bindings {
		ip := strings.TrimSpace(binding.VMPrivateIP)
		if ip == "" || net.ParseIP(ip) == nil || seen[ip] {
			continue
		}
		seen[ip] = true
		ips = append(ips, ip)
	}
	sort.Strings(ips)
	return ips
}

func GetUserPublicIPUsage(username string) int {
	var count int64
	if model.DB != nil {
		model.DB.Model(&model.PublicIPBinding{}).Where("username = ?", strings.TrimSpace(username)).Count(&count)
	}
	return int(count)
}
