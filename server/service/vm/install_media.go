package vm

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"

	"qvmhub/service/arch"
)

var (
	vmDiskTargetRegex = regexp.MustCompile(`<target\b[^>]*dev=['"]([^'"]+)['"][^>]*bus=['"]([^'"]+)['"][^>]*/?>`)
	vmCDROMBlockRegex = regexp.MustCompile(`(?s)<disk\b[^>]*device=['"]cdrom['"][^>]*>.*?</disk>`)
)

type vmInstallMediaDisk struct {
	XMLName  xml.Name                    `xml:"disk"`
	Type     string                      `xml:"type,attr"`
	Device   string                      `xml:"device,attr"`
	Driver   *vmInstallMediaDiskDriver   `xml:"driver,omitempty"`
	Source   *vmInstallMediaDiskSource   `xml:"source,omitempty"`
	Target   *vmInstallMediaDiskTarget   `xml:"target,omitempty"`
	ReadOnly *vmInstallMediaDiskReadOnly `xml:"readonly,omitempty"`
}

type vmInstallMediaDiskDriver struct {
	Name string `xml:"name,attr,omitempty"`
	Type string `xml:"type,attr,omitempty"`
}

type vmInstallMediaDiskSource struct {
	File string `xml:"file,attr"`
}

type vmInstallMediaDiskTarget struct {
	Dev string `xml:"dev,attr"`
	Bus string `xml:"bus,attr"`
}

type vmInstallMediaDiskReadOnly struct{}

// NormalizeInstallISOSelection 规范化创建时选择的 ISO 列表。
func NormalizeInstallISOSelection(primary string, paths []string) (string, []string) {
	seen := make(map[string]struct{}, len(paths)+1)
	ordered := make([]string, 0, len(paths)+1)
	appendPath := func(path string) {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			return
		}
		if _, exists := seen[trimmed]; exists {
			return
		}
		seen[trimmed] = struct{}{}
		ordered = append(ordered, trimmed)
	}

	appendPath(primary)
	for _, path := range paths {
		appendPath(path)
	}

	if len(ordered) == 0 {
		return "", nil
	}
	if len(ordered) == 1 {
		return ordered[0], nil
	}
	return ordered[0], ordered[1:]
}

// ApplyAdditionalCDROMsToDomainXML 将额外 ISO 以只读光驱形式写入 domain XML。
func ApplyAdditionalCDROMsToDomainXML(xmlContent string, isoPaths []string) (string, error) {
	if len(isoPaths) == 0 {
		return xmlContent, nil
	}
	if !strings.Contains(xmlContent, "</devices>") {
		return "", fmt.Errorf("写入额外 ISO 失败：未找到 devices 节点")
	}

	cdromBus := detectPrimaryCDROMBus(xmlContent)
	updatedXML := xmlContent
	for _, isoPath := range isoPaths {
		trimmed := strings.TrimSpace(isoPath)
		if trimmed == "" {
			continue
		}
		nextDev, err := nextAvailableInstallMediaDevice(updatedXML, cdromBus)
		if err != nil {
			return "", err
		}
		diskXML, err := buildInstallMediaDiskXML(trimmed, nextDev, cdromBus)
		if err != nil {
			return "", err
		}
		updatedXML = strings.Replace(updatedXML, "</devices>", diskXML+"\n  </devices>", 1)
	}

	return updatedXML, nil
}

func detectPrimaryCDROMBus(xmlContent string) string {
	block := vmCDROMBlockRegex.FindString(xmlContent)
	if block == "" {
		return arch.GetProfile(arch.DetectHostArch()).GetCDROMBus()
	}
	matches := vmDiskTargetRegex.FindStringSubmatch(block)
	if len(matches) >= 3 && strings.TrimSpace(matches[2]) != "" {
		return strings.TrimSpace(matches[2])
	}
	return arch.GetProfile(arch.DetectHostArch()).GetCDROMBus()
}

func nextAvailableInstallMediaDevice(xmlContent, bus string) (string, error) {
	usedDevices := make(map[string]bool)
	matches := vmDiskTargetRegex.FindAllStringSubmatch(xmlContent, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		dev := strings.TrimSpace(match[1])
		if dev != "" {
			usedDevices[dev] = true
		}
	}

	prefix := D.GetDevPrefix(bus)
	for _, letter := range "abcdefghijklmnop" {
		candidate := prefix + string(letter)
		if !usedDevices[candidate] {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("没有可用的 %s 光驱设备名", strings.ToUpper(bus))
}

func buildInstallMediaDiskXML(isoPath, dev, bus string) (string, error) {
	disk := vmInstallMediaDisk{
		Type:   "file",
		Device: "cdrom",
		Driver: &vmInstallMediaDiskDriver{
			Name: "qemu",
			Type: "raw",
		},
		Source: &vmInstallMediaDiskSource{
			File: isoPath,
		},
		Target: &vmInstallMediaDiskTarget{
			Dev: dev,
			Bus: bus,
		},
		ReadOnly: &vmInstallMediaDiskReadOnly{},
	}

	raw, err := xml.MarshalIndent(disk, "    ", "  ")
	if err != nil {
		return "", fmt.Errorf("构造光驱 XML 失败: %w", err)
	}
	return string(raw), nil
}
