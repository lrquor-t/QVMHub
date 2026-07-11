package vm_xml

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// KVMFeatureConfig KVM 虚拟化特性配置
type KVMFeatureConfig struct {
	KVMHidden bool   `json:"kvm_hidden"` // 隐藏 KVM 标志
	VendorID  string `json:"vendor_id"`  // Hyper-V vendor_id 伪装（空字符串表示不设置）
}

var (
	// <kvm><hidden state='on'/></kvm>
	vmKVMBlockRegexp  = regexp.MustCompile(`(?s)<kvm\b[^>]*>.*?</kvm>`)
	vmKVMHiddenRegexp = regexp.MustCompile(`<hidden\b[^>]*\bstate=['"]on['"][^>]*/>`)

	// <vendor_id state="on" value="..."/>
	vmVendorIDRegexp       = regexp.MustCompile(`<vendor_id\b[^/]*/>`)
	vmFeaturesBlockExprKVM = regexp.MustCompile(`(?s)<features\b[^>]*>.*?</features>`)

	// 嵌套虚拟化 CPU feature: <feature policy='require' name='vmx'/> 或 svm（含 disable 变体）
	vmNestedVirtFeatureRegexp = regexp.MustCompile(`<feature\b[^>]*\bname=['"](?:vmx|svm)['"][^>]*/>`)
	// <cpu ...> 块（自闭合或展开）
	vmCPUSelfCloseBlockRegex = regexp.MustCompile(`<cpu\b[^>/]*/>`)
	vmCPUBlockRegex          = regexp.MustCompile(`(?s)<cpu\b[^>]*>.*?</cpu>`)
)

// HasKVMFeatureValue 判断 KVM 特性配置是否有实际值
func (cfg *KVMFeatureConfig) HasValue() bool {
	if cfg == nil {
		return false
	}
	return cfg.KVMHidden || strings.TrimSpace(cfg.VendorID) != ""
}

// ParseKVMHiddenFromDomainXML 从 domain XML 解析 KVM 隐藏标志是否启用
func ParseKVMHiddenFromDomainXML(xmlStr string) bool {
	return vmKVMHiddenRegexp.MatchString(xmlStr)
}

// ParseNestedVirtFromDomainXML 从 domain XML 解析嵌套虚拟化是否启用
// 返回 true：存在 policy='require' 的 vmx/svm feature
// 返回 false：不存在 或 存在 policy='disable'（关闭嵌套虚拟化）
func ParseNestedVirtFromDomainXML(xmlStr string) bool {
	// 先检查是否有 require 标记
	requireRe := regexp.MustCompile(`<feature\b[^>]*\bpolicy=['"]require['"][^>]*\bname=['"](?:vmx|svm)['"][^>]*/>`)
	if requireRe.MatchString(xmlStr) {
		return true
	}
	// 如果有 disable 标记，说明用户显式关闭了嵌套虚拟化
	disableRe := regexp.MustCompile(`<feature\b[^>]*\bpolicy=['"]disable['"][^>]*\bname=['"](?:vmx|svm)['"][^>]*/>`)
	if disableRe.MatchString(xmlStr) {
		return false
	}
	// 没有嵌套虚拟化 feature → 默认为 host-passthrough 透传，视为已启用
	// （如果是非 host-passthrough 模式，没有 feature 就是未启用，但解析层面不做判断）
	return true
}

// ParseVendorIDFromDomainXML 从 domain XML 解析 Hyper-V vendor_id 伪装值
// 返回空字符串表示未设置 vendor_id
func ParseVendorIDFromDomainXML(xmlStr string) string {
	matches := vmVendorIDRegexp.FindString(xmlStr)
	if matches == "" {
		return ""
	}
	// 提取 value 属性
	valRe := regexp.MustCompile(`value=['"]([^'"]*)['"]`)
	valMatches := valRe.FindStringSubmatch(matches)
	if len(valMatches) < 2 {
		return ""
	}
	return strings.TrimSpace(valMatches[1])
}

// renderKVMHiddenBlock 生成 <kvm><hidden state='on'/></kvm> XML 块
func renderKVMHiddenBlock() string {
	return "    <kvm>\n      <hidden state='on'/>\n    </kvm>"
}

// renderVendorIDBlock 生成 <vendor_id state="on" value="..."/> XML
func renderVendorIDBlock(vendorID string) string {
	return fmt.Sprintf("      <vendor_id state='on' value='%s'/>", vendorID)
}

// ApplyKVMHiddenToDomainXML 向 domain XML 的 <features> 中注入或移除 KVM 隐藏标志
// 传入 nil 表示不修改，true/false 分别表示启用/关闭
func ApplyKVMHiddenToDomainXML(xmlStr string, enabled *bool) (string, error) {
	if enabled == nil {
		return xmlStr, nil
	}

	// 先移除现有的 kvm 块
	updated := vmKVMBlockRegexp.ReplaceAllString(xmlStr, "")

	if !*enabled {
		return updated, nil
	}

	// 注入 <kvm><hidden state='on'/></kvm>
	block := renderKVMHiddenBlock()
	if vmFeaturesBlockExprKVM.MatchString(updated) {
		return vmFeaturesBlockExprKVM.ReplaceAllStringFunc(updated, func(featuresBlock string) string {
			return strings.Replace(featuresBlock, "</features>", block+"\n  </features>", 1)
		}), nil
	}

	// 没有 <features> 块的兜底逻辑
	featuresXML := "  <features>\n" + block + "\n  </features>\n"
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
		return "", fmt.Errorf("写入 KVM 隐藏标志失败：未找到可插入 features 的位置")
	}
}

// renderNestedVirtFeatureBlock 生成嵌套虚拟化 feature XML
// policy 为 "require"（启用）或 "disable"（关闭），用于覆盖 host-passthrough 的透传
func renderNestedVirtFeatureBlock(featureName, policy string) string {
	return fmt.Sprintf("    <feature policy='%s' name='%s'/>", policy, featureName)
}

// DetectHostNestedVirtFeatureName 检测宿主机 CPU 的嵌套虚拟化特性名称
// Intel 返回 "vmx"，AMD 返回 "svm"，不支持的平台返回空字符串
func DetectHostNestedVirtFeatureName() string {
	// 检查 /proc/cpuinfo 中的 flags 字段
	data, err := readProcCPUInfo()
	if err != nil {
		return ""
	}
	if strings.Contains(data, " vmx ") || strings.HasSuffix(data, " vmx") {
		return "vmx"
	}
	if strings.Contains(data, " svm ") || strings.HasSuffix(data, " svm") {
		return "svm"
	}
	return ""
}

// readProcCPUInfo 读取 /proc/cpuinfo 内容
func readProcCPUInfo() (string, error) {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ApplyNestedVirtToDomainXML 向 domain XML 的 <cpu> 中注入嵌套虚拟化特性
// enabled 为 nil 不修改，true 启用（注入 policy='require'），false 关闭（注入 policy='disable'）
// 关闭时使用 policy='disable' 来覆盖 host-passthrough 的透传，确保虚拟机不继承宿主机 vmx/svm
// 仅 x86_64 + KVM 模式支持嵌套虚拟化；featureName 由 DetectHostNestedVirtFeatureName 传入
func ApplyNestedVirtToDomainXML(xmlStr string, enabled *bool, featureName string) (string, error) {
	if enabled == nil {
		return xmlStr, nil
	}

	// 先移除现有的嵌套虚拟化 feature（无论 require 还是 disable）
	updated := vmNestedVirtFeatureRegexp.ReplaceAllString(xmlStr, "")

	if featureName == "" {
		// 未检测到宿主机支持嵌套虚拟化，不做注入
		return updated, nil
	}

	// 检查架构（ARM/RISC-V 不支持嵌套 KVM 虚拟化）
	arch := ParseVMArchFromDomainXML(updated)
	if arch != "" && arch != "x86_64" {
		return updated, nil
	}

	policy := "require"
	if !*enabled {
		policy = "disable"
	}

	featureXML := "    <feature policy='" + policy + "' name='" + featureName + "'/>\n"

	// 情况 1: <cpu>...</cpu> 展开块
	if vmCPUBlockRegex.MatchString(updated) {
		return vmCPUBlockRegex.ReplaceAllStringFunc(updated, func(cpuBlock string) string {
			return strings.Replace(cpuBlock, "</cpu>", featureXML+"  </cpu>", 1)
		}), nil
	}

	// 情况 2: <cpu .../> 自闭合块 → 展开为完整块
	if vmCPUSelfCloseBlockRegex.MatchString(updated) {
		return vmCPUSelfCloseBlockRegex.ReplaceAllStringFunc(updated, func(selfClose string) string {
			// 去掉结尾的 />，替换为 >...\n</cpu>
			expanded := strings.Replace(selfClose, "/>", ">\n", 1)
			return expanded + featureXML + "  </cpu>"
		}), nil
	}

	// 没有 <cpu> 块则不需要操作
	return updated, nil
}

// ApplyVendorIDToHyperVBlock 向 domain XML 的 <hyperv> 块中注入或移除 vendor_id
// vendorID 为空字符串时移除 vendor_id，否则设置对应的伪装值
// 仅 x86_64 架构支持 Hyper-V vendor_id
func ApplyVendorIDToHyperVBlock(xmlStr string, vendorID string) (string, error) {
	trimmed := strings.TrimSpace(vendorID)

	// 先移除现有的 vendor_id
	updated := vmVendorIDRegexp.ReplaceAllString(xmlStr, "")

	if trimmed == "" {
		return updated, nil
	}

	// 检查是否支持（非 x86_64 架构不支持 Hyper-V）
	arch := ParseVMArchFromDomainXML(updated)
	if arch != "" && arch != "x86_64" {
		// ARM/RISC-V 等架构不支持 Hyper-V vendor_id，直接返回
		return updated, nil
	}

	// 注入 vendor_id 到 <hyperv> 块中
	block := renderVendorIDBlock(trimmed)
	if vmHyperVBlockRegexp.MatchString(updated) {
		return vmHyperVBlockRegexp.ReplaceAllStringFunc(updated, func(hypervBlock string) string {
			return strings.Replace(hypervBlock, "</hyperv>", block+"\n    </hyperv>", 1)
		}), nil
	}

	// 没有 <hyperv> 块：检查是否有 <features>
	if vmFeaturesBlockExprKVM.MatchString(updated) {
		// 在 </features> 前插入完整的 hyperv 块
		hypervXML := "    <hyperv mode='custom'>\n" + block + "\n    </hyperv>"
		return vmFeaturesBlockExprKVM.ReplaceAllStringFunc(updated, func(featuresBlock string) string {
			return strings.Replace(featuresBlock, "</features>", hypervXML+"\n  </features>", 1)
		}), nil
	}

	// 没有 <features> 块的兜底
	hypervXML := "  <features>\n" +
		"    <hyperv mode='custom'>\n" +
		block + "\n" +
		"    </hyperv>\n" +
		"  </features>\n"
	switch {
	case strings.Contains(updated, "<clock "):
		return strings.Replace(updated, "<clock ", hypervXML+"  <clock ", 1), nil
	case strings.Contains(updated, "<clock>"):
		return strings.Replace(updated, "<clock>", hypervXML+"  <clock>", 1), nil
	case strings.Contains(updated, "<devices/>"):
		return strings.Replace(updated, "<devices/>", hypervXML+"  <devices/>", 1), nil
	case strings.Contains(updated, "<devices />"):
		return strings.Replace(updated, "<devices />", hypervXML+"  <devices />", 1), nil
	case strings.Contains(updated, "<devices>"):
		return strings.Replace(updated, "<devices>", hypervXML+"  <devices>", 1), nil
	case strings.Contains(updated, "<on_poweroff>"):
		return strings.Replace(updated, "<on_poweroff>", hypervXML+"  <on_poweroff>", 1), nil
	default:
		return "", fmt.Errorf("写入 vendor_id 失败：未找到可插入 features 的位置")
	}
}
