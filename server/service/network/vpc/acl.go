package vpc

import (
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"qvmhub/config"
	"qvmhub/model"
	"qvmhub/utils"
)

func BuildVPCACLRules() (string, error) {
	table := config.GlobalConfig.VPCACLTable
	if strings.TrimSpace(table) == "" {
		table = "kvm_console_vpc_acl"
	}
	var bindings []model.VPCVMBinding
	model.DB.Find(&bindings)
	var b strings.Builder
	b.WriteString("table inet ")
	b.WriteString(table)
	b.WriteString(" {\n")
	b.WriteString("  chain forward {\n")
	b.WriteString("    type filter hook forward priority -40; policy accept;\n")
	var vmIPs []string
	for _, binding := range bindings {
		var sw model.VPCSwitch
		if err := model.DB.First(&sw, binding.SwitchID).Error; err == nil && HookSwitchUsesDirectBridge(sw) {
			continue
		}
		bindingIPs := vpcFirewallIPsForVM(binding.VMName)
		if len(bindingIPs) == 0 {
			continue
		}
		for _, vmIP := range bindingIPs {
			allows, err := buildVPCIngressAllowRules(binding, vmIP)
			if err != nil {
				return "", err
			}
			for _, line := range allows {
				b.WriteString(line)
			}
			b.WriteString(fmt.Sprintf("    ct status dnat ip daddr %s reject\n", vmIP))
			vmIPs = append(vmIPs, vmIP)
		}
	}
	b.WriteString("    ct state established,related accept\n")
	sort.Strings(vmIPs)
	for _, vmIP := range vmIPs {
		b.WriteString(fmt.Sprintf("    ip daddr %s reject\n", vmIP))
	}
	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.String(), nil
}

func vpcFirewallIPsForVM(vmName string) []string {
	candidates := []string{HookGetFirewallVMIP(vmName)}
	candidates = append(candidates, HookPublicIPNATPrivateIPsForVM(vmName)...)
	seen := map[string]bool{}
	var ips []string
	for _, candidate := range candidates {
		ip := normalizeFirewallIPv4(candidate)
		if ip == "" || seen[ip] {
			continue
		}
		seen[ip] = true
		ips = append(ips, ip)
	}
	sort.Strings(ips)
	return ips
}

func normalizeFirewallIPv4(ipText string) string {
	ipText = strings.TrimSpace(ipText)
	if ipText == "" || ipText == "unknown" {
		return ""
	}
	ipText = strings.Fields(ipText)[0]
	ipText = strings.TrimSuffix(ipText, "(静态)")
	if addr, err := netip.ParseAddr(ipText); err == nil && addr.Is4() {
		return ipText
	}
	return ""
}

func buildVPCIngressAllowRules(binding model.VPCVMBinding, vmIP string) ([]string, error) {
	var rules []model.VPCSecurityGroupRule
	model.DB.Where("security_group_id = ? AND direction = ?", binding.SecurityGroupID, "ingress").Find(&rules)
	var lines []string
	for _, rule := range rules {
		sources, err := resolveRuleSources(rule)
		if err != nil {
			return nil, err
		}
		for _, src := range sources {
			match := fmt.Sprintf("    ip daddr %s ip saddr %s", vmIP, src)
			switch rule.Protocol {
			case "tcp", "udp":
				portMatch := strconv.Itoa(rule.PortStart)
				if rule.PortEnd > rule.PortStart {
					portMatch = fmt.Sprintf("%d-%d", rule.PortStart, rule.PortEnd)
				}
				match += fmt.Sprintf(" %s dport %s accept\n", rule.Protocol, portMatch)
			case "icmp":
				match += " icmp type echo-request accept\n"
			default:
				match += " accept\n"
			}
			lines = append(lines, match)
		}
	}
	sort.Strings(lines)
	return lines, nil
}

func resolveRuleSources(rule model.VPCSecurityGroupRule) ([]string, error) {
	switch rule.TargetType {
	case "cidr":
		return []string{normalizeCIDROrIP(rule.TargetValue)}, nil
	case "switch":
		id, _ := strconv.Atoi(rule.TargetValue)
		var sw model.VPCSwitch
		if err := model.DB.First(&sw, id).Error; err != nil {
			return nil, fmt.Errorf("安全组规则引用的交换机不存在")
		}
		return []string{sw.CIDR}, nil
	case "security_group":
		id, _ := strconv.Atoi(rule.TargetValue)
		var bindings []model.VPCVMBinding
		model.DB.Where("security_group_id = ?", id).Find(&bindings)
		var sources []string
		for _, binding := range bindings {
			for _, ip := range vpcFirewallIPsForVM(binding.VMName) {
				sources = append(sources, ip+"/32")
			}
		}
		if len(sources) == 0 {
			return []string{"255.255.255.255/32"}, nil
		}
		return sources, nil
	default:
		return nil, fmt.Errorf("安全组规则目标类型无效")
	}
}

func PreviewVPCACLRules() (string, error) {
	return BuildVPCACLRules()
}

func ApplyVPCACLRules() error {
	rules, err := BuildVPCACLRules()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(VPCConfigDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(VPCConfigDir, "acl.nft")
	if err := os.WriteFile(path, []byte(rules), 0644); err != nil {
		return fmt.Errorf("写入 VPC ACL 规则失败: %w", err)
	}
	check := utils.ExecCommand("nft", "-c", "-f", path)
	if check.Error != nil {
		return fmt.Errorf("VPC ACL 规则校验失败: %s", check.Stderr)
	}
	table := config.GlobalConfig.VPCACLTable
	if table == "" {
		table = "kvm_console_vpc_acl"
	}
	result := utils.ExecShell(fmt.Sprintf("nft delete table inet %s 2>/dev/null || true; nft -f %s", utils.ShellSingleQuote(table), utils.ShellSingleQuote(path)))
	if result.Error != nil {
		return fmt.Errorf("应用 VPC ACL 失败: %s", result.Stderr)
	}
	HookRemoveVPCPortForwardAcceptRules()
	_ = HookSavePortForwardRules()
	return nil
}
