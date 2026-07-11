package firewall

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/utils"
)

func defaultFirewallPolicy() *FirewallPolicy {
	subnet := "192.168.122"
	if config.GlobalConfig != nil {
		subnet = strings.TrimSpace(config.GlobalConfig.SubnetPrefix)
	}
	if subnet == "" {
		subnet = "192.168.122"
	}
	return &FirewallPolicy{
		Enabled:                false,
		Bridge:                 HookOvsBridgeName(),
		VMSubnet:               subnet + ".0/24",
		OutboundEnabled:        false,
		InboundEnabled:         false,
		DisableVMIPv6:          true,
		BlockAction:            "reject",
		OutboundAllowedRegions: []string{},
		InboundAllowedRegions:  []string{},
		WhitelistCIDRs:         []string{},
		Regions:                []FirewallRegion{},
		VMOverrides:            map[string]FirewallVMOverride{},
		PortForwardExemptions:  map[string]bool{},
		GeoIPBaseURL:           defaultGeoBaseURL,
	}
}

func ensureFirewallDir() error {
	if err := os.MkdirAll(filepath.Join(firewallDir, "backups"), 0755); err != nil {
		return fmt.Errorf("创建防火墙目录失败: %w", err)
	}
	// 测试目录可写性
	testFile := filepath.Join(firewallDir, ".writetest")
	if err := os.WriteFile(testFile, []byte(""), 0600); err != nil {
		return fmt.Errorf("防火墙目录不可写: %w", err)
	}
	os.Remove(testFile)
	return nil
}

func normalizeFirewallPolicy(policy *FirewallPolicy) *FirewallPolicy {
	if policy == nil {
		policy = defaultFirewallPolicy()
	}
	def := defaultFirewallPolicy()
	if strings.TrimSpace(policy.Bridge) == "" {
		policy.Bridge = def.Bridge
	}
	if HookUseOVSNetwork() && strings.TrimSpace(policy.Bridge) == "virbr0" {
		policy.Bridge = HookOvsBridgeName()
	}
	if strings.TrimSpace(policy.VMSubnet) == "" {
		policy.VMSubnet = def.VMSubnet
	}
	if strings.TrimSpace(policy.BlockAction) == "" {
		policy.BlockAction = def.BlockAction
	}
	if strings.TrimSpace(policy.GeoIPBaseURL) == "" {
		policy.GeoIPBaseURL = def.GeoIPBaseURL
	}
	if policy.VMOverrides == nil {
		policy.VMOverrides = map[string]FirewallVMOverride{}
	}
	if policy.PortForwardExemptions == nil {
		policy.PortForwardExemptions = map[string]bool{}
	}
	if policy.Regions == nil {
		policy.Regions = []FirewallRegion{}
	}
	policy.BlockAction = normalizeBlockAction(policy.BlockAction)
	policy.OutboundAllowedRegions = normalizeStringList(policy.OutboundAllowedRegions)
	policy.InboundAllowedRegions = normalizeStringList(policy.InboundAllowedRegions)
	policy.WhitelistCIDRs = normalizeCIDRList(policy.WhitelistCIDRs)
	for i := range policy.Regions {
		policy.Regions[i].Code = normalizeRegionCode(policy.Regions[i].Code)
		policy.Regions[i].CIDRs = normalizeCIDRList(policy.Regions[i].CIDRs)
	}
	return policy
}

// GetFirewallPolicy 读取策略；文件不存在时返回默认策略。
func GetFirewallPolicy() (*FirewallPolicy, error) {
	data, err := os.ReadFile(firewallPolicyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultFirewallPolicy(), nil
		}
		return nil, fmt.Errorf("读取防火墙策略失败: %w", err)
	}
	var policy FirewallPolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("解析防火墙策略失败: %w", err)
	}
	return normalizeFirewallPolicy(&policy), nil
}

// SaveFirewallPolicy 保存策略但不应用规则。
func SaveFirewallPolicy(policy *FirewallPolicy) error {
	if err := validateRawFirewallPolicy(policy); err != nil {
		return err
	}
	policy = normalizeFirewallPolicy(policy)
	if err := ValidateFirewallPolicy(policy); err != nil {
		return err
	}
	if err := ensureFirewallDir(); err != nil {
		return err
	}
	oldPolicy, _ := os.ReadFile(firewallPolicyFile)
	if len(oldPolicy) > 0 {
		backupPath := filepath.Join(firewallDir, "backups", fmt.Sprintf("policy.%s.json", time.Now().Format("20060102_150405")))
		if err := os.WriteFile(backupPath, oldPolicy, 0644); err != nil {
			logger.App.Warn("备份防火墙规则失败", "path", backupPath, "error", err)
		}
		pruneFirewallBackups("policy.*.json", 10)
	}
	policy.UpdatedAt = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化防火墙策略失败: %w", err)
	}
	if err := os.WriteFile(firewallPolicyFile, data, 0644); err != nil {
		return fmt.Errorf("保存防火墙策略失败: %w", err)
	}
	return nil
}

// ValidateFirewallPolicy 校验策略合法性。
func ValidateFirewallPolicy(policy *FirewallPolicy) error {
	if _, err := netip.ParsePrefix(policy.VMSubnet); err != nil {
		return fmt.Errorf("虚拟机网段无效: %s", policy.VMSubnet)
	}
	if policy.BlockAction != "reject" && policy.BlockAction != "drop" {
		return fmt.Errorf("拦截动作只支持 reject 或 drop")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_.:-]+$`).MatchString(policy.Bridge) {
		return fmt.Errorf("网桥名称格式无效")
	}
	for _, cidr := range policy.WhitelistCIDRs {
		if _, err := netip.ParsePrefix(cidr); err != nil {
			return fmt.Errorf("白名单 CIDR 无效: %s", cidr)
		}
	}
	for _, region := range policy.Regions {
		if region.Code == "" {
			return fmt.Errorf("区域代码不能为空")
		}
		for _, cidr := range region.CIDRs {
			if _, err := netip.ParsePrefix(cidr); err != nil {
				return fmt.Errorf("区域 %s 中存在无效 CIDR: %s", region.Code, cidr)
			}
		}
	}
	for vmName, override := range policy.VMOverrides {
		if strings.TrimSpace(vmName) == "" {
			return fmt.Errorf("VM 覆盖策略中存在空名称")
		}
		mode := normalizeOverrideMode(override.Mode)
		if mode == "" {
			return fmt.Errorf("VM %s 的覆盖模式无效", vmName)
		}
	}
	return nil
}

func validateRawFirewallPolicy(policy *FirewallPolicy) error {
	if policy == nil {
		return nil
	}
	for _, cidr := range policy.WhitelistCIDRs {
		if strings.TrimSpace(cidr) == "" {
			continue
		}
		if err := validateIPv4CIDROrAddr(cidr); err != nil {
			return fmt.Errorf("白名单 CIDR 无效: %s", cidr)
		}
	}
	for _, region := range policy.Regions {
		for _, cidr := range region.CIDRs {
			if strings.TrimSpace(cidr) == "" {
				continue
			}
			if err := validateIPv4CIDROrAddr(cidr); err != nil {
				return fmt.Errorf("区域 %s 中存在无效 CIDR: %s", region.Code, cidr)
			}
		}
	}
	return nil
}

// GetFirewallStatus 获取策略状态和 nft 实际状态。
func GetFirewallStatus() (*FirewallStatus, error) {
	policy, err := GetFirewallPolicy()
	if err != nil {
		return nil, err
	}
	activeResult := utils.ExecCommand("nft", "list", "table", "inet", firewallTable)
	nftResult := utils.ExecCommand("nft", "--version")
	return &FirewallStatus{
		Policy:         policy,
		Active:         activeResult.Error == nil,
		LastError:      strings.TrimSpace(activeResult.Stderr),
		RuleFile:       firewallRulesFile,
		PolicyFile:     firewallPolicyFile,
		TableName:      firewallTable,
		NftAvailable:   nftResult.Error == nil,
		VMs:            ListAllVMNames(),
		IPv6Note:       "第一版仅管控 KVM IPv4 转发流量；VM IPv6 转发默认被独立表拒绝，宿主机 IPv6 不在本功能范围内。",
		GeoIPCopyright: "内置下载源默认使用 IPdeny 聚合 CIDR 数据，请遵守其版权和使用限制。",
	}, nil
}

// PreviewFirewallRules 只生成规则文本，不写入系统规则。
func PreviewFirewallRules(policy *FirewallPolicy) (string, error) {
	if err := validateRawFirewallPolicy(policy); err != nil {
		return "", err
	}
	policy = normalizeFirewallPolicy(policy)
	if err := ValidateFirewallPolicy(policy); err != nil {
		return "", err
	}
	return BuildFirewallRules(policy)
}

// ApplyFirewallPolicy 保存、校验并应用 nft 规则。
func ApplyFirewallPolicy(policy *FirewallPolicy, progress func(int, string)) error {
	if progress != nil {
		progress(10, "正在生成防火墙规则...")
	}
	if err := validateRawFirewallPolicy(policy); err != nil {
		return err
	}
	policy = normalizeFirewallPolicy(policy)
	policy.Enabled = true
	rules, err := BuildFirewallRules(policy)
	if err != nil {
		return err
	}
	if err := ensureFirewallDir(); err != nil {
		return err
	}
	if progress != nil {
		progress(35, "正在执行 nft dry-run 校验...")
	}
	if err := writeAndCheckFirewallRules(rules); err != nil {
		return err
	}
	if progress != nil {
		progress(60, "正在备份旧规则并应用新规则...")
	}
	backupCurrentFirewallFiles()
	result := utils.ExecShell(fmt.Sprintf("nft delete table inet %s 2>/dev/null || true; nft -f '%s'", firewallTable, firewallRulesFile))
	if result.Error != nil {
		return fmt.Errorf("应用 nft 防火墙规则失败: %s", result.Stderr)
	}
	policy.AppliedAt = time.Now().Format(time.RFC3339)
	if err := SaveFirewallPolicy(policy); err != nil {
		return err
	}
	if progress != nil {
		progress(100, "防火墙规则已应用")
	}
	return nil
}

// DisableFirewall 删除独立 nft 表并把策略标记为未启用。
func DisableFirewall(progress func(int, string)) error {
	if progress != nil {
		progress(20, "正在删除 KVM 防火墙独立规则表...")
	}
	result := utils.ExecShell(fmt.Sprintf("nft delete table inet %s 2>/dev/null || true", firewallTable))
	if result.Error != nil {
		return fmt.Errorf("禁用防火墙失败: %s", result.Stderr)
	}
	policy, err := GetFirewallPolicy()
	if err != nil {
		return err
	}
	policy.Enabled = false
	if err := SaveFirewallPolicy(policy); err != nil {
		return err
	}
	if progress != nil {
		progress(100, "防火墙已禁用")
	}
	return nil
}

// RollbackFirewall 当前第一版回滚为删除独立表，保证快速恢复网络。
func RollbackFirewall(progress func(int, string)) error {
	return DisableFirewall(progress)
}

func writeAndCheckFirewallRules(rules string) error {
	if err := os.WriteFile(firewallRulesFile, []byte(rules), 0644); err != nil {
		return fmt.Errorf("写入 nft 规则文件失败: %w", err)
	}
	result := utils.ExecCommand("nft", "-c", "-f", firewallRulesFile)
	if result.Error != nil {
		return fmt.Errorf("nft dry-run 校验失败: %s", result.Stderr)
	}
	return nil
}

func backupCurrentFirewallFiles() {
	_ = ensureFirewallDir()
	ts := time.Now().Format("20060102_150405")
	if data, err := os.ReadFile(firewallRulesFile); err == nil && len(data) > 0 {
		_ = os.WriteFile(filepath.Join(firewallDir, "backups", "rules."+ts+".nft"), data, 0644)
	}
	pruneFirewallBackups("rules.*.nft", 10)
}

func pruneFirewallBackups(pattern string, keep int) {
	matches, _ := filepath.Glob(filepath.Join(firewallDir, "backups", pattern))
	sort.Slice(matches, func(i, j int) bool {
		ii, _ := os.Stat(matches[i])
		jj, _ := os.Stat(matches[j])
		if ii == nil || jj == nil {
			return matches[i] > matches[j]
		}
		return ii.ModTime().After(jj.ModTime())
	})
	for i := keep; i < len(matches); i++ {
		_ = os.Remove(matches[i])
	}
}
