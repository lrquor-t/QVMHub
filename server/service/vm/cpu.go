package vm

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// vmCPUModelTagRegex 匹配 <cpu> 块内 <model>...</model> 标签体内容
	vmCPUModelTagRegex = regexp.MustCompile(`(?s)<cpu\b[^>]*>.*?<model\b[^>]*>\s*([^<]+?)\s*</model>`)

	// vmCPUModelAttrRegex 匹配 <cpu> 标签 model='xxx' 属性（如 <cpu model='host-passthrough' .../>）
	vmCPUModelAttrRegex = regexp.MustCompile(`(?s)<cpu\b[^>]*\bmodel=['"]([^'"]+)['"]`)

	// vmCPUBlockFullRegex 匹配完整的 <cpu>...</cpu> 块
	vmCPUBlockFullRegex = regexp.MustCompile(`(?s)<cpu\b[^>]*>.*?</cpu>`)

	// vmCPUVendorRegex 匹配 <vendor>...</vendor>
	vmCPUVendorRegex = regexp.MustCompile(`(?s)\s*<vendor\b[^>]*>.*?</vendor>`)

	// vmCPUFeatureRegex 匹配 <feature ... /> 或 <feature ...>...</feature>
	vmCPUFeatureRegex = regexp.MustCompile(`(?s)\s*<feature\b[^>]*(?:/>|>.*?</feature>)`)

	// vmCPUCacheRegex 匹配 <cache ... /> 或 <cache ...>...</cache>
	vmCPUCacheRegex = regexp.MustCompile(`(?s)\s*<cache\b[^>]*(?:/>|>.*?</cache>)`)
)

// NormalizeDomainXML 标准化 domain XML 的换行符并裁剪尾部空白行。
func NormalizeDomainXML(xmlContent string) string {
	normalized := strings.ReplaceAll(xmlContent, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.TrimRight(normalized, "\n ") + "\n"
}

// ExtractCPUModelFromXML 从 domain XML 中提取 CPU 模型名称。
// 优先匹配 <cpu><model>xxx</model></cpu> 标签体，
// 回退匹配 <cpu model='xxx' .../> 属性。
// 未找到时返回空字符串。
func ExtractCPUModelFromXML(xmlContent string) string {
	// 优先：<cpu ...><model ...>Skylake-Client</model></cpu>
	if matches := vmCPUModelTagRegex.FindStringSubmatch(xmlContent); len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}
	// 回退：<cpu model='host-passthrough' .../>
	if matches := vmCPUModelAttrRegex.FindStringSubmatch(xmlContent); len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// InjectCPUFeatures 向 domain XML 的 <cpu> 块中注入 vendor / features / cache 配置。
// 仅对已有 <cpu> 块生效；不存在 <cpu> 块时返回原始 XML 不做修改。
// 已存在的同名子元素会先清除再重新注入，确保幂等。
func InjectCPUFeatures(xmlContent, vendor string, features []string, cacheLevel int, cacheMode string) string {
	if !vmCPUBlockFullRegex.MatchString(xmlContent) {
		return xmlContent
	}

	return vmCPUBlockFullRegex.ReplaceAllStringFunc(xmlContent, func(cpuBlock string) string {
		return injectIntoCPUBlock(cpuBlock, vendor, features, cacheLevel, cacheMode)
	})
}

// injectIntoCPUBlock 向单个 <cpu>...</cpu> 块注入子元素。
func injectIntoCPUBlock(cpuBlock, vendor string, features []string, cacheLevel int, cacheMode string) string {
	// 清除已有的 vendor / feature / cache
	cleaned := vmCPUVendorRegex.ReplaceAllString(cpuBlock, "")
	cleaned = vmCPUFeatureRegex.ReplaceAllString(cleaned, "")
	cleaned = vmCPUCacheRegex.ReplaceAllString(cleaned, "")

	// 构建待注入的子元素
	var inject strings.Builder

	if vendor != "" {
		inject.WriteString(fmt.Sprintf("\n    <vendor>%s</vendor>", vendor))
	}
	for _, feat := range features {
		feat = strings.TrimSpace(feat)
		if feat != "" {
			inject.WriteString(fmt.Sprintf("\n    <feature policy='require' name='%s'/>", feat))
		}
	}
	if cacheLevel > 0 {
		mode := strings.TrimSpace(cacheMode)
		if mode == "" {
			mode = "emulate"
		}
		inject.WriteString(fmt.Sprintf("\n    <cache level='%d' mode='%s'/>", cacheLevel, mode))
	}

	if inject.Len() == 0 {
		return cleaned
	}

	// 在 </cpu> 之前插入
	if strings.Contains(cleaned, "</cpu>") {
		return strings.Replace(cleaned, "</cpu>", inject.String()+"\n  </cpu>", 1)
	}
	return cleaned
}
