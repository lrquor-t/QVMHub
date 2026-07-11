package diagnostics

import (
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

var captureInterfaceRe = regexp.MustCompile(`^[A-Za-z0-9_.:-]+$`)

// ParseNetworkCaptureParams 解析抓包参数 JSON
func ParseNetworkCaptureParams(raw string) (NetworkCaptureParams, error) {
	var params NetworkCaptureParams
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		return params, err
	}
	params.VMName = strings.TrimSpace(params.VMName)
	if params.VMName == "" {
		return params, fmt.Errorf("虚拟机名称不能为空")
	}
	return params, nil
}

// BuildNetworkCaptureBPF 构建抓包 BPF 过滤表达式
func BuildNetworkCaptureBPF(filter NetworkDiagnosticFilter) (string, error) {
	var parts []string
	protocol := strings.ToLower(strings.TrimSpace(filter.Protocol))
	switch protocol {
	case "", "any", "all":
	case "tcp", "udp", "icmp", "arp":
		parts = append(parts, protocol)
	case "dhcp":
		parts = append(parts, "(udp and (port 67 or port 68))")
	case "dns":
		parts = append(parts, "port 53")
	default:
		return "", fmt.Errorf("不支持的协议过滤条件")
	}
	if (protocol == "arp" || protocol == "icmp") && (filter.Port > 0 || filter.SourcePort > 0 || filter.DestPort > 0) {
		return "", fmt.Errorf("%s 协议不能同时指定端口过滤", strings.ToUpper(protocol))
	}
	if src := strings.TrimSpace(filter.SourceIP); src != "" {
		if net.ParseIP(src) == nil {
			return "", fmt.Errorf("源 IP 格式不正确")
		}
		parts = append(parts, "src host "+src)
	}
	if dst := strings.TrimSpace(filter.DestIP); dst != "" {
		if net.ParseIP(dst) == nil {
			return "", fmt.Errorf("目标 IP 格式不正确")
		}
		parts = append(parts, "dst host "+dst)
	}
	if filter.Port > 0 {
		if !validPort(filter.Port) {
			return "", fmt.Errorf("端口范围必须为 1-65535")
		}
		parts = append(parts, "port "+strconv.Itoa(filter.Port))
	}
	if filter.SourcePort > 0 {
		if !validPort(filter.SourcePort) {
			return "", fmt.Errorf("源端口范围必须为 1-65535")
		}
		parts = append(parts, "src port "+strconv.Itoa(filter.SourcePort))
	}
	if filter.DestPort > 0 {
		if !validPort(filter.DestPort) {
			return "", fmt.Errorf("目标端口范围必须为 1-65535")
		}
		parts = append(parts, "dst port "+strconv.Itoa(filter.DestPort))
	}
	return strings.Join(parts, " and "), nil
}

// NormalizeNetworkCaptureRequest 校验并规范化抓包请求
func NormalizeNetworkCaptureRequest(vmName string, req NetworkCaptureRequest) (NetworkCaptureRequest, string, string, error) {
	if HookGetVMNetworkRuntimeStatus == nil {
		return req, "", "", fmt.Errorf("HookGetVMNetworkRuntimeStatus 未注入")
	}
	status, err := HookGetVMNetworkRuntimeStatus(vmName)
	if err != nil {
		return req, "", "", err
	}
	if strings.TrimSpace(status.State) != "running" {
		return req, "", "", fmt.Errorf("虚拟机未运行，无法对运行态接口抓包")
	}
	selected := strings.TrimSpace(req.InterfaceName)
	var matched *VMNetworkInterface
	for i := range status.Interfaces {
		if !isUsableCaptureInterface(status.Interfaces[i]) {
			continue
		}
		if selected == "" || status.Interfaces[i].Target == selected {
			matched = &status.Interfaces[i]
			selected = status.Interfaces[i].Target
			break
		}
	}
	if matched == nil {
		return req, "", "", fmt.Errorf("未找到可抓包的 VM 运行态接口")
	}
	if !captureInterfaceRe.MatchString(selected) {
		return req, "", "", fmt.Errorf("接口名称不合法")
	}
	req.DurationSeconds = clampInt(req.DurationSeconds, captureDefaultSeconds(), captureMaxSeconds())
	req.MaxMB = clampInt(req.MaxMB, captureMaxMB(), captureMaxMB())
	req.MaxPackets = clampInt(req.MaxPackets, captureMaxPackets(), captureMaxPackets())
	bpf, err := BuildNetworkCaptureBPF(req.Filter)
	if err != nil {
		return req, "", "", err
	}
	return req, selected, bpf, nil
}
