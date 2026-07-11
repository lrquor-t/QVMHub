package vm

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"qvmhub/utils"
)

var (
	vmAPICRegex         = regexp.MustCompile(`(?s)\n?\s*<apic\b[^>]*/>`)
	vmFeaturesBlockExpr = regexp.MustCompile(`(?s)<features\b[^>]*>.*?</features>`)
	vmAPICArchRegex     = regexp.MustCompile(`<type\b[^>]*\barch=['"]([^'"]+)['"]`)
)

// supportsVMAPICForArch 判断指定架构是否支持 APIC（仅 x86 系列支持）。
func supportsVMAPICForArch(archStr string) bool {
	switch strings.ToLower(strings.TrimSpace(archStr)) {
	case "", "x86_64", "i686", "i586", "i486", "i386":
		return true
	default:
		return false
	}
}

// supportsVMAPICForDomainXML 从 domain XML 中提取架构并判断是否支持 APIC。
func supportsVMAPICForDomainXML(xmlContent string) bool {
	matches := vmAPICArchRegex.FindStringSubmatch(xmlContent)
	if len(matches) < 2 {
		return true // 解析失败默认支持（x86降级）
	}
	return supportsVMAPICForArch(matches[1])
}

// ResolveVMAPICEnabled 解析 APIC 最终状态，未显式传入时默认启用。
func ResolveVMAPICEnabled(enabled *bool) bool {
	if enabled == nil {
		return true
	}
	return *enabled
}

// ParseVMAPICFromDomainXML 从 domain XML 中解析 APIC 是否启用。
func ParseVMAPICFromDomainXML(xmlContent string) bool {
	return vmAPICRegex.MatchString(xmlContent)
}

// ApplyVMAPICToDomainXML 将 APIC 开关写入 domain XML。
func ApplyVMAPICToDomainXML(xmlContent string, enabled *bool) (string, error) {
	// ARM 等非 x86 架构不支持 APIC，直接返回
	if !supportsVMAPICForDomainXML(xmlContent) {
		return xmlContent, nil
	}

	resolvedEnabled := ResolveVMAPICEnabled(enabled)
	updated := vmAPICRegex.ReplaceAllString(xmlContent, "")
	if !resolvedEnabled {
		return updated, nil
	}

	if vmFeaturesBlockExpr.MatchString(updated) {
		return vmFeaturesBlockExpr.ReplaceAllStringFunc(updated, func(block string) string {
			return strings.Replace(block, "</features>", "    <apic/>\n  </features>", 1)
		}), nil
	}

	featuresXML := "  <features>\n    <apic/>\n  </features>\n"
	switch {
	case strings.Contains(updated, "<clock "):
		return strings.Replace(updated, "<clock ", featuresXML+"  <clock ", 1), nil
	case strings.Contains(updated, "<clock>"):
		return strings.Replace(updated, "<clock>", featuresXML+"  <clock>", 1), nil
	case strings.Contains(updated, "<devices/>"):
		return strings.Replace(updated, "<devices/>", featuresXML+"  <devices/>", 1), nil
	case strings.Contains(updated, "<devices />"):
		return strings.Replace(updated, "<devices />", featuresXML+"  <devices />", 1), nil
	case strings.Contains(updated, "<devices>"):
		return strings.Replace(updated, "<devices>", featuresXML+"  <devices>", 1), nil
	case strings.Contains(updated, "<on_poweroff>"):
		return strings.Replace(updated, "<on_poweroff>", featuresXML+"  <on_poweroff>", 1), nil
	default:
		return "", fmt.Errorf("写入 APIC 配置失败：未找到可插入 features 的位置")
	}
}

// SetVMAPICConfig 修改虚拟机 APIC 配置。
func SetVMAPICConfig(name string, enabled bool) error {
	xmlResult := utils.ExecCommand("virsh", "dumpxml", name, "--inactive")
	if xmlResult.Error != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %s", xmlResult.Stderr)
	}

	newXML, err := ApplyVMAPICToDomainXML(xmlResult.Stdout, &enabled)
	if err != nil {
		return err
	}

	xmlPath := fmt.Sprintf("/tmp/_apic-%s.xml", name)
	if err := os.WriteFile(xmlPath, []byte(newXML), 0644); err != nil {
		return fmt.Errorf("写入 APIC 配置文件失败: %w", err)
	}
	defer os.Remove(xmlPath)

	defineResult := utils.ExecCommand("virsh", "define", xmlPath)
	if defineResult.Error != nil {
		return fmt.Errorf("设置 APIC 配置失败: %s", defineResult.Stderr)
	}

	return nil
}
