package diagnostics

import (
	"fmt"
	"strconv"
	"strings"
)

// BuildNetworkDiagnosticTemplates 构建网络诊断模板
func BuildNetworkDiagnosticTemplates(defaultIP string, forwards []PortForwardRule) []NetworkDiagnosticTemplate {
	templates := []NetworkDiagnosticTemplate{
		{Key: "arp", Name: "ARP", Description: "检查 ARP 请求与应答", Filter: NetworkDiagnosticFilter{Protocol: "arp"}},
		{Key: "dhcp", Name: "DHCP", Description: "检查 DHCP 获取地址过程", Filter: NetworkDiagnosticFilter{Protocol: "dhcp"}},
		{Key: "dns", Name: "DNS", Description: "检查 DNS 查询与响应", Filter: NetworkDiagnosticFilter{Protocol: "dns"}},
	}
	if defaultIP != "" {
		templates = append(templates, NetworkDiagnosticTemplate{
			Key:         "vm_ip",
			Name:        "当前 VM IP",
			Description: "只查看当前 VM IP 的流量",
			Filter:      NetworkDiagnosticFilter{SourceIP: defaultIP},
		})
	}
	for _, rule := range forwards {
		port, _ := strconv.Atoi(rule.DestPort)
		if port <= 0 {
			continue
		}
		protocol := strings.ToLower(rule.Protocol)
		if protocol != "tcp" && protocol != "udp" {
			protocol = "any"
		}
		templates = append(templates, NetworkDiagnosticTemplate{
			Key:         "pf_" + rule.StableKey(),
			Name:        fmt.Sprintf("端口转发 %s/%s", rule.DestPort, strings.ToUpper(rule.Protocol)),
			Description: fmt.Sprintf("检查宿主机端口 %s 到 VM %s:%s 的入站流量", rule.HostPort, rule.DestIP, rule.DestPort),
			Filter:      NetworkDiagnosticFilter{Protocol: protocol, DestIP: rule.DestIP, DestPort: port},
		})
	}
	return templates
}
