package vm

import (
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"strings"

	"qvmhub/service/vm_xml"
	"qvmhub/utils"
)

// ValidateVMInactiveDomainXML 校验用于编辑的 domain XML。
func ValidateVMInactiveDomainXML(name, xmlContent string) error {
	trimmed := strings.TrimSpace(xmlContent)
	if trimmed == "" {
		return fmt.Errorf("XML 内容不能为空")
	}

	decoder := xml.NewDecoder(strings.NewReader(trimmed))
	decoder.Strict = true

	var domain vm_xml.VMXMLDomainEnvelope
	if err := decoder.Decode(&domain); err != nil {
		return fmt.Errorf("XML 格式不合法: %w", err)
	}
	if domain.XMLName.Local != "domain" {
		return fmt.Errorf("XML 根节点必须是 domain")
	}

	xmlName := strings.TrimSpace(domain.Name)
	if xmlName == "" {
		return fmt.Errorf("XML 中缺少虚拟机名称")
	}
	if xmlName != strings.TrimSpace(name) {
		return fmt.Errorf("不支持通过 XML 编辑修改虚拟机名称")
	}

	return nil
}

func buildDomainXMLTempPattern(name string) string {
	safeName := vm_xml.VMXMLTempNameSanitizer.ReplaceAllString(strings.TrimSpace(name), "_")
	if safeName == "" {
		safeName = "vm"
	}
	return fmt.Sprintf("_domain-xml-%s-*.xml", safeName)
}

// GetVMInactiveDomainXML 获取虚拟机持久化配置对应的 domain XML。
func GetVMInactiveDomainXML(name string) (string, error) {
	xmlResult := utils.ExecCommand("virsh", "dumpxml", name, "--inactive")
	if xmlResult.Error != nil {
		return "", fmt.Errorf("获取虚拟机 XML 失败: %s", xmlResult.Stderr)
	}
	return vm_xml.NormalizeDomainXMLForEdit(xmlResult.Stdout), nil
}

// SetVMInactiveDomainXML 写入虚拟机持久化配置对应的 domain XML。
func SetVMInactiveDomainXML(name, xmlContent string) error {
	normalized := vm_xml.NormalizeDomainXMLForEdit(xmlContent)
	if err := ValidateVMInactiveDomainXML(name, normalized); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp("/tmp", buildDomainXMLTempPattern(name))
	if err != nil {
		return fmt.Errorf("创建临时 XML 文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("关闭临时 XML 文件失败: %w", err)
	}
	defer os.Remove(tmpPath)

	if err := os.WriteFile(tmpPath, []byte(normalized), 0600); err != nil {
		return fmt.Errorf("写入临时 XML 文件失败: %w", err)
	}

	defineResult := utils.ExecCommand("virsh", "define", tmpPath)
	if defineResult.Error != nil {
		return fmt.Errorf("保存虚拟机 XML 失败: %s", defineResult.Stderr)
	}

	return nil
}

// 匹配自闭合 <controller ... model='pcie-root-port' .../> 标签
var reSelfClosePCIERootPort = regexp.MustCompile(`\s*<controller[^>]*model=['"]pcie-root-port['"][^>]*/\s*>`)

// 匹配非自闭合 <controller ... model='pcie-root-port'>...</controller> 标签
var reFullPCIERootPort = regexp.MustCompile(`(?s)\s*<controller[^>]*model=['"]pcie-root-port['"][^>]*>.*?</controller>`)

// InjectPCIERootPorts 为 q35 虚拟机 XML 注入正确索引的 pcie-root-port 控制器
// 会先清除 virt-install 可能生成的不正确的 pcie-root-port，再按正确索引重建
func InjectPCIERootPorts(xmlContent string, portCount int) string {
	// ARM virt 机型的 PCIe 拓扑由 virt-install 自动生成，无需手动注入。
	// 此处仅对 q35 机型（x86）注入额外的 PCIe root ports。
	if !strings.Contains(xmlContent, "q35") {
		return xmlContent
	}
	if portCount <= 0 {
		return xmlContent
	}

	// 清除所有已有的 pcie-root-port 控制器（virt-install 可能生成了错误索引的）
	cleaned := reSelfClosePCIERootPort.ReplaceAllString(xmlContent, "")
	cleaned = reFullPCIERootPort.ReplaceAllString(cleaned, "")

	// 构建正确索引的 pcie-root-port 控制器（index 从 1 开始，0 留给 pcie-root）
	var ports strings.Builder
	for i := 0; i < portCount; i++ {
		ports.WriteString(fmt.Sprintf("    <controller type='pci' index='%d' model='pcie-root-port'/>\n", i+1))
	}

	// 注入到 </devices> 之前
	result := strings.Replace(cleaned, "  </devices>", ports.String()+"  </devices>", 1)
	return result
}
