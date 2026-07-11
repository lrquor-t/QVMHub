package firewall

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"qvmhub/utils"
)

func PreviewHostFirewallConnections(mode string) (*HostFirewallConnectionPreview, error) {
	mode = normalizeHostFirewallConnectionMode(mode)
	connections := listHostTCPConnections()
	allowedPorts := hostFirewallAllowedPorts()
	var targets []HostFirewallConnection
	for _, conn := range connections {
		conn.AllowedPort = allowedPorts[conn.LocalPort]
		if mode == "all" || !conn.AllowedPort {
			targets = append(targets, conn)
		}
	}
	warning := "将关闭当前筛选出的 TCP 已建立连接。"
	if mode == "all" {
		warning = "将尝试关闭所有 TCP 已建立连接，包括 SSH 和面板连接，当前会话可能立即断开。"
	}
	return &HostFirewallConnectionPreview{
		Mode:        mode,
		Connections: targets,
		Count:       len(targets),
		Warning:     warning,
	}, nil
}

func CloseHostFirewallConnections(mode string) (int, error) {
	preview, err := PreviewHostFirewallConnections(mode)
	if err != nil {
		return 0, err
	}
	if preview.Mode == "all" {
		result := utils.ExecShellWithTimeout("ss -K -t state established 2>/dev/null || true", 15*time.Second)
		if result.Error != nil {
			return 0, fmt.Errorf("关闭全部连接失败: %s", result.Stderr)
		}
		return preview.Count, nil
	}
	for _, conn := range preview.Connections {
		cmd := fmt.Sprintf("ss -K -t state established sport = :%d dport = :%d dst %s 2>/dev/null || true",
			conn.LocalPort, conn.PeerPort, utils.ShellSingleQuote(conn.PeerIP))
		utils.ExecShellWithTimeout(cmd, 5*time.Second)
	}
	return preview.Count, nil
}

func hostFirewallAllowedPorts() map[int]bool {
	rules, err := ListHostFirewallRules()
	if err != nil {
		return map[int]bool{}
	}
	allowed := map[int]bool{}
	for _, rule := range rules {
		if rule.Action != "allow" {
			continue
		}
		for port := rule.PortStart; port <= rule.PortEnd; port++ {
			allowed[port] = true
		}
	}
	return allowed
}

func listHostTCPConnections() []HostFirewallConnection {
	result := utils.ExecShell(`ss -Htn state established 2>/dev/null`)
	var connections []HostFirewallConnection
	for _, line := range strings.Split(result.Stdout, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 4 {
			continue
		}
		localIP, localPort, ok := splitAddressPort(fields[2])
		if !ok {
			continue
		}
		peerIP, peerPort, ok := splitAddressPort(fields[3])
		if !ok {
			continue
		}
		connections = append(connections, HostFirewallConnection{
			Protocol:  "tcp",
			LocalIP:   localIP,
			LocalPort: localPort,
			PeerIP:    peerIP,
			PeerPort:  peerPort,
		})
	}
	return connections
}

func splitAddressPort(value string) (string, int, bool) {
	value = strings.Trim(value, "[]")
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		idx := strings.LastIndex(value, ":")
		if idx < 0 {
			return "", 0, false
		}
		host = strings.Trim(value[:idx], "[]")
		portText = value[idx+1:]
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return "", 0, false
	}
	return host, port, true
}

func normalizeHostFirewallConnectionMode(mode string) string {
	if strings.TrimSpace(mode) == "all" {
		return "all"
	}
	return "non_firewall"
}
