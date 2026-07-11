package vm_xml

import (
	"fmt"
	"regexp"
	"strings"
)

const vmGuestAgentTargetName = "org.qemu.guest_agent.0"

var (
	vmGuestAgentTargetRegex  = regexp.MustCompile(`<target\b[^>]*name=['"]org\.qemu\.guest_agent\.0['"][^>]*/?>`)
	vmGuestAgentChannelRegex = regexp.MustCompile(`(?s)\n?\s*<channel\b[^>]*>.*?<target\b[^>]*name=['"]org\.qemu\.guest_agent\.0['"][^>]*/?>.*?</channel>`)
)

// VMGuestAgentConfig 虚拟机 QEMU Guest Agent 配置
type VMGuestAgentConfig struct {
	Enabled bool `json:"enabled"`
}

// NormalizeVMGuestAgentConfig 规范化 Guest Agent 配置，nil 或未指定时默认启用
func NormalizeVMGuestAgentConfig(cfg *VMGuestAgentConfig) *VMGuestAgentConfig {
	if cfg == nil {
		return &VMGuestAgentConfig{Enabled: true}
	}
	return &VMGuestAgentConfig{
		Enabled: cfg.Enabled,
	}
}

// ParseVMGuestAgentConfigFromDomainXML 从 domain XML 中解析 Guest Agent 配置
func ParseVMGuestAgentConfigFromDomainXML(xmlContent string) *VMGuestAgentConfig {
	return &VMGuestAgentConfig{
		Enabled: vmGuestAgentTargetRegex.MatchString(xmlContent),
	}
}

// ApplyVMGuestAgentConfigToDomainXML 将 Guest Agent 配置写入 domain XML
func ApplyVMGuestAgentConfigToDomainXML(xmlContent string, cfg *VMGuestAgentConfig) (string, error) {
	normalized := NormalizeVMGuestAgentConfig(cfg)
	cleanedXML := vmGuestAgentChannelRegex.ReplaceAllString(xmlContent, "")

	if !normalized.Enabled {
		return cleanedXML, nil
	}

	channelXML := "" +
		"    <channel type='unix'>\n" +
		"      <source mode='bind'/>\n" +
		"      <target type='virtio' name='" + vmGuestAgentTargetName + "'/>\n" +
		"    </channel>\n"

	if strings.Contains(cleanedXML, "</devices>") {
		return strings.Replace(cleanedXML, "</devices>", channelXML+"  </devices>", 1), nil
	}

	return "", fmt.Errorf("写入 QEMU Guest Agent 配置失败：未找到 devices 节点")
}
