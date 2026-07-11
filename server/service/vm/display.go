package vm

import (
	"fmt"
	"os"
	"strings"

	"qvmhub/service/vm_xml"
	"qvmhub/utils"
)

// SetVMVideoModel 设置虚拟机视频模型。运行中的虚拟机需要先关机后再修改。
func SetVMVideoModel(name, videoModel string) error {
	stateResult := utils.ExecCommand("virsh", "domstate", name)
	if stateResult.Error != nil {
		return fmt.Errorf("获取虚拟机状态失败: %s", stateResult.Stderr)
	}
	state := strings.TrimSpace(stateResult.Stdout)
	if state == "running" || state == "paused" {
		return fmt.Errorf("请先关机后再修改显示设备")
	}

	xmlResult := utils.ExecCommand("virsh", "dumpxml", name, "--inactive")
	if xmlResult.Error != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %s", xmlResult.Stderr)
	}

	xmlStr := xmlResult.Stdout
	osType := DetectVMOSType("", xmlStr)
	xmlStr = vm_xml.ApplyVMVideoModelToDomainXML(xmlStr, videoModel, osType)
	if osType == "windows" {
		xmlStr = vm_xml.ApplyWindowsGuestOptimizationsToDomainXML(xmlStr)
	}

	xmlPath := fmt.Sprintf("/tmp/_video-%s.xml", name)
	if err := os.WriteFile(xmlPath, []byte(xmlStr), 0644); err != nil {
		return fmt.Errorf("写入显示设备 XML 失败: %v", err)
	}
	defer os.Remove(xmlPath)
	defineResult := utils.ExecCommand("virsh", "define", xmlPath)
	if defineResult.Error != nil {
		return fmt.Errorf("修改显示设备失败: %s", defineResult.Stderr)
	}
	return nil
}
