package vm

import (
	"fmt"
	"os"

	"qvmhub/service/vm_xml"
	"qvmhub/utils"
)

// SetVMSMBIOS1Config 设置虚拟机 SMBIOS1 配置。
func SetVMSMBIOS1Config(name string, cfg *vm_xml.VMSMBIOS1Config) error {
	xmlResult := utils.ExecCommand("virsh", "dumpxml", name, "--inactive")
	if xmlResult.Error != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %s", xmlResult.Stderr)
	}

	newXML, err := vm_xml.ApplySMBIOS1ConfigToDomainXML(xmlResult.Stdout, cfg, false)
	if err != nil {
		return err
	}

	xmlPath := fmt.Sprintf("/tmp/_smbios1-%s.xml", name)
	if err := os.WriteFile(xmlPath, []byte(newXML), 0644); err != nil {
		return fmt.Errorf("写入 SMBIOS 配置文件失败: %w", err)
	}
	defer os.Remove(xmlPath)

	defineResult := utils.ExecCommand("virsh", "define", xmlPath)
	if defineResult.Error != nil {
		return fmt.Errorf("设置 SMBIOS 配置失败: %s", defineResult.Stderr)
	}
	return nil
}
