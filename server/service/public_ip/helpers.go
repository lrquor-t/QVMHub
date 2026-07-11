package public_ip

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"qvmhub/model"
	"qvmhub/utils"
)

func NormalizePublicIPMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "1:1 nat", "nat":
		return PublicIPModeNAT
	case "classic", "classic_route", "经典网络-路由":
		return PublicIPModeClassicRoute
	case "classic_bridge", "经典网络-桥接":
		return PublicIPModeClassicBridge
	default:
		return strings.ToLower(strings.TrimSpace(mode))
	}
}

func PublicIPModeLabel(mode string) string {
	switch NormalizePublicIPMode(mode) {
	case PublicIPModeNAT:
		return "1:1 NAT"
	case PublicIPModeClassicRoute:
		return "经典网络-路由"
	case PublicIPModeClassicBridge:
		return "经典网络-桥接"
	default:
		return mode
	}
}

func ParsePublicIPOperationParams(raw string) (PublicIPOperationParams, error) {
	var params PublicIPOperationParams
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		return params, err
	}
	return params, nil
}

func ParsePublicIPID(raw string) (uint, error) {
	id, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("公网 IP ID 无效")
	}
	return uint(id), nil
}

func normalizePublicIPRequest(req PublicIPRequest, current *model.PublicIP) (*model.PublicIP, error) {
	ipText := strings.TrimSpace(req.IP)
	if current != nil && ipText == "" {
		ipText = current.IP
	}
	if net.ParseIP(ipText) == nil {
		return nil, fmt.Errorf("公网 IP 格式无效")
	}
	if strings.Contains(ipText, ":") {
		return nil, fmt.Errorf("当前仅支持 IPv4 公网 IP")
	}
	cidr := strings.TrimSpace(req.CIDR)
	if cidr != "" {
		if err := validatePublicIPCidr(ipText, cidr); err != nil {
			return nil, err
		}
	}
	gateway := strings.TrimSpace(req.Gateway)
	if gateway != "" && net.ParseIP(gateway) == nil {
		return nil, fmt.Errorf("网关 IP 格式无效")
	}
	modes := normalizeSupportedPublicIPModes(req.SupportedModes)
	status := strings.TrimSpace(req.Status)
	if status == "" {
		if current != nil {
			status = current.Status
		} else {
			status = PublicIPStatusFree
		}
	}
	if status != PublicIPStatusFree && status != PublicIPStatusBound && status != "reserved" {
		return nil, fmt.Errorf("公网 IP 状态无效")
	}
	return &model.PublicIP{
		IP:             ipText,
		CIDR:           cidr,
		Gateway:        gateway,
		UplinkIF:       strings.TrimSpace(req.UplinkIF),
		SupportedModes: modes,
		Status:         status,
		Remark:         strings.TrimSpace(req.Remark),
	}, nil
}

func validatePublicIPCidr(ipText, cidr string) error {
	if strings.Contains(cidr, "/") {
		ip, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("CIDR/掩码格式无效")
		}
		publicIP := net.ParseIP(ipText)
		if publicIP == nil || !network.Contains(publicIP) {
			return fmt.Errorf("公网 IP 不在填写的 CIDR 范围内")
		}
		if ip.String() == ipText {
			return nil
		}
		return nil
	}
	if net.ParseIP(cidr) == nil {
		return fmt.Errorf("CIDR/掩码格式无效")
	}
	return nil
}

func publicIPPrefix(ipRow model.PublicIP) int {
	if strings.Contains(ipRow.CIDR, "/") {
		_, network, err := net.ParseCIDR(ipRow.CIDR)
		if err == nil {
			ones, _ := network.Mask.Size()
			if ones > 0 {
				return ones
			}
		}
	}
	if maskIP := net.ParseIP(strings.TrimSpace(ipRow.CIDR)); maskIP != nil {
		mask := net.IPMask(maskIP.To4())
		if len(mask) == net.IPv4len {
			if ones, bits := mask.Size(); bits == 32 && ones >= 0 {
				return ones
			}
		}
	}
	return 32
}

func publicIPAddrForHost(ipRow model.PublicIP) string {
	prefix := publicIPPrefix(ipRow)
	if prefix <= 0 || prefix > 32 {
		prefix = 32
	}
	return fmt.Sprintf("%s/%d", strings.TrimSpace(ipRow.IP), prefix)
}

func publicIPVMInterface(vmName string) (string, string) {
	for _, iface := range HookParseVirshDomiflistOutput(utils.ExecCommand("virsh", "domiflist", strings.TrimSpace(vmName)).Stdout) {
		if strings.TrimSpace(iface.Name) != "" && iface.Name != "-" && strings.TrimSpace(iface.Source) != "" {
			return iface.Name, iface.Source
		}
	}
	return "", ""
}

func publicIPVMBridge(vmName string) string {
	_, bridge := publicIPVMInterface(vmName)
	return bridge
}

func publicIPFlowCookie(ipText string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.TrimSpace(ipText)))
	value := h.Sum64() & 0x00ffffffffffffff
	return publicIPFlowPrefix + fmt.Sprintf("%014x", value)
}

func copyFile(src, dst string, perm os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, perm)
}

func publicIPManagedBridges() []string {
	seen := map[string]bool{HookOvsBridgeName(): true}
	bridges := []string{HookOvsBridgeName()}
	if model.DB != nil {
		var rows []model.NetworkBridge
		model.DB.Find(&rows)
		for _, row := range rows {
			name := strings.TrimSpace(row.Name)
			if name != "" && !seen[name] {
				seen[name] = true
				bridges = append(bridges, name)
			}
		}
	}
	sort.Strings(bridges)
	return bridges
}

func publicIPRuntimeRuleSummary(ipRow model.PublicIP, binding model.PublicIPBinding) []string {
	req := PublicIPBindRequest{
		Username:    binding.Username,
		VMName:      binding.VMName,
		VMPrivateIP: binding.VMPrivateIP,
		Mode:        binding.Mode,
	}
	commands, err := buildPublicIPCommands(ipRow, req)
	if err != nil {
		return []string{err.Error()}
	}
	return commands
}

func parsePublicIPModes(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = PublicIPModeNAT + "," + PublicIPModeClassicRoute + "," + PublicIPModeClassicBridge
	}
	seen := map[string]bool{}
	var modes []string
	for _, part := range strings.Split(raw, ",") {
		mode := NormalizePublicIPMode(part)
		if mode == "" || seen[mode] {
			continue
		}
		seen[mode] = true
		modes = append(modes, mode)
	}
	return modes
}

func publicIPModeLabels(modes []string) []string {
	labels := make([]string, 0, len(modes))
	for _, mode := range modes {
		labels = append(labels, PublicIPModeLabel(mode))
	}
	return labels
}

func normalizeSupportedPublicIPModes(raw string) string {
	modes := parsePublicIPModes(raw)
	if len(modes) == 0 {
		modes = []string{PublicIPModeNAT}
	}
	return strings.Join(modes, ",")
}

func publicIPModeAllowed(ipRow model.PublicIP, mode string) bool {
	mode = NormalizePublicIPMode(mode)
	for _, item := range parsePublicIPModes(ipRow.SupportedModes) {
		if item == mode {
			return true
		}
	}
	return false
}

func getPublicIP(id uint) (*model.PublicIP, error) {
	var row model.PublicIP
	if err := model.DB.First(&row, id).Error; err != nil {
		return nil, fmt.Errorf("公网 IP 不存在")
	}
	return &row, nil
}
