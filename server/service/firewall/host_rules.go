package firewall

import (
	"fmt"
	"strconv"
	"strings"
)

func AddHostFirewallRule(req HostFirewallRuleRequest) (*HostFirewallRule, error) {
	rules := normalizeHostFirewallRuleRequests([]HostFirewallRuleRequest{req})
	if len(rules) == 0 {
		return nil, fmt.Errorf("规则参数无效")
	}
	for _, rule := range rules {
		if err := ensureHostFirewallRule(rule); err != nil {
			return nil, err
		}
	}
	return &rules[0], nil
}

func UpdateHostFirewallRule(id string, req HostFirewallRuleRequest) (*HostFirewallRule, error) {
	current, err := FindHostFirewallRule(id)
	if err != nil {
		return nil, err
	}
	if current.Protected {
		return nil, fmt.Errorf("SSH 和面板服务端口规则不允许编辑")
	}
	nextRules := normalizeHostFirewallRuleRequests([]HostFirewallRuleRequest{req})
	if len(nextRules) == 0 {
		return nil, fmt.Errorf("规则参数无效")
	}
	for _, rule := range nextRules {
		if err := ensureHostFirewallRule(rule); err != nil {
			return nil, err
		}
	}
	if err := deleteHostFirewallRuleBySpec(current); err != nil {
		return nil, err
	}
	return &nextRules[0], nil
}

func DeleteHostFirewallRule(id string) error {
	rule, err := FindHostFirewallRule(id)
	if err != nil {
		return err
	}
	if rule.Protected {
		return fmt.Errorf("SSH 和面板服务端口规则不允许删除")
	}
	return deleteHostFirewallRuleBySpec(rule)
}

func AddHostFirewallVNCDefaultRule() (*HostFirewallRule, error) {
	rule := HostFirewallRule{
		Action:         "allow",
		Protocol:       "tcp",
		PortStart:      5900,
		PortEnd:        5999,
		Comment:        hostFirewallVNCComment,
		ManagedByPanel: true,
	}
	if err := ensureHostFirewallRule(rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

func FindHostFirewallRule(id string) (HostFirewallRule, error) {
	rules, err := ListHostFirewallRules()
	if err != nil {
		return HostFirewallRule{}, err
	}
	for _, rule := range rules {
		if rule.ID == id {
			return rule, nil
		}
	}
	return HostFirewallRule{}, fmt.Errorf("未找到防火墙规则")
}

func BuildHostFirewallRecommendedRules() []HostFirewallRule {
	recommended := buildProtectedHostFirewallRules(DetectSSHPorts(), DetectPanelPorts())
	for _, pf := range currentPortForwardRulesForPolicy(nil) {
		proto := strings.ToLower(strings.TrimSpace(pf.Protocol))
		if proto != "tcp" && proto != "udp" {
			continue
		}
		port, err := strconv.Atoi(strings.TrimSpace(pf.HostPort))
		if err != nil || port <= 0 {
			continue
		}
		recommended = append(recommended, HostFirewallRule{
			Action:         "allow",
			Protocol:       proto,
			PortStart:      port,
			PortEnd:        port,
			Comment:        hostFirewallPortForwardPrefix,
			ManagedByPanel: true,
		})
	}
	return mergeHostFirewallRules(recommended)
}
