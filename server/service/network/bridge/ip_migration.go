package bridge

import (
	"fmt"
	"strings"

	"qvmhub/utils"
)

// HostIPConfig 存储从接口捕获的 IP 配置，用于持久化到数据库和恢复脚本。
type HostIPConfig struct {
	Addrs   string // 换行分隔的 CIDR 地址列表
	Gateway string // 默认网关 IP
	Metric  string // 路由 metric
	DNS     string // 空格分隔的 DNS 服务器 IP 列表
}

// CaptureInterfaceIPv4 从指定接口捕获当前 IPv4 配置（地址、网关、metric、DNS）。
func CaptureInterfaceIPv4(iface string) HostIPConfig {
	var cfg HostIPConfig
	result := utils.ExecCommand("bash", "-c", fmt.Sprintf(
		`ip -4 -o addr show dev %s scope global 2>/dev/null | awk '{print $4}'`,
		utils.ShellSingleQuote(iface)))
	cfg.Addrs = strings.TrimSpace(result.Stdout)

	result = utils.ExecCommand("bash", "-c", fmt.Sprintf(
		`ip -4 route show default dev %s 2>/dev/null | awk '{print $3; exit}'`,
		utils.ShellSingleQuote(iface)))
	cfg.Gateway = strings.TrimSpace(result.Stdout)

	result = utils.ExecCommand("bash", "-c", fmt.Sprintf(
		`ip -4 route show default dev %s 2>/dev/null | awk '{for (i=1;i<=NF;i++) if ($i=="metric") {print $(i+1); exit}}'`,
		utils.ShellSingleQuote(iface)))
	cfg.Metric = strings.TrimSpace(result.Stdout)
	cfg.DNS = captureInterfaceDNSServers(iface)
	return cfg
}

// captureInterfaceDNSServers 从 resolvectl 捕获指定接口的 DNS 服务器，返回空格分隔的 IP 列表。
// 优先从指定接口捕获，回退到全局 DNS。
// 使用 Go 原生 IP 解析（resolvectlDNSServers），避免 shell sed 正则被 IPv6 地址中的冒号干扰。
func captureInterfaceDNSServers(iface string) string {
	// 先从指定接口获取
	servers := resolvectlDNSServers(iface)
	if len(servers) > 0 {
		return strings.Join(servers, " ")
	}
	// 回退到全局
	servers = resolvectlDNSServers("")
	if len(servers) > 0 {
		return strings.Join(servers, " ")
	}
	return ""
}

func migrateInterfaceIPv4ToBridge(uplink, bridge string) {
	script := fmt.Sprintf(`set -e
UPLINK=%s
BRIDGE=%s
%s
`, utils.ShellSingleQuote(uplink), utils.ShellSingleQuote(bridge), bridgeHostIPMigrationShell())
	utils.ExecCommand("bash", "-c", script)
}

// applyStaticIPv4ToBridge 使用静态存储的 IP 配置应用到网桥（用于重启后恢复）。
func applyStaticIPv4ToBridge(bridge string, cfg HostIPConfig) {
	if strings.TrimSpace(cfg.Addrs) == "" {
		return
	}
	script := fmt.Sprintf(`set -e
BRIDGE=%s
HOST_ADDRS=%s
HOST_GW=%s
HOST_METRIC=%s
%s
`, utils.ShellSingleQuote(bridge),
		utils.ShellSingleQuote(cfg.Addrs),
		utils.ShellSingleQuote(cfg.Gateway),
		utils.ShellSingleQuote(cfg.Metric),
		bridgeHostIPApplyStaticShell())
	utils.ExecCommand("bash", "-c", script)
}

// bridgeHostIPApplyStaticShell 生成使用静态变量应用 IP 的 shell 代码。
func bridgeHostIPApplyStaticShell() string {
	return `if [ -n "$HOST_ADDRS" ]; then
  while IFS= read -r addr; do
    [ -n "$addr" ] || continue
    ip addr replace "$addr" dev "$BRIDGE"
  done <<< "$HOST_ADDRS"
fi
if [ -n "$HOST_GW" ]; then
  ip route replace "$HOST_GW" dev "$BRIDGE" scope link 2>/dev/null || true
  if [ -n "$HOST_METRIC" ]; then
    ip route replace default via "$HOST_GW" dev "$BRIDGE" metric "$HOST_METRIC"
  else
    ip route replace default via "$HOST_GW" dev "$BRIDGE"
  fi
fi
`
}

func migrateBridgeIPv4ToInterface(bridge, uplink string) {
	script := fmt.Sprintf(`set -e
BRIDGE=%s
UPLINK=%s
%s
`, utils.ShellSingleQuote(bridge), utils.ShellSingleQuote(uplink), bridgeHostIPRollbackShell())
	utils.ExecCommand("bash", "-c", script)
}

func bridgeHostIPMigrationShell() string {
	return bridgeHostIPCaptureShell() + bridgeHostIPApplyShell()
}

func bridgeHostIPRollbackShell() string {
	return bridgeHostIPCaptureFromBridgeShell() + bridgeHostIPApplyToUplinkShell()
}

func bridgeHostIPCaptureShell() string {
	return `HOST_ADDRS="$(ip -4 -o addr show dev "$UPLINK" scope global 2>/dev/null | awk '{print $4}')"
HOST_GW="$(ip -4 route show default dev "$UPLINK" 2>/dev/null | awk '{print $3; exit}')"
HOST_METRIC="$(ip -4 route show default dev "$UPLINK" 2>/dev/null | awk '{for (i=1;i<=NF;i++) if ($i=="metric") {print $(i+1); exit}}')"
`
}

func bridgeHostIPCaptureFromBridgeShell() string {
	return `HOST_ADDRS="$(ip -4 -o addr show dev "$BRIDGE" scope global 2>/dev/null | awk '{print $4}')"
HOST_GW="$(ip -4 route show default dev "$BRIDGE" 2>/dev/null | awk '{print $3; exit}')"
HOST_METRIC="$(ip -4 route show default dev "$BRIDGE" 2>/dev/null | awk '{for (i=1;i<=NF;i++) if ($i=="metric") {print $(i+1); exit}}')"
`
}

func bridgeHostIPApplyShell() string {
	return `if [ -n "$HOST_ADDRS" ]; then
  ip addr flush dev "$UPLINK"
  while IFS= read -r addr; do
    [ -n "$addr" ] || continue
    ip addr replace "$addr" dev "$BRIDGE"
  done <<< "$HOST_ADDRS"
fi
if [ -n "$HOST_GW" ]; then
  ip route del "$HOST_GW" dev "$UPLINK" 2>/dev/null || true
  ip route replace "$HOST_GW" dev "$BRIDGE" scope link
  if [ -n "$HOST_METRIC" ]; then
    ip route replace default via "$HOST_GW" dev "$BRIDGE" metric "$HOST_METRIC"
  else
    ip route replace default via "$HOST_GW" dev "$BRIDGE"
  fi
fi
`
}

func bridgeHostIPApplyToUplinkShell() string {
	return `ip link set "$UPLINK" up
if [ -n "$HOST_ADDRS" ]; then
  ip addr flush dev "$BRIDGE"
  while IFS= read -r addr; do
    [ -n "$addr" ] || continue
    ip addr replace "$addr" dev "$UPLINK"
  done <<< "$HOST_ADDRS"
fi
if [ -n "$HOST_GW" ]; then
  ip route del "$HOST_GW" dev "$BRIDGE" 2>/dev/null || true
  ip route replace "$HOST_GW" dev "$UPLINK" scope link
  if [ -n "$HOST_METRIC" ]; then
    ip route replace default via "$HOST_GW" dev "$UPLINK" metric "$HOST_METRIC"
  else
    ip route replace default via "$HOST_GW" dev "$UPLINK"
  fi
fi
`
}
