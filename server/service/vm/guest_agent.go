package vm

import (
	"fmt"
	"os"

	"qvmhub/service/vm_xml"
	"qvmhub/utils"
)

// SetVMGuestAgentConfig 修改虚拟机 QEMU Guest Agent 配置
func SetVMGuestAgentConfig(name string, cfg *vm_xml.VMGuestAgentConfig) error {
	xmlResult := utils.ExecCommand("virsh", "dumpxml", name, "--inactive")
	if xmlResult.Error != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %s", xmlResult.Stderr)
	}

	newXML, err := vm_xml.ApplyVMGuestAgentConfigToDomainXML(xmlResult.Stdout, cfg)
	if err != nil {
		return err
	}

	xmlPath := fmt.Sprintf("/tmp/_guest-agent-%s.xml", name)
	if err := os.WriteFile(xmlPath, []byte(newXML), 0644); err != nil {
		return fmt.Errorf("写入 QEMU Guest Agent 配置文件失败: %w", err)
	}
	defer os.Remove(xmlPath)

	defineResult := utils.ExecCommand("virsh", "define", xmlPath)
	if defineResult.Error != nil {
		return fmt.Errorf("设置 QEMU Guest Agent 配置失败: %s", defineResult.Stderr)
	}

	return nil
}
