package vm_xml

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

type VMSMBIOS1Config struct {
	Base64       bool   `json:"base64,omitempty"`
	Family       string `json:"family,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Product      string `json:"product,omitempty"`
	Serial       string `json:"serial,omitempty"`
	SKU          string `json:"sku,omitempty"`
	UUID         string `json:"uuid,omitempty"`
	Version      string `json:"version,omitempty"`
}

type domainSMBIOSXML struct {
	XMLName xml.Name           `xml:"domain"`
	Sysinfo []domainSysinfoXML `xml:"sysinfo"`
}

type domainSysinfoXML struct {
	Type   string              `xml:"type,attr"`
	System *domainSMBIOSSystem `xml:"system"`
}

type domainSMBIOSSystem struct {
	Entries []domainSMBIOSEntry `xml:"entry"`
}

type domainSMBIOSEntry struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

var (
	domainUUIDElementRegex = regexp.MustCompile(`(?s)<uuid>[^<]*</uuid>`)
	smbiosSysinfoRegex     = regexp.MustCompile(`(?s)\n?\s*<sysinfo type=['"]smbios['"]>.*?</sysinfo>`)
	osSMBIOSModeRegex      = regexp.MustCompile(`(?s)\n?\s*<smbios\b[^>]*/>`)
	smbiosUUIDPattern      = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

func (cfg *VMSMBIOS1Config) HasValue() bool {
	if cfg == nil {
		return false
	}
	return strings.TrimSpace(cfg.Family) != "" ||
		strings.TrimSpace(cfg.Manufacturer) != "" ||
		strings.TrimSpace(cfg.Product) != "" ||
		strings.TrimSpace(cfg.Serial) != "" ||
		strings.TrimSpace(cfg.SKU) != "" ||
		strings.TrimSpace(cfg.UUID) != "" ||
		strings.TrimSpace(cfg.Version) != ""
}

func cloneSMBIOS1Config(cfg *VMSMBIOS1Config) *VMSMBIOS1Config {
	if cfg == nil {
		return nil
	}
	return &VMSMBIOS1Config{
		Base64:       cfg.Base64,
		Family:       cfg.Family,
		Manufacturer: cfg.Manufacturer,
		Product:      cfg.Product,
		Serial:       cfg.Serial,
		SKU:          cfg.SKU,
		UUID:         cfg.UUID,
		Version:      cfg.Version,
	}
}

func decodeSMBIOS1Field(value, fieldName string, useBase64 bool) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if !useBase64 {
		return trimmed, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return "", fmt.Errorf("SMBIOS %s 的 Base64 值无效", fieldName)
	}
	return strings.TrimSpace(string(decoded)), nil
}

// NormalizeSMBIOS1Config 规范化 SMBIOS1 配置。
func NormalizeSMBIOS1Config(cfg *VMSMBIOS1Config) (*VMSMBIOS1Config, error) {
	normalized := cloneSMBIOS1Config(cfg)
	if normalized == nil {
		return &VMSMBIOS1Config{}, nil
	}

	var err error
	if normalized.Family, err = decodeSMBIOS1Field(normalized.Family, "家族名称", normalized.Base64); err != nil {
		return nil, err
	}
	if normalized.Manufacturer, err = decodeSMBIOS1Field(normalized.Manufacturer, "厂商名", normalized.Base64); err != nil {
		return nil, err
	}
	if normalized.Product, err = decodeSMBIOS1Field(normalized.Product, "产品 ID", normalized.Base64); err != nil {
		return nil, err
	}
	if normalized.Serial, err = decodeSMBIOS1Field(normalized.Serial, "序列号", normalized.Base64); err != nil {
		return nil, err
	}
	if normalized.SKU, err = decodeSMBIOS1Field(normalized.SKU, "SKU", normalized.Base64); err != nil {
		return nil, err
	}
	if normalized.UUID, err = decodeSMBIOS1Field(normalized.UUID, "UUID", normalized.Base64); err != nil {
		return nil, err
	}
	if normalized.Version, err = decodeSMBIOS1Field(normalized.Version, "版本", normalized.Base64); err != nil {
		return nil, err
	}

	normalized.Base64 = normalized.Base64 && normalized.HasValue()
	if normalized.UUID != "" && !smbiosUUIDPattern.MatchString(normalized.UUID) {
		return nil, fmt.Errorf("SMBIOS UUID 格式无效，请使用标准 UUID")
	}

	return normalized, nil
}

// ParseDomainUUIDFromXML 从 domain XML 中解析 UUID。
func ParseDomainUUIDFromXML(xmlContent string) string {
	match := domainUUIDElementRegex.FindString(xmlContent)
	if match == "" {
		return ""
	}
	match = strings.TrimPrefix(match, "<uuid>")
	match = strings.TrimSuffix(match, "</uuid>")
	return strings.TrimSpace(match)
}

// ParseSMBIOS1ConfigFromDomainXML 从 domain XML 中解析 SMBIOS1 配置。
func ParseSMBIOS1ConfigFromDomainXML(xmlContent string) *VMSMBIOS1Config {
	var doc domainSMBIOSXML
	if err := xml.Unmarshal([]byte(xmlContent), &doc); err != nil {
		return nil
	}

	for _, sysinfo := range doc.Sysinfo {
		if strings.ToLower(strings.TrimSpace(sysinfo.Type)) != "smbios" || sysinfo.System == nil {
			continue
		}

		cfg := &VMSMBIOS1Config{}
		for _, entry := range sysinfo.System.Entries {
			value := strings.TrimSpace(entry.Value)
			switch strings.ToLower(strings.TrimSpace(entry.Name)) {
			case "family":
				cfg.Family = value
			case "manufacturer":
				cfg.Manufacturer = value
			case "product":
				cfg.Product = value
			case "serial":
				cfg.Serial = value
			case "sku":
				cfg.SKU = value
			case "uuid":
				cfg.UUID = value
			case "version":
				cfg.Version = value
			}
		}
		if cfg.HasValue() {
			return cfg
		}
	}

	return nil
}

func escapeSMBIOSValue(value string) string {
	var builder strings.Builder
	_ = xml.EscapeText(&builder, []byte(value))
	return builder.String()
}

func buildSMBIOS1SystemXML(cfg *VMSMBIOS1Config) string {
	var entries []string
	appendEntry := func(name, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		entries = append(entries, fmt.Sprintf("      <entry name='%s'>%s</entry>", name, escapeSMBIOSValue(value)))
	}

	appendEntry("manufacturer", cfg.Manufacturer)
	appendEntry("product", cfg.Product)
	appendEntry("version", cfg.Version)
	appendEntry("serial", cfg.Serial)
	appendEntry("uuid", cfg.UUID)
	appendEntry("sku", cfg.SKU)
	appendEntry("family", cfg.Family)

	if len(entries) == 0 {
		return ""
	}

	return "  <sysinfo type='smbios'>\n    <system>\n" + strings.Join(entries, "\n") + "\n    </system>\n  </sysinfo>\n"
}

func upsertDomainUUID(xmlContent, uuid string) string {
	if strings.TrimSpace(uuid) == "" {
		return xmlContent
	}
	uuidXML := fmt.Sprintf("<uuid>%s</uuid>", escapeSMBIOSValue(uuid))
	if domainUUIDElementRegex.MatchString(xmlContent) {
		return domainUUIDElementRegex.ReplaceAllString(xmlContent, uuidXML)
	}
	if strings.Contains(xmlContent, "</name>") {
		return strings.Replace(xmlContent, "</name>", "</name>\n  "+uuidXML, 1)
	}
	return xmlContent
}

func removeSMBIOS1ConfigFromDomainXML(xmlContent string) string {
	updated := smbiosSysinfoRegex.ReplaceAllString(xmlContent, "")
	updated = osSMBIOSModeRegex.ReplaceAllString(updated, "")
	return updated
}

func insertSMBIOS1ConfigIntoDomainXML(xmlContent, sysinfoXML string) string {
	updated := xmlContent
	if sysinfoXML != "" {
		switch {
		case strings.Contains(updated, "</vcpu>"):
			updated = strings.Replace(updated, "</vcpu>", "</vcpu>\n"+strings.TrimRight(sysinfoXML, "\n"), 1)
		case strings.Contains(updated, "<os"):
			updated = strings.Replace(updated, "<os", strings.TrimRight(sysinfoXML, "\n")+"\n  <os", 1)
		case strings.Contains(updated, "</name>"):
			updated = strings.Replace(updated, "</name>", "</name>\n"+strings.TrimRight(sysinfoXML, "\n"), 1)
		}
	}

	osStart := strings.Index(updated, "<os")
	if osStart == -1 {
		return updated
	}
	osCloseRelative := strings.Index(updated[osStart:], "</os>")
	if osCloseRelative == -1 {
		return updated
	}

	osClose := osStart + osCloseRelative
	return updated[:osClose] + "    <smbios mode='sysinfo'/>\n  " + updated[osClose:]
}

// ApplySMBIOS1ConfigToDomainXML 将 SMBIOS1 配置写入 domain XML。
func ApplySMBIOS1ConfigToDomainXML(xmlContent string, cfg *VMSMBIOS1Config, syncDomainUUID bool) (string, error) {
	normalized, err := NormalizeSMBIOS1Config(cfg)
	if err != nil {
		return "", err
	}

	cleanedXML := removeSMBIOS1ConfigFromDomainXML(xmlContent)
	currentUUID := ParseDomainUUIDFromXML(cleanedXML)

	if normalized.UUID != "" {
		if syncDomainUUID {
			cleanedXML = upsertDomainUUID(cleanedXML, normalized.UUID)
		} else if currentUUID != "" && !strings.EqualFold(currentUUID, normalized.UUID) {
			return "", fmt.Errorf("当前虚拟机的 SMBIOS UUID 必须与虚拟机 UUID 保持一致，已存在的虚拟机请保持当前 UUID 不变")
		}
	}

	if !normalized.HasValue() {
		return cleanedXML, nil
	}

	return insertSMBIOS1ConfigIntoDomainXML(cleanedXML, buildSMBIOS1SystemXML(normalized)), nil
}
