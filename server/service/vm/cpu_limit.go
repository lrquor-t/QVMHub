package vm

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"qvmhub/utils"
)

const (
	VMCPULimitUnlimited     = 0
	vmCPULimitDefaultPeriod = 100000
	vmCPULimitMinPercent    = 1
	vmCPULimitMaxPercent    = 100
)

var (
	vmCPUTuneBlockRegexp       = regexp.MustCompile(`(?s)<cputune\b[^>]*(?:/>|>.*?</cputune>)`)
	vmSelfClosingCPUTuneRegexp = regexp.MustCompile(`^<cputune\b[^>]*/>$`)
	vmCPUTunePeriodRegexp      = regexp.MustCompile(`(?s)\n?\s*<period>\s*-?[0-9]+\s*</period>`)
	vmCPUTuneQuotaRegexp       = regexp.MustCompile(`(?s)\n?\s*<quota>\s*-?[0-9]+\s*</quota>`)
	vmCPUTunePeriodValueRegexp = regexp.MustCompile(`(?s)<period>\s*(-?[0-9]+)\s*</period>`)
	vmCPUTuneQuotaValueRegexp  = regexp.MustCompile(`(?s)<quota>\s*(-?[0-9]+)\s*</quota>`)
)

// NormalizeVMCPULimitPercent 规范化 CPU 限制百分比，0 表示无限制。
func NormalizeVMCPULimitPercent(percent int) int {
	if percent <= 0 {
		return VMCPULimitUnlimited
	}
	return percent
}

// ValidateVMCPULimitPercent 校验 CPU 限制百分比。
func ValidateVMCPULimitPercent(percent int) error {
	if percent == VMCPULimitUnlimited {
		return nil
	}
	if percent < vmCPULimitMinPercent || percent > vmCPULimitMaxPercent {
		return fmt.Errorf("CPU 限制必须在 %d-%d%% 之间，或留空表示无限制", vmCPULimitMinPercent, vmCPULimitMaxPercent)
	}
	return nil
}

// ParseVMCPULimitPercentFromDomainXML 从 domain XML 中解析 CPU 限制百分比。
func ParseVMCPULimitPercentFromDomainXML(xmlStr string, vcpu int) int {
	block := vmCPUTuneBlockRegexp.FindString(xmlStr)
	if strings.TrimSpace(block) == "" {
		return VMCPULimitUnlimited
	}

	period := parseVMCPULimitTagValue(block, vmCPUTunePeriodValueRegexp)
	quota := parseVMCPULimitTagValue(block, vmCPUTuneQuotaValueRegexp)
	if period <= 0 || quota == 0 || quota < 0 {
		return VMCPULimitUnlimited
	}

	if vcpu <= 0 {
		vcpu = ParseVCPUCountFromDomainXML(xmlStr)
	}
	if vcpu <= 0 {
		return VMCPULimitUnlimited
	}

	totalQuota := int64(period) * int64(vcpu)
	if totalQuota <= 0 {
		return VMCPULimitUnlimited
	}

	percent := int((int64(quota)*100 + totalQuota/2) / totalQuota)
	if percent <= 0 {
		return vmCPULimitMinPercent
	}
	if percent > vmCPULimitMaxPercent {
		return vmCPULimitMaxPercent
	}
	return percent
}

// ApplyVMCPULimitToDomainXML 将 CPU 限制写入 domain XML。
func ApplyVMCPULimitToDomainXML(xmlStr string, vcpu, percent int) string {
	normalized := NormalizeVMCPULimitPercent(percent)
	block := vmCPUTuneBlockRegexp.FindString(xmlStr)
	if strings.TrimSpace(block) == "" {
		if normalized == VMCPULimitUnlimited {
			return xmlStr
		}
		newBlock := buildVMCPUTuneBlock(vcpu, normalized)
		if strings.Contains(xmlStr, "<devices>") {
			return strings.Replace(xmlStr, "<devices>", newBlock+"\n  <devices>", 1)
		}
		return xmlStr
	}

	updatedBlock := applyVMCPULimitToCPUTuneBlock(block, vcpu, normalized)
	if strings.TrimSpace(updatedBlock) == "" {
		return vmCPUTuneBlockRegexp.ReplaceAllString(xmlStr, "")
	}
	return vmCPUTuneBlockRegexp.ReplaceAllString(xmlStr, updatedBlock)
}

// SetVMCPULimitPercent 设置虚拟机 CPU 限制百分比。
func SetVMCPULimitPercent(name string, vcpu, percent int) error {
	if err := D.HookEnsureVMNotMigrating(name, "设置 CPU 限制"); err != nil {
		return err
	}

	normalized := NormalizeVMCPULimitPercent(percent)
	if err := ValidateVMCPULimitPercent(normalized); err != nil {
		return err
	}

	xmlStr, err := GetVMInactiveDomainXML(name)
	if err != nil {
		return err
	}
	if vcpu <= 0 {
		vcpu = ParseVCPUCountFromDomainXML(xmlStr)
	}
	if vcpu <= 0 {
		return fmt.Errorf("无法识别虚拟机 CPU 核心数")
	}

	updatedXML := ApplyVMCPULimitToDomainXML(xmlStr, vcpu, normalized)
	if err := SetVMInactiveDomainXML(name, updatedXML); err != nil {
		return err
	}

	stateResult := utils.ExecCommand("virsh", "domstate", name)
	if stateResult.Error == nil && strings.TrimSpace(stateResult.Stdout) == "running" {
		if err := applyVMLiveCPULimit(name, vcpu, normalized); err != nil {
			return fmt.Errorf("已保存持久化 CPU 限制，但同步运行态失败: %w", err)
		}
	}

	RefreshVMCacheByNameAsync(name)
	return nil
}

func buildVMCPUTuneBlock(vcpu, percent int) string {
	quota := calculateVMCPULimitQuota(vcpu, percent)
	return fmt.Sprintf("  <cputune>\n    <period>%d</period>\n    <quota>%d</quota>\n  </cputune>", vmCPULimitDefaultPeriod, quota)
}

func applyVMCPULimitToCPUTuneBlock(block string, vcpu, percent int) string {
	trimmed := strings.TrimSpace(block)
	if trimmed == "" {
		return ""
	}
	if percent == VMCPULimitUnlimited && vmSelfClosingCPUTuneRegexp.MatchString(trimmed) {
		return ""
	}
	if percent != VMCPULimitUnlimited && vmSelfClosingCPUTuneRegexp.MatchString(trimmed) {
		return buildVMCPUTuneBlock(vcpu, percent)
	}

	updated := vmCPUTunePeriodRegexp.ReplaceAllString(block, "")
	updated = vmCPUTuneQuotaRegexp.ReplaceAllString(updated, "")
	if percent == VMCPULimitUnlimited {
		if isVMCPUTuneBlockEmpty(updated) {
			return ""
		}
		return updated
	}

	indent := leadingWhitespace(block)
	childIndent := indent + "  "
	limitXML := fmt.Sprintf("\n%s<period>%d</period>\n%s<quota>%d</quota>", childIndent, vmCPULimitDefaultPeriod, childIndent, calculateVMCPULimitQuota(vcpu, percent))
	if strings.Contains(updated, "</cputune>") {
		return strings.Replace(updated, "</cputune>", limitXML+"\n"+indent+"</cputune>", 1)
	}
	return updated
}

func isVMCPUTuneBlockEmpty(block string) bool {
	trimmed := strings.TrimSpace(block)
	if trimmed == "" {
		return true
	}
	openStart := strings.Index(trimmed, ">")
	closeStart := strings.LastIndex(trimmed, "</cputune>")
	if openStart < 0 || closeStart < 0 || closeStart <= openStart {
		return false
	}
	inner := strings.TrimSpace(trimmed[openStart+1 : closeStart])
	return inner == ""
}

func calculateVMCPULimitQuota(vcpu, percent int) int64 {
	if vcpu <= 0 || percent <= 0 {
		return -1
	}
	quota := int64(vmCPULimitDefaultPeriod) * int64(vcpu) * int64(percent) / 100
	if quota > 0 && quota < 1000 {
		return 1000
	}
	return quota
}

func applyVMLiveCPULimit(name string, vcpu, percent int) error {
	args := []string{"schedinfo", name, "--live"}
	if percent == VMCPULimitUnlimited {
		args = append(args, "--set", "vcpu_period=0", "--set", "vcpu_quota=-1")
	} else {
		args = append(args,
			"--set", fmt.Sprintf("vcpu_period=%d", vmCPULimitDefaultPeriod),
			"--set", fmt.Sprintf("vcpu_quota=%d", calculateVMCPULimitQuota(vcpu, percent)),
		)
	}
	result := utils.ExecCommand("virsh", args...)
	if result.Error != nil {
		message := strings.TrimSpace(result.Stderr)
		if message == "" && result.Error != nil {
			message = result.Error.Error()
		}
		return fmt.Errorf("%s", message)
	}
	return nil
}

func parseVMCPULimitTagValue(block string, pattern *regexp.Regexp) int {
	matches := pattern.FindStringSubmatch(block)
	if len(matches) < 2 {
		return 0
	}
	value, err := strconv.Atoi(strings.TrimSpace(matches[1]))
	if err != nil {
		return 0
	}
	return value
}
