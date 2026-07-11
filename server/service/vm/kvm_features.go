package vm

import (
	"fmt"
	"os"

	"qvmhub/service/vm_xml"
	"qvmhub/utils"
)

// SetVMKVMHidden 修改虚拟机的 KVM 隐藏标志配置。
func SetVMKVMHidden(name string, enabled bool) error {
	xmlResult := utils.ExecCommand("virsh", "dumpxml", name, "--inactive")
	if xmlResult.Error != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %s", xmlResult.Stderr)
	}

	newXML, err := vm_xml.ApplyKVMHiddenToDomainXML(xmlResult.Stdout, &enabled)
	if err != nil {
		return err
	}

	xmlPath := fmt.Sprintf("/tmp/_kvm-hidden-%s.xml", name)
	if err := os.WriteFile(xmlPath, []byte(newXML), 0644); err != nil {
		return fmt.Errorf("写入 KVM 隐藏配置失败: %w", err)
	}
	defer os.Remove(xmlPath)

	defineResult := utils.ExecCommand("virsh", "define", xmlPath)
	if defineResult.Error != nil {
		return fmt.Errorf("设置 KVM 隐藏配置失败: %s", defineResult.Stderr)
	}

	return nil
}

// SetVMNestedVirt 修改虚拟机的嵌套虚拟化配置。
// enabled 为 true 时注入 policy='require'，false 时注入 policy='disable' 覆盖 host-passthrough。
func SetVMNestedVirt(name string, enabled bool) error {
	xmlResult := utils.ExecCommand("virsh", "dumpxml", name, "--inactive")
	if xmlResult.Error != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %s", xmlResult.Stderr)
	}

	// 无论启用还是关闭，都需要检测 feature name 以注入 require/disable
	featureName := vm_xml.DetectHostNestedVirtFeatureName()

	newXML, err := vm_xml.ApplyNestedVirtToDomainXML(xmlResult.Stdout, &enabled, featureName)
	if err != nil {
		return err
	}

	xmlPath := fmt.Sprintf("/tmp/_nested-virt-%s.xml", name)
	if err := os.WriteFile(xmlPath, []byte(newXML), 0644); err != nil {
		return fmt.Errorf("写入嵌套虚拟化配置失败: %w", err)
	}
	defer os.Remove(xmlPath)

	defineResult := utils.ExecCommand("virsh", "define", xmlPath)
	if defineResult.Error != nil {
		return fmt.Errorf("设置嵌套虚拟化配置失败: %s", defineResult.Stderr)
	}

	return nil
}

// SetVMVendorID 修改虚拟机的 Hyper-V vendor_id 伪装配置。
// vendorID 为空字符串时清除 vendor_id。
func SetVMVendorID(name string, vendorID string) error {
	xmlResult := utils.ExecCommand("virsh", "dumpxml", name, "--inactive")
	if xmlResult.Error != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %s", xmlResult.Stderr)
	}

	newXML, err := vm_xml.ApplyVendorIDToHyperVBlock(xmlResult.Stdout, vendorID)
	if err != nil {
		return err
	}

	xmlPath := fmt.Sprintf("/tmp/_vendor-id-%s.xml", name)
	if err := os.WriteFile(xmlPath, []byte(newXML), 0644); err != nil {
		return fmt.Errorf("写入 vendor_id 配置失败: %w", err)
	}
	defer os.Remove(xmlPath)

	defineResult := utils.ExecCommand("virsh", "define", xmlPath)
	if defineResult.Error != nil {
		return fmt.Errorf("设置 vendor_id 配置失败: %s", defineResult.Stderr)
	}

	return nil
}
