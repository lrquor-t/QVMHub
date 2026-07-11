package firewall

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/netip"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"qvmhub/config"
	"qvmhub/utils"
)

func GetHostFirewallStatus() (*HostFirewallStatus, error) {
	rules, err := ListHostFirewallRules()
	if err != nil {
		return nil, err
	}
	statusText := utils.ExecCommand("ufw", "status", "verbose")
	active := strings.Contains(strings.ToLower(statusText.Stdout), "status: active")
	defaultIncoming, defaultOutgoing, defaultRouted := parseUFWDefaults(statusText.Stdout)
	sshPorts := DetectSSHPorts()
	panelPorts := DetectPanelPorts()
	protected := buildProtectedHostFirewallRules(sshPorts, panelPorts)
	for i := range rules {
		markHostFirewallProtection(&rules[i], sshPorts, panelPorts)
	}
	return &HostFirewallStatus{
		Active:              active,
		UFWAvailable:        utils.ExecCommand("ufw", "--version").Error == nil,
		DefaultIncoming:     defaultIncoming,
		DefaultOutgoing:     defaultOutgoing,
		DefaultRouted:       defaultRouted,
		Rules:               rules,
		ProtectedRules:      protected,
		RecommendedRules:    BuildHostFirewallRecommendedRules(),
		SSHPorts:            sshPorts,
		PanelPorts:          panelPorts,
		DockerCompatible:    true,
		DockerCompatibility: "宿主机防火墙不写入 Docker 链，启用时保持 routed 默认允许，Docker bridge 模式不受面板防火墙约束。",
		LastError:           strings.TrimSpace(statusText.Stderr),
	}, nil
}

func ListHostFirewallRules() ([]HostFirewallRule, error) {
	result := utils.ExecCommand("ufw", "show", "added")
	if result.Error != nil {
		return nil, fmt.Errorf("读取 UFW 规则失败: %s", result.Stderr)
	}
	rules := parseUFWAddedRules(result.Stdout)
	sshPorts := DetectSSHPorts()
	panelPorts := DetectPanelPorts()
	for i := range rules {
		markHostFirewallProtection(&rules[i], sshPorts, panelPorts)
	}
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].Protected != rules[j].Protected {
			return rules[i].Protected
		}
		if rules[i].PortStart != rules[j].PortStart {
			return rules[i].PortStart < rules[j].PortStart
		}
		return rules[i].ID < rules[j].ID
	})
	return rules, nil
}

func PreviewEnableHostFirewall(req HostFirewallEnableRequest) (*HostFirewallStatus, error) {
	status, err := GetHostFirewallStatus()
	if err != nil {
		return nil, err
	}
	status.RecommendedRules = mergeHostFirewallRules(BuildHostFirewallRecommendedRules(), normalizeHostFirewallRuleRequests(req.Rules))
	return status, nil
}

func EnableHostFirewall(req HostFirewallEnableRequest, progress func(int, string)) error {
	if progress != nil {
		progress(10, "正在探测 SSH 和面板端口...")
	}
	allRules := mergeHostFirewallRules(buildProtectedHostFirewallRules(DetectSSHPorts(), DetectPanelPorts()), normalizeHostFirewallRuleRequests(req.Rules))
	if len(allRules) == 0 {
		return fmt.Errorf("未检测到需要保护的 SSH 或面板端口，已取消启用防火墙")
	}
	if progress != nil {
		progress(25, "正在补齐 UFW 基础策略...")
	}
	commands := [][]string{
		{"default", "deny", "incoming"},
		{"default", "allow", "outgoing"},
		{"default", "allow", "routed"},
	}
	for _, args := range commands {
		result := utils.ExecCommand("ufw", args...)
		if result.Error != nil {
			return fmt.Errorf("设置 UFW 默认策略失败: %s", result.Stderr)
		}
	}
	if progress != nil {
		progress(50, "正在写入确认后的放通规则...")
	}
	for _, rule := range allRules {
		if err := ensureHostFirewallRule(rule); err != nil {
			return err
		}
	}
	if progress != nil {
		progress(80, "正在启用宿主机防火墙...")
	}
	result := utils.ExecCommandWithTimeout("ufw", 2*time.Minute, "--force", "enable")
	if result.Error != nil {
		return fmt.Errorf("启用 UFW 失败: %s", result.Stderr)
	}
	if progress != nil {
		progress(100, "宿主机防火墙已启用")
	}
	return nil
}

func DisableHostFirewall(progress func(int, string)) error {
	if progress != nil {
		progress(20, "正在关闭宿主机防火墙...")
	}
	result := utils.ExecCommandWithTimeout("ufw", 2*time.Minute, "--force", "disable")
	if result.Error != nil {
		return fmt.Errorf("关闭 UFW 失败: %s", result.Stderr)
	}
	if progress != nil {
		progress(100, "宿主机防火墙已关闭")
	}
	return nil
}

func DetectSSHPorts() []int {
	ports := map[int]bool{}
	result := utils.ExecCommand("sshd", "-T")
	if result.Error == nil {
		for _, line := range strings.Split(result.Stdout, "\n") {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) == 2 && fields[0] == "port" {
				if port, err := strconv.Atoi(fields[1]); err == nil && port > 0 && port <= 65535 {
					ports[port] = true
				}
			}
		}
	}
	if len(ports) == 0 {
		result = utils.ExecShellQuiet(`ss -tlnp 2>/dev/null | grep -E 'sshd|/ssh' | awk '{print $4}' | grep -oE '[0-9]+$' | sort -un`)
		for _, line := range strings.Split(result.Stdout, "\n") {
			if port, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && port > 0 && port <= 65535 {
				ports[port] = true
			}
		}
	}
	if len(ports) == 0 {
		ports[22] = true
	}
	return sortedPorts(ports)
}

func DetectPanelPorts() []int {
	ports := map[int]bool{}
	if config.GlobalConfig != nil && config.GlobalConfig.Port > 0 {
		ports[config.GlobalConfig.Port] = true
	}
	result := utils.ExecShellQuiet(`ss -tlnp 2>/dev/null | grep -E 'kvm-console|server' | awk '{print $4}' | grep -oE '[0-9]+$' | sort -un`)
	for _, line := range strings.Split(result.Stdout, "\n") {
		if port, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && port > 0 && port <= 65535 {
			ports[port] = true
		}
	}
	return sortedPorts(ports)
}

func sortedPorts(values map[int]bool) []int {
	ports := make([]int, 0, len(values))
	for port := range values {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports
}

func parseUFWDefaults(text string) (string, string, string) {
	incoming, outgoing, routed := "", "", ""
	re := regexp.MustCompile(`(?i)default:\s*([^,]+)\s*\(incoming\),\s*([^,]+)\s*\(outgoing\)(?:,\s*([^\n]+)\s*\(routed\))?`)
	if m := re.FindStringSubmatch(text); len(m) > 0 {
		incoming = strings.TrimSpace(m[1])
		outgoing = strings.TrimSpace(m[2])
		if len(m) > 3 {
			routed = strings.TrimSpace(m[3])
		}
	}
	return incoming, outgoing, routed
}

// ── UFW rule parsing helpers ──

func parseUFWAddedRules(text string) []HostFirewallRule {
	var rules []HostFirewallRule
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "ufw ") {
			continue
		}
		rule, ok := parseUFWAddedRuleLine(line)
		if ok {
			rules = append(rules, rule)
		}
	}
	return rules
}

func parseUFWAddedRuleLine(line string) (HostFirewallRule, bool) {
	fields := shellLikeFields(line)
	if len(fields) < 3 || fields[0] != "ufw" {
		return HostFirewallRule{}, false
	}
	rule := HostFirewallRule{Raw: line, Action: normalizeHostFirewallAction(fields[1]), SourceCIDR: ""}
	if rule.Action == "" {
		return HostFirewallRule{}, false
	}
	commentIndex := indexOfString(fields, "comment")
	if commentIndex >= 0 && commentIndex+1 < len(fields) {
		rule.Comment = fields[commentIndex+1]
		fields = fields[:commentIndex]
	}
	if len(fields) >= 6 && fields[2] == "from" {
		rule.SourceCIDR = strings.TrimSpace(fields[3])
		portIndex := indexOfString(fields, "port")
		protoIndex := indexOfString(fields, "proto")
		if portIndex < 0 || portIndex+1 >= len(fields) {
			return HostFirewallRule{}, false
		}
		start, end, proto, ok := parseHostFirewallPortSpec(fields[portIndex+1])
		if !ok {
			return HostFirewallRule{}, false
		}
		if protoIndex >= 0 && protoIndex+1 < len(fields) {
			proto = normalizeHostFirewallProtocol(fields[protoIndex+1])
		}
		rule.PortStart, rule.PortEnd, rule.Protocol = start, end, proto
	} else {
		start, end, proto, ok := parseHostFirewallPortSpec(fields[2])
		if !ok {
			return HostFirewallRule{}, false
		}
		rule.PortStart, rule.PortEnd, rule.Protocol = start, end, proto
	}
	if rule.Protocol == "" {
		rule.Protocol = "both"
	}
	rule.ManagedByPanel = strings.HasPrefix(rule.Comment, hostFirewallPanelPrefix)
	rule.ID = hostFirewallRuleID(rule)
	return rule, true
}

func parseHostFirewallPortSpec(spec string) (int, int, string, bool) {
	spec = strings.TrimSpace(spec)
	proto := ""
	if strings.Contains(spec, "/") {
		parts := strings.SplitN(spec, "/", 2)
		spec = parts[0]
		proto = normalizeHostFirewallProtocol(parts[1])
	}
	start, end, ok := parseHostFirewallPortRange(spec)
	return start, end, proto, ok
}

func parseHostFirewallPortRange(text string) (int, int, bool) {
	text = strings.TrimSpace(strings.ReplaceAll(text, "-", ":"))
	if text == "" {
		return 0, 0, false
	}
	parts := strings.Split(text, ":")
	if len(parts) > 2 {
		return 0, 0, false
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	end := start
	if len(parts) == 2 {
		end, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, false
		}
	}
	if start < 1 || start > 65535 || end < start || end > 65535 {
		return 0, 0, false
	}
	return start, end, true
}

// ── Host firewall rule manipulation helpers ──

func normalizeHostFirewallRuleRequests(requests []HostFirewallRuleRequest) []HostFirewallRule {
	var rules []HostFirewallRule
	for _, req := range requests {
		action := normalizeHostFirewallAction(req.Action)
		proto := normalizeHostFirewallProtocol(req.Protocol)
		if action == "" {
			action = "allow"
		}
		if proto == "" {
			proto = "tcp"
		}
		start, end := req.PortStart, req.PortEnd
		if end == 0 {
			end = start
		}
		if start < 1 || start > 65535 || end < start || end > 65535 {
			continue
		}
		source := strings.TrimSpace(req.SourceCIDR)
		if source != "" {
			if _, err := netip.ParsePrefix(source); err != nil {
				if addr, addrErr := netip.ParseAddr(source); addrErr == nil {
					if addr.Is4() {
						source = addr.String() + "/32"
					} else {
						source = addr.String() + "/128"
					}
				} else {
					continue
				}
			}
		}
		base := HostFirewallRule{
			Action:     action,
			Protocol:   proto,
			PortStart:  start,
			PortEnd:    end,
			SourceCIDR: source,
			Comment:    strings.TrimSpace(req.Comment),
		}
		if strings.HasPrefix(base.Comment, hostFirewallPanelPrefix) {
			base.ManagedByPanel = true
		}
		if proto == "both" {
			tcpRule := base
			tcpRule.Protocol = "tcp"
			udpRule := base
			udpRule.Protocol = "udp"
			tcpRule.ID = hostFirewallRuleID(tcpRule)
			udpRule.ID = hostFirewallRuleID(udpRule)
			rules = append(rules, tcpRule, udpRule)
			continue
		}
		base.ID = hostFirewallRuleID(base)
		rules = append(rules, base)
	}
	return mergeHostFirewallRules(rules)
}

func buildProtectedHostFirewallRules(sshPorts, panelPorts []int) []HostFirewallRule {
	var rules []HostFirewallRule
	for _, port := range sshPorts {
		rules = append(rules, HostFirewallRule{
			Action:          "allow",
			Protocol:        "tcp",
			PortStart:       port,
			PortEnd:         port,
			Comment:         hostFirewallProtectedSSHPrefix,
			Protected:       true,
			ProtectedReason: "SSH 端口",
			ManagedByPanel:  true,
		})
	}
	for _, port := range panelPorts {
		rules = append(rules, HostFirewallRule{
			Action:          "allow",
			Protocol:        "tcp",
			PortStart:       port,
			PortEnd:         port,
			Comment:         hostFirewallProtectedPanelPrefix,
			Protected:       true,
			ProtectedReason: "面板服务端口",
			ManagedByPanel:  true,
		})
	}
	return mergeHostFirewallRules(rules)
}

func markHostFirewallProtection(rule *HostFirewallRule, sshPorts, panelPorts []int) {
	rule.ManagedByPanel = rule.ManagedByPanel || strings.HasPrefix(rule.Comment, hostFirewallPanelPrefix)
	if strings.HasPrefix(rule.Comment, hostFirewallProtectedSSHPrefix) {
		rule.Protected = true
		rule.ProtectedReason = "SSH 端口"
		return
	}
	if strings.HasPrefix(rule.Comment, hostFirewallProtectedPanelPrefix) {
		rule.Protected = true
		rule.ProtectedReason = "面板服务端口"
		return
	}
	if rule.Action != "allow" || rule.SourceCIDR != "" || rule.Protocol != "tcp" {
		return
	}
	for _, port := range sshPorts {
		if rule.PortStart == port && rule.PortEnd == port {
			rule.Protected = true
			rule.ProtectedReason = "SSH 端口"
			return
		}
	}
	for _, port := range panelPorts {
		if rule.PortStart == port && rule.PortEnd == port {
			rule.Protected = true
			rule.ProtectedReason = "面板服务端口"
			return
		}
	}
}

func ensureHostFirewallRule(rule HostFirewallRule) error {
	if err := validateHostFirewallRule(rule); err != nil {
		return err
	}
	existing, _ := ListHostFirewallRules()
	for _, item := range existing {
		if hostFirewallRuleEquivalent(item, rule) {
			return nil
		}
	}
	args := buildUFWRuleArgs(rule, false)
	result := utils.ExecCommand("ufw", args...)
	if result.Error != nil {
		return fmt.Errorf("写入 UFW 规则失败: %s", result.Stderr)
	}
	return nil
}

func deleteHostFirewallRuleBySpec(rule HostFirewallRule) error {
	args := buildUFWRuleArgs(rule, true)
	result := utils.ExecCommand("ufw", args...)
	if result.Error != nil {
		return fmt.Errorf("删除 UFW 规则失败: %s", result.Stderr)
	}
	return nil
}

func buildUFWRuleArgs(rule HostFirewallRule, delete bool) []string {
	portSpec := hostFirewallPortSpec(rule)
	args := []string{}
	if delete {
		args = append(args, "delete")
	}
	args = append(args, rule.Action)
	if strings.TrimSpace(rule.SourceCIDR) != "" {
		args = append(args, "from", strings.TrimSpace(rule.SourceCIDR), "to", "any", "port", portSpec, "proto", rule.Protocol)
	} else {
		args = append(args, portSpec+"/"+rule.Protocol)
	}
	if !delete && strings.TrimSpace(rule.Comment) != "" {
		args = append(args, "comment", strings.TrimSpace(rule.Comment))
	}
	return args
}

func hostFirewallPortSpec(rule HostFirewallRule) string {
	if rule.PortStart == rule.PortEnd {
		return strconv.Itoa(rule.PortStart)
	}
	return fmt.Sprintf("%d:%d", rule.PortStart, rule.PortEnd)
}

func validateHostFirewallRule(rule HostFirewallRule) error {
	if normalizeHostFirewallAction(rule.Action) == "" {
		return fmt.Errorf("规则动作只支持 allow 或 deny")
	}
	if rule.Protocol != "tcp" && rule.Protocol != "udp" {
		return fmt.Errorf("协议只支持 tcp 或 udp")
	}
	if rule.PortStart < 1 || rule.PortStart > 65535 || rule.PortEnd < rule.PortStart || rule.PortEnd > 65535 {
		return fmt.Errorf("端口范围无效")
	}
	if strings.TrimSpace(rule.SourceCIDR) != "" {
		if _, err := netip.ParsePrefix(strings.TrimSpace(rule.SourceCIDR)); err != nil {
			return fmt.Errorf("来源 CIDR 无效")
		}
	}
	return nil
}

func hostFirewallRuleEquivalent(a, b HostFirewallRule) bool {
	return a.Action == b.Action &&
		a.Protocol == b.Protocol &&
		a.PortStart == b.PortStart &&
		a.PortEnd == b.PortEnd &&
		strings.TrimSpace(a.SourceCIDR) == strings.TrimSpace(b.SourceCIDR)
}

func hostFirewallRuleID(rule HostFirewallRule) string {
	base := fmt.Sprintf("%s|%s|%d|%d|%s|%s", rule.Action, rule.Protocol, rule.PortStart, rule.PortEnd, strings.TrimSpace(rule.SourceCIDR), strings.TrimSpace(rule.Comment))
	sum := sha1.Sum([]byte(base))
	return hex.EncodeToString(sum[:])[:16]
}

func mergeHostFirewallRules(groups ...[]HostFirewallRule) []HostFirewallRule {
	seen := map[string]HostFirewallRule{}
	for _, group := range groups {
		for _, rule := range group {
			if rule.ID == "" {
				rule.ID = hostFirewallRuleID(rule)
			}
			key := fmt.Sprintf("%s|%s|%d|%d|%s", rule.Action, rule.Protocol, rule.PortStart, rule.PortEnd, strings.TrimSpace(rule.SourceCIDR))
			if old, ok := seen[key]; ok {
				if old.Protected {
					continue
				}
				if rule.Comment == "" {
					rule.Comment = old.Comment
				}
			}
			seen[key] = rule
		}
	}
	result := make([]HostFirewallRule, 0, len(seen))
	for _, rule := range seen {
		rule.ID = hostFirewallRuleID(rule)
		result = append(result, rule)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Protected != result[j].Protected {
			return result[i].Protected
		}
		if result[i].PortStart != result[j].PortStart {
			return result[i].PortStart < result[j].PortStart
		}
		return result[i].Protocol < result[j].Protocol
	})
	return result
}

func normalizeHostFirewallAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "allow", "deny":
		return strings.ToLower(strings.TrimSpace(action))
	default:
		return ""
	}
}

func normalizeHostFirewallProtocol(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "", "tcp":
		return "tcp"
	case "udp":
		return "udp"
	case "both", "any":
		return "both"
	default:
		return ""
	}
}

func shellLikeFields(line string) []string {
	var fields []string
	var b strings.Builder
	inSingle := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case ch == '\'':
			inSingle = !inSingle
		case !inSingle && (ch == ' ' || ch == '\t'):
			if b.Len() > 0 {
				fields = append(fields, b.String())
				b.Reset()
			}
		default:
			b.WriteByte(ch)
		}
	}
	if b.Len() > 0 {
		fields = append(fields, b.String())
	}
	return fields
}

func indexOfString(values []string, target string) int {
	for i, value := range values {
		if value == target {
			return i
		}
	}
	return -1
}
