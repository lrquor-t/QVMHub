package firewall

import (
	"fmt"
	"strconv"
	"strings"

	"qvmhub/utils"
)

func IsHostFirewallActive() bool {
	result := utils.ExecCommand("ufw", "status")
	return result.Error == nil && strings.Contains(strings.ToLower(result.Stdout), "status: active")
}

func EnsureHostFirewallPortForwardRule(hostPort, protocol, comment string) error {
	port, err := strconv.Atoi(strings.TrimSpace(hostPort))
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("宿主机端口格式无效")
	}
	proto := normalizeHostFirewallProtocol(protocol)
	if proto == "both" {
		proto = "tcp"
	}
	if proto != "tcp" && proto != "udp" {
		return fmt.Errorf("协议只支持 tcp 或 udp")
	}
	rule := HostFirewallRule{
		Action:         "allow",
		Protocol:       proto,
		PortStart:      port,
		PortEnd:        port,
		Comment:        strings.TrimSpace(hostFirewallPortForwardPrefix + ":" + comment),
		ManagedByPanel: true,
	}
	return ensureHostFirewallRule(rule)
}

func DeleteHostFirewallPortForwardRule(hostPort, protocol string) error {
	port, err := strconv.Atoi(strings.TrimSpace(hostPort))
	if err != nil || port <= 0 || port > 65535 {
		return nil
	}
	proto := normalizeHostFirewallProtocol(protocol)
	if proto != "tcp" && proto != "udp" {
		return nil
	}
	rules, err := ListHostFirewallRules()
	if err != nil {
		return nil
	}
	for _, rule := range rules {
		if rule.PortStart == port && rule.PortEnd == port && rule.Protocol == proto && strings.HasPrefix(rule.Comment, hostFirewallPortForwardPrefix) {
			_ = deleteHostFirewallRuleBySpec(rule)
		}
	}
	return nil
}
