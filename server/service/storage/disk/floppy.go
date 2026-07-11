package disk

import (
	"fmt"
	"strings"

	"qvmhub/service/libvirt_rpc"

	"github.com/digitalocean/go-libvirt"
)

// ChangeFloppy changes/inserts a floppy disk image.
// forceNew: when true, force-add a new floppy device instead of replacing existing.
func ChangeFloppy(vmName, imagePath, device string, forceNew bool) error {
	if err := EnsureNotMigrating(vmName, "更换软盘"); err != nil {
		return err
	}
	vmState, _ := libvirt_rpc.GetDomainStateRPC(vmName)

	// force new mode: add a new floppy device directly
	if forceNew {
		return attachNewFloppy(vmName, imagePath, vmState)
	}

	if device == "" {
		// auto-find floppy device
		device = findFloppyDevice(vmName)
		if device == "" {
			// no existing floppy device, add a new one
			return attachNewFloppy(vmName, imagePath, vmState)
		}
	}

	// change floppy media via libvirt_rpc.AttachDeviceFlagsRPC
	floppyXML := fmt.Sprintf(
		"<disk type='file' device='floppy'>\n"+
			"  <driver name='qemu' type='raw'/>\n"+
			"  <source file='%s'/>\n"+
			"  <target dev='%s' bus='fdc'/>\n"+
			"</disk>", imagePath, device)

	var attachFlags uint32 = 2 // VIR_DOMAIN_DEVICE_MODIFY_CONFIG
	if vmState == "running" {
		attachFlags = 3 // VIR_DOMAIN_DEVICE_MODIFY_LIVE | VIR_DOMAIN_DEVICE_MODIFY_CONFIG
	}
	if err := libvirt_rpc.AttachDeviceFlagsRPC(vmName, floppyXML, attachFlags); err != nil {
		return fmt.Errorf("插入软盘失败: %w", err)
	}
	return nil
}

// EjectFloppy ejects a floppy disk (keeps the device).
func EjectFloppy(vmName, device string) error {
	if err := EnsureNotMigrating(vmName, "弹出软盘"); err != nil {
		return err
	}
	vmState, _ := libvirt_rpc.GetDomainStateRPC(vmName)

	if device == "" {
		device = findFloppyDevice(vmName)
		if device == "" {
			return fmt.Errorf("未找到软盘设备")
		}
	}

	// eject floppy media (leave source empty)
	floppyXML := fmt.Sprintf(
		"<disk type='file' device='floppy'>\n"+
			"  <driver name='qemu' type='raw'/>\n"+
			"  <target dev='%s' bus='fdc'/>\n"+
			"</disk>", device)

	var attachFlags uint32 = 2 // VIR_DOMAIN_DEVICE_MODIFY_CONFIG
	if vmState == "running" {
		attachFlags = 3 // VIR_DOMAIN_DEVICE_MODIFY_LIVE | VIR_DOMAIN_DEVICE_MODIFY_CONFIG
	}
	if err := libvirt_rpc.AttachDeviceFlagsRPC(vmName, floppyXML, attachFlags); err != nil {
		// ignore "no media" errors
		if strings.Contains(err.Error(), "doesn't have media") ||
			strings.Contains(err.Error(), "is not removable") {
			return nil
		}
		return fmt.Errorf("弹出软盘失败: %w", err)
	}
	return nil
}

// RemoveFloppy completely removes a floppy device by editing XML.
func RemoveFloppy(vmName, device string) error {
	if err := EnsureNotMigrating(vmName, "移除软盘"); err != nil {
		return err
	}
	vmState, _ := libvirt_rpc.GetDomainStateRPC(vmName)

	if device == "" {
		device = findFloppyDevice(vmName)
		if device == "" {
			return fmt.Errorf("未找到软盘设备")
		}
	}

	// running VMs cannot directly detach floppy device, must edit XML
	if vmState == "running" {
		// eject media first
		EjectFloppy(vmName, device)
	}

	// get current XML
	xmlResult, err := libvirt_rpc.GetDomainXMLRPC(vmName, libvirt.DomainXMLInactive)
	if err != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %w", err)
	}

	// remove the floppy disk node from XML
	xmlStr := xmlResult
	lines := strings.Split(xmlStr, "\n")
	var newLines []string
	var diskBuffer []string
	inFloppyDisk := false
	isFloppy := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// detect floppy disk block start
		if strings.Contains(trimmed, "<disk ") && strings.Contains(trimmed, "device='floppy'") {
			inFloppyDisk = true
			isFloppy = false
			diskBuffer = []string{line}
			continue
		}

		if inFloppyDisk {
			diskBuffer = append(diskBuffer, line)
			// check if it contains target device name
			if strings.Contains(trimmed, "<target") && strings.Contains(trimmed, "dev='"+device+"'") {
				isFloppy = true
			}
			// if reached </disk>, decide whether to keep
			if strings.Contains(trimmed, "</disk>") {
				inFloppyDisk = false
				if isFloppy {
					// this is the floppy node to delete, discard buffer
				} else {
					// keep this disk node
					newLines = append(newLines, diskBuffer...)
				}
				diskBuffer = nil
			}
			continue
		}

		newLines = append(newLines, line)
	}

	newXML := strings.Join(newLines, "\n")
	if _, err := libvirt_rpc.DefineDomainXMLRPC(newXML); err != nil {
		return fmt.Errorf("移除软盘失败: %w", err)
	}

	return nil
}

// findFloppyDevice finds the first floppy device name of a VM.
func findFloppyDevice(vmName string) string {
	devices := findAllFloppyDevices(vmName)
	if len(devices) > 0 {
		return devices[0]
	}
	return ""
}

// findAllFloppyDevices finds all floppy device names of a VM.
func findAllFloppyDevices(vmName string) []string {
	xmlStr, err := libvirt_rpc.GetDomainXMLRPC(vmName, 0)
	if err != nil {
		return nil
	}

	var devices []string
	lines := strings.Split(xmlStr, "\n")
	inFloppy := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "device='floppy'") {
			inFloppy = true
		}
		if inFloppy && strings.Contains(trimmed, "<target") {
			parts := strings.Split(trimmed, "dev='")
			if len(parts) > 1 {
				dev := strings.Split(parts[1], "'")[0]
				devices = append(devices, dev)
			}
		}
		if inFloppy && strings.Contains(trimmed, "</disk>") {
			inFloppy = false
		}
	}
	return devices
}

// attachNewFloppy attaches a new floppy device.
func attachNewFloppy(vmName, imagePath, vmState string) error {
	existingDisks, _ := ListDisks(vmName)

	usedDevs := make(map[string]bool)
	for _, d := range existingDisks {
		usedDevs[d.Device] = true
	}

	// floppy uses 'fdc' bus, device names are 'fda', 'fdb'
	nextDev := ""
	for _, letter := range "ab" {
		dev := "fd" + string(letter)
		if !usedDevs[dev] {
			nextDev = dev
			break
		}
	}
	if nextDev == "" {
		return fmt.Errorf("没有可用的软盘设备名")
	}

	// add floppy device via libvirt_rpc.AttachDeviceFlagsRPC
	floppyXML := fmt.Sprintf(
		"<disk type='file' device='floppy'>\n"+
			"  <driver name='qemu' type='raw'/>\n"+
			"  <source file='%s'/>\n"+
			"  <target dev='%s' bus='fdc'/>\n"+
			"  <readonly/>\n"+
			"</disk>", imagePath, nextDev)

	var attachFlags uint32 = 2 // VIR_DOMAIN_DEVICE_MODIFY_CONFIG
	if vmState == "running" {
		attachFlags = 3 // VIR_DOMAIN_DEVICE_MODIFY_LIVE | VIR_DOMAIN_DEVICE_MODIFY_CONFIG
	}
	if err := libvirt_rpc.AttachDeviceFlagsRPC(vmName, floppyXML, attachFlags); err != nil {
		return fmt.Errorf("添加软盘失败: %w", err)
	}
	return nil
}
