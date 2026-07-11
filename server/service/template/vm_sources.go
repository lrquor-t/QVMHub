package template

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"qvmhub/utils"
)

// listVMTemplateSources scans VM XML files to find template source references.
func listVMTemplateSources() ([]TemplateRelatedVM, error) {
	xmlPaths, err := filepath.Glob("/etc/libvirt/qemu/*.xml")
	if err != nil {
		return nil, err
	}
	sort.Strings(xmlPaths)

	result := make([]TemplateRelatedVM, 0, len(xmlPaths))
	for _, xmlPath := range xmlPaths {
		content, err := os.ReadFile(xmlPath)
		if err != nil {
			continue
		}
		text := string(content)
		nameMatch := templateSourceNamePattern.FindStringSubmatch(text)
		if len(nameMatch) < 2 {
			continue
		}
		vmName := strings.TrimSuffix(filepath.Base(xmlPath), ".xml")
		nodeID := ""
		nodeMatch := templateSourceNodePattern.FindStringSubmatch(text)
		if len(nodeMatch) >= 2 {
			nodeID = strings.TrimSpace(nodeMatch[1])
		}
		cloneMode := ""
		cloneMatch := templateSourceCloneModePattern.FindStringSubmatch(text)
		if len(cloneMatch) >= 2 {
			cloneMode = strings.TrimSpace(cloneMatch[1])
		}
		result = append(result, TemplateRelatedVM{
			Name:      vmName,
			Template:  strings.TrimSpace(nameMatch[1]),
			NodeID:    nodeID,
			CloneMode: cloneMode,
		})
	}
	return result, nil
}

// WriteVMTemplateSource writes template source metadata to a VM's libvirt XML.
func WriteVMTemplateSource(vmName, templateName, cloneMode string) error {
	tpl, err := GetTemplateInfoByName(templateName)
	if err != nil {
		return err
	}
	wrapper := vmTemplateSource{
		XMLNS:        vmTemplateSourceMetadataURI,
		TemplateName: tpl.Name,
		TemplateUID:  tpl.TemplateUID,
		NodeID:       tpl.NodeID,
		CloneMode:    cloneMode,
	}
	xmlBytes, err := xml.Marshal(wrapper)
	if err != nil {
		return err
	}
	result := utils.ExecCommand(
		"virsh", "metadata", vmName, vmTemplateSourceMetadataURI,
		"--config", "--key", vmTemplateSourceMetadataKey, "--set", string(xmlBytes),
	)
	if result.Error != nil {
		return fmt.Errorf("写入虚拟机模板来源失败: %s", result.Stderr)
	}
	return nil
}

// ReadVMTemplateSource reads template source metadata from a VM's libvirt XML.
func ReadVMTemplateSource(vmName string) *vmTemplateSource {
	result := utils.ExecCommand("virsh", "metadata", vmName, vmTemplateSourceMetadataURI, "--config")
	if result.Error != nil || strings.TrimSpace(result.Stdout) == "" {
		return nil
	}
	var source vmTemplateSource
	if err := xml.Unmarshal([]byte(result.Stdout), &source); err != nil {
		return nil
	}
	return &source
}
