package vpc

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"qvmhub/logger"
	"qvmhub/service/ip_resolver"
	"qvmhub/utils"
)

func ensureVMVPCInterfaceConfig(vmName string, vlanID int) error {
	if strings.TrimSpace(vmName) == "" || vlanID <= 0 {
		return nil
	}
	result := utils.ExecCommand("virsh", "dumpxml", vmName, "--inactive")
	if result.Error != nil {
		result = utils.ExecCommand("virsh", "dumpxml", vmName)
	}
	if result.Error != nil {
		return fmt.Errorf("读取 VM XML 失败: %s", result.Stderr)
	}
	if firstOVSInterfaceHasVLANTag(result.Stdout, vlanID) && firstOVSInterfaceUsesBridge(result.Stdout, HookOvsBridgeName()) {
		return nil
	}
	updated, changed := setFirstOVSInterfaceVPC(result.Stdout, vlanID)
	if !changed {
		return nil
	}
	xmlPath := filepath.Join(os.TempDir(), fmt.Sprintf("_vpc-vlan-%s.xml", SafeVMXMLFileName(vmName)))
	if err := os.WriteFile(xmlPath, []byte(updated), 0600); err != nil {
		return fmt.Errorf("写入 VM VPC XML 失败: %w", err)
	}
	defer os.Remove(xmlPath)
	define := utils.ExecCommand("virsh", "define", xmlPath)
	if define.Error != nil {
		return fmt.Errorf("持久化 VM VPC VLAN 失败: %s", define.Stderr)
	}
	return nil
}

func ensureVMVPCRuntimeInterfaceConfig(vmName string, vlanID int) error {
	if strings.TrimSpace(vmName) == "" || vlanID <= 0 {
		return nil
	}
	result := utils.ExecCommand("virsh", "dumpxml", vmName)
	if result.Error != nil {
		return fmt.Errorf("读取运行中 VM XML 失败: %s", result.Stderr)
	}
	if firstOVSInterfaceHasVLANTag(result.Stdout, vlanID) && firstOVSInterfaceUsesBridge(result.Stdout, HookOvsBridgeName()) {
		return nil
	}
	updated, changed := setFirstOVSInterfaceVPC(result.Stdout, vlanID)
	if !changed {
		return nil
	}
	currentBlock, ok := extractFirstOVSInterfaceBlock(result.Stdout)
	if !ok {
		return nil
	}
	updatedBlock, ok := extractFirstOVSInterfaceBlock(updated)
	if !ok {
		return nil
	}
	if mac := ip_resolver.GetFirstVMMAC(vmName); mac != "" {
		HookCleanAllVPCDHCPLeases(mac, "")
		HookCleanOVSDHCPLease(mac, "")
	}
	return replugVMInterfaceLive(vmName, currentBlock, updatedBlock)
}

func replugVMInterfaceLive(vmName, currentBlock, updatedBlock string) error {
	currentBlock = StripRuntimeOnlyInterfaceElements(currentBlock)
	updatedBlock = StripRuntimeOnlyInterfaceElements(updatedBlock)
	if strings.TrimSpace(currentBlock) == "" || strings.TrimSpace(updatedBlock) == "" {
		return nil
	}
	detachPath := filepath.Join(os.TempDir(), fmt.Sprintf("_vm-net-detach-%s.xml", SafeVMXMLFileName(vmName)))
	attachPath := filepath.Join(os.TempDir(), fmt.Sprintf("_vm-net-attach-%s.xml", SafeVMXMLFileName(vmName)))
	if err := os.WriteFile(detachPath, []byte(currentBlock), 0600); err != nil {
		return fmt.Errorf("写入运行态网卡 detach XML 失败: %w", err)
	}
	defer os.Remove(detachPath)
	if err := os.WriteFile(attachPath, []byte(updatedBlock), 0600); err != nil {
		return fmt.Errorf("写入运行态网卡 attach XML 失败: %w", err)
	}
	defer os.Remove(attachPath)
	detach := utils.ExecCommandWithTimeout("virsh", 60*time.Second, "detach-device", vmName, detachPath, "--live")
	if detach.Error != nil {
		return fmt.Errorf("热拔 VM 网卡失败: %s", HookFirstNonEmpty(detach.Stderr, detach.Error.Error()))
	}
	time.Sleep(2 * time.Second)
	attach := utils.ExecCommandWithTimeout("virsh", 60*time.Second, "attach-device", vmName, attachPath, "--live")
	if attach.Error != nil {
		restore := utils.ExecCommandWithTimeout("virsh", 60*time.Second, "attach-device", vmName, detachPath, "--live")
		if restore.Error != nil {
			return fmt.Errorf("热插 VM 网卡失败: %s；恢复原网卡也失败: %s", HookFirstNonEmpty(attach.Stderr, attach.Error.Error()), HookFirstNonEmpty(restore.Stderr, restore.Error.Error()))
		}
		return fmt.Errorf("热插 VM 网卡失败，已恢复原网卡: %s", HookFirstNonEmpty(attach.Stderr, attach.Error.Error()))
	}
	return nil
}

func ensureVMBridgeInterfaceConfig(vmName, bridge string, vlanID int) error {
	if strings.TrimSpace(vmName) == "" || strings.TrimSpace(bridge) == "" {
		return nil
	}
	result := utils.ExecCommand("virsh", "dumpxml", vmName, "--inactive")
	if result.Error != nil {
		result = utils.ExecCommand("virsh", "dumpxml", vmName)
	}
	if result.Error != nil {
		return fmt.Errorf("读取 VM XML 失败: %s", result.Stderr)
	}
	updated, changed := setFirstOVSInterfaceDirectBridge(result.Stdout, bridge, vlanID)
	if !changed {
		return nil
	}
	xmlPath := filepath.Join(os.TempDir(), fmt.Sprintf("_vm-bridge-%s.xml", SafeVMXMLFileName(vmName)))
	if err := os.WriteFile(xmlPath, []byte(updated), 0600); err != nil {
		return fmt.Errorf("写入 VM 桥接 XML 失败: %w", err)
	}
	defer os.Remove(xmlPath)
	define := utils.ExecCommand("virsh", "define", xmlPath)
	if define.Error != nil {
		return fmt.Errorf("持久化 VM 桥接配置失败: %s", define.Stderr)
	}
	return nil
}

func ensureVMDirectBridgeRuntimeVLAN(vmName string, vlanID int) error {
	vnetIF := ip_resolver.GetVMVnetIF(vmName)
	if vnetIF == "" {
		logger.App.Warn("无法获取桥接 VM vnet 接口，跳过 VLAN tag 设置", "vm", vmName)
		return nil
	}
	// 检查端口是否实际存在于 OVS
	if !ovsPortExists(vnetIF) {
		logger.App.Warn("OVS 端口不存在，跳过桥接 VLAN tag 设置", "port", vnetIF)
		return nil
	}
	if vlanID > 0 {
		targetTag := strconv.Itoa(vlanID)
		if currentTag, ok := getOVSPortTag(vnetIF); ok && currentTag == targetTag {
			return nil
		}
		result := utils.ExecCommand("ovs-vsctl", "set", "Port", vnetIF, "tag="+targetTag)
		if result.Error != nil {
			return fmt.Errorf("设置桥接 VM OVS VLAN tag 失败: %s", result.Stderr)
		}
		return nil
	}
	result := utils.ExecCommand("ovs-vsctl", "remove", "Port", vnetIF, "tag", "0")
	if result.Error != nil {
		result = utils.ExecCommand("ovs-vsctl", "clear", "Port", vnetIF, "tag")
	}
	if result.Error != nil {
		return fmt.Errorf("清理桥接 VM OVS VLAN tag 失败: %s", result.Stderr)
	}
	return nil
}

func firstOVSInterfaceHasVLANTag(xmlText string, vlanID int) bool {
	if strings.TrimSpace(xmlText) == "" || vlanID <= 0 {
		return false
	}
	current, ok := extractFirstOVSInterfaceVLANTag(xmlText)
	return ok && current == vlanID
}

func firstOVSInterfaceUsesBridge(xmlText, bridge string) bool {
	if strings.TrimSpace(xmlText) == "" || strings.TrimSpace(bridge) == "" {
		return false
	}
	current, ok := extractFirstOVSInterfaceBridge(xmlText)
	return ok && current == strings.TrimSpace(bridge)
}

func extractFirstOVSInterfaceBridge(xmlText string) (string, bool) {
	if strings.TrimSpace(xmlText) == "" {
		return "", false
	}
	searchFrom := 0
	for {
		startRel := strings.Index(xmlText[searchFrom:], "<interface ")
		if startRel < 0 {
			return "", false
		}
		start := searchFrom + startRel
		endRel := strings.Index(xmlText[start:], "</interface>")
		if endRel < 0 {
			return "", false
		}
		end := start + endRel + len("</interface>")
		block := xmlText[start:end]
		hasBridgeType := strings.Contains(block, "<interface type='bridge'") || strings.Contains(block, `<interface type="bridge"`)
		hasOVS := strings.Contains(block, "virtualport type='openvswitch'") || strings.Contains(block, `virtualport type="openvswitch"`)
		if hasBridgeType && hasOVS {
			sourceRe := regexp.MustCompile(`<source\s+bridge=['"]([^'"]+)['"]\s*/>`)
			match := sourceRe.FindStringSubmatch(block)
			if len(match) == 2 {
				return match[1], true
			}
			return "", false
		}
		searchFrom = end
	}
}

func extractFirstOVSInterfaceBlock(xmlText string) (string, bool) {
	if strings.TrimSpace(xmlText) == "" {
		return "", false
	}
	searchFrom := 0
	for {
		startRel := strings.Index(xmlText[searchFrom:], "<interface ")
		if startRel < 0 {
			return "", false
		}
		start := searchFrom + startRel
		endRel := strings.Index(xmlText[start:], "</interface>")
		if endRel < 0 {
			return "", false
		}
		end := start + endRel + len("</interface>")
		block := xmlText[start:end]
		hasBridgeType := strings.Contains(block, "<interface type='bridge'") || strings.Contains(block, `<interface type="bridge"`)
		hasOVS := strings.Contains(block, "virtualport type='openvswitch'") || strings.Contains(block, `virtualport type="openvswitch"`)
		if hasBridgeType && hasOVS {
			return block, true
		}
		searchFrom = end
	}
}

func StripRuntimeOnlyInterfaceElements(block string) string {
	if strings.TrimSpace(block) == "" {
		return block
	}
	runtimeRe := regexp.MustCompile(`(?m)\n\s*<(target|alias|address)\b[^>]*/>`)
	return runtimeRe.ReplaceAllString(block, "")
}

func SafeVMXMLFileName(vmName string) string {
	value := regexp.MustCompile(`[^a-zA-Z0-9_.-]+`).ReplaceAllString(strings.TrimSpace(vmName), "_")
	if value == "" {
		return "vm"
	}
	return value
}
