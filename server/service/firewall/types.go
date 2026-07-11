package firewall

import (
	"fmt"
	"net/netip"
	"regexp"
	"sort"
	"strings"
)

// ── Constants ──

const (
	firewallDir        = "/etc/kvm-console/firewall"
	firewallPolicyFile = "/etc/kvm-console/firewall/policy.json"
	firewallRulesFile  = "/etc/kvm-console/firewall/rules.nft"
	firewallTable      = "kvm_console_fw"
	defaultGeoBaseURL  = "https://www.ipdeny.com/ipblocks/data/aggregated"
)

const (
	hostFirewallPanelPrefix          = "kvm-console:"
	hostFirewallProtectedSSHPrefix   = "kvm-console:protected:ssh"
	hostFirewallProtectedPanelPrefix = "kvm-console:protected:panel"
	hostFirewallPortForwardPrefix    = "kvm-console:port-forward"
	hostFirewallVNCComment           = "kvm-console:vnc-default"
)

// ── VM firewall policy types ──

// FirewallPolicy 防火墙策略，存放在本地 JSON 文件，避免依赖数据库。
type FirewallPolicy struct {
	Enabled                bool                          `json:"enabled"`
	Bridge                 string                        `json:"bridge"`
	VMSubnet               string                        `json:"vm_subnet"`
	OutboundEnabled        bool                          `json:"outbound_enabled"`
	InboundEnabled         bool                          `json:"inbound_enabled"`
	DisableVMIPv6          bool                          `json:"disable_vm_ipv6"`
	BlockAction            string                        `json:"block_action"`
	OutboundAllowedRegions []string                      `json:"outbound_allowed_regions"`
	InboundAllowedRegions  []string                      `json:"inbound_allowed_regions"`
	WhitelistCIDRs         []string                      `json:"whitelist_cidrs"`
	Regions                []FirewallRegion              `json:"regions"`
	VMOverrides            map[string]FirewallVMOverride `json:"vm_overrides"`
	PortForwardExemptions  map[string]bool               `json:"port_forward_exemptions"`
	GeoIPBaseURL           string                        `json:"geoip_base_url"`
	UpdatedAt              string                        `json:"updated_at"`
	AppliedAt              string                        `json:"applied_at"`
}

// FirewallRegion 表示一个区域的 IPv4 CIDR 集合。
type FirewallRegion struct {
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	CIDRs     []string `json:"cidrs"`
	Source    string   `json:"source"`
	UpdatedAt string   `json:"updated_at"`
}

// FirewallVMOverride 表示单台 VM 的覆盖策略。
type FirewallVMOverride struct {
	Mode    string   `json:"mode"`    // inherit/disabled/inbound_only/allow/block
	Regions []string `json:"regions"` // allow/block 模式下使用
}

// FirewallStatus 返回当前策略与系统实际状态。
type FirewallStatus struct {
	Policy         *FirewallPolicy `json:"policy"`
	Active         bool            `json:"active"`
	LastError      string          `json:"last_error"`
	RuleFile       string          `json:"rule_file"`
	PolicyFile     string          `json:"policy_file"`
	TableName      string          `json:"table_name"`
	NftAvailable   bool            `json:"nft_available"`
	VMs            []string        `json:"vms"`
	IPv6Note       string          `json:"ipv6_note"`
	GeoIPCopyright string          `json:"geoip_copyright"`
}

type FirewallImportParams struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	CIDRs  string `json:"cidrs"`
	Source string `json:"source"`
}

type FirewallGeoUpdateParams struct {
	Codes   []string `json:"codes"`
	BaseURL string   `json:"base_url"`
}

type FirewallOperationParams struct {
	Action string `json:"action"`
}

type firewallNetworkScope struct {
	IfName string
	CIDR   string
}

// ── Host firewall types ──

type HostFirewallRule struct {
	ID              string `json:"id"`
	Action          string `json:"action"`
	Protocol        string `json:"protocol"`
	PortStart       int    `json:"port_start"`
	PortEnd         int    `json:"port_end"`
	SourceCIDR      string `json:"source_cidr"`
	Comment         string `json:"comment"`
	Protected       bool   `json:"protected"`
	ProtectedReason string `json:"protected_reason"`
	ManagedByPanel  bool   `json:"managed_by_panel"`
	Raw             string `json:"raw"`
}

type HostFirewallStatus struct {
	Active              bool               `json:"active"`
	UFWAvailable        bool               `json:"ufw_available"`
	DefaultIncoming     string             `json:"default_incoming"`
	DefaultOutgoing     string             `json:"default_outgoing"`
	DefaultRouted       string             `json:"default_routed"`
	Rules               []HostFirewallRule `json:"rules"`
	ProtectedRules      []HostFirewallRule `json:"protected_rules"`
	RecommendedRules    []HostFirewallRule `json:"recommended_rules"`
	SSHPorts            []int              `json:"ssh_ports"`
	PanelPorts          []int              `json:"panel_ports"`
	DockerCompatible    bool               `json:"docker_compatible"`
	DockerCompatibility string             `json:"docker_compatibility"`
	LastError           string             `json:"last_error"`
}

type HostFirewallRuleRequest struct {
	Action     string `json:"action"`
	Protocol   string `json:"protocol"`
	PortStart  int    `json:"port_start"`
	PortEnd    int    `json:"port_end"`
	SourceCIDR string `json:"source_cidr"`
	Comment    string `json:"comment"`
}

type HostFirewallEnableRequest struct {
	Rules []HostFirewallRuleRequest `json:"rules"`
}

type HostFirewallConnection struct {
	Protocol    string `json:"protocol"`
	LocalIP     string `json:"local_ip"`
	LocalPort   int    `json:"local_port"`
	PeerIP      string `json:"peer_ip"`
	PeerPort    int    `json:"peer_port"`
	AllowedPort bool   `json:"allowed_port"`
}

type HostFirewallConnectionPreview struct {
	Mode        string                   `json:"mode"`
	Connections []HostFirewallConnection `json:"connections"`
	Count       int                      `json:"count"`
	Warning     string                   `json:"warning"`
}

type HostFirewallCloseConnectionsRequest struct {
	Mode string `json:"mode"`
}

// ── Port forward mirror type (avoids direct import of service/network) ──

// PortForwardRule mirrors the subset of network.PortForwardRule fields used by firewall logic.
type PortForwardRule struct {
	Protocol string
	HostPort string
	DestIP   string
	DestPort string
}

// StableKey returns a stable identifier for the port forward rule.
func (r PortForwardRule) StableKey() string {
	return strings.ToLower(strings.TrimSpace(r.Protocol)) + "|" +
		strings.TrimSpace(r.HostPort) + "|" +
		strings.TrimSpace(r.DestIP) + "|" +
		strings.TrimSpace(r.DestPort)
}

// ── Shared normalization / validation helpers ──

func normalizeRegionCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	code = regexp.MustCompile(`[^a-z0-9_-]`).ReplaceAllString(code, "_")
	return code
}

func normalizeOverrideMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "inherit":
		return "inherit"
	case "disabled", "inbound_only", "allow", "block":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return ""
	}
}

func normalizeBlockAction(action string) string {
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "drop" {
		return "drop"
	}
	return "reject"
}

func normalizeStringList(values []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, value := range values {
		value = normalizeRegionCode(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func normalizeCIDRList(values []string) []string {
	seen := make(map[string]bool)
	var prefixes []netip.Prefix
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if addr, err := netip.ParseAddr(value); err == nil && addr.Is4() {
			value = addr.String() + "/32"
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil || !prefix.Addr().Is4() {
			continue
		}
		prefix = prefix.Masked()
		value = prefix.String()
		if seen[value] {
			continue
		}
		seen[value] = true
		prefixes = append(prefixes, prefix)
	}

	sort.Slice(prefixes, func(i, j int) bool {
		startI, _, bitsI := ipv4PrefixRange(prefixes[i])
		startJ, _, bitsJ := ipv4PrefixRange(prefixes[j])
		if startI != startJ {
			return startI < startJ
		}
		return bitsI < bitsJ
	})

	var compacted []netip.Prefix
	for _, prefix := range prefixes {
		if prefixContainedInAny(prefix, compacted) {
			continue
		}
		compacted = append(compacted, prefix)
	}

	var result []string
	for _, prefix := range compacted {
		result = append(result, prefix.String())
	}
	sort.Strings(result)
	return result
}

func prefixContainedInAny(prefix netip.Prefix, existing []netip.Prefix) bool {
	start, end, _ := ipv4PrefixRange(prefix)
	for _, item := range existing {
		itemStart, itemEnd, _ := ipv4PrefixRange(item)
		if itemStart <= start && itemEnd >= end {
			return true
		}
	}
	return false
}

func ipv4PrefixRange(prefix netip.Prefix) (uint32, uint32, int) {
	addr := prefix.Masked().Addr().As4()
	start := uint32(addr[0])<<24 | uint32(addr[1])<<16 | uint32(addr[2])<<8 | uint32(addr[3])
	bits := prefix.Bits()
	var mask uint32
	if bits == 0 {
		mask = 0
	} else {
		mask = ^uint32(0) << uint(32-bits)
	}
	end := start | ^mask
	return start, end, bits
}

func sortedMapKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func vmSetName(vmName string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	name := strings.ToLower(re.ReplaceAllString(vmName, "_"))
	if name == "" {
		name = "vm"
	}
	return "vm_" + name + "_regions4"
}

func validateIPv4CIDROrAddr(value string) error {
	value = strings.TrimSpace(value)
	if addr, err := netip.ParseAddr(value); err == nil && addr.Is4() {
		return nil
	}
	prefix, err := netip.ParsePrefix(value)
	if err != nil || !prefix.Addr().Is4() {
		return fmt.Errorf("无效的 IPv4 CIDR")
	}
	return nil
}
