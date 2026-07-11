package arch

import (
	"encoding/xml"
)

// ==================== Profile 注册表 ====================

var profiles = map[string]ArchProfile{}

// RegisterProfile 将 profile 注册到全局注册表
func RegisterProfile(p ArchProfile) {
	profiles[p.Arch()] = p
}

// GetProfile 查找指定架构的 Profile；未找到时回退到 x86_64
func GetProfile(arch string) ArchProfile {
	if p, ok := profiles[arch]; ok {
		return p
	}
	return profiles[ArchX8664]
}

// GetProfileByXML 从 domain XML 内容解析架构后查找 Profile
func GetProfileByXML(xmlContent string) ArchProfile {
	var dom struct {
		OS struct {
			Type struct {
				Arch string `xml:"arch,attr"`
			} `xml:"type"`
		} `xml:"os"`
	}
	if err := xml.Unmarshal([]byte(xmlContent), &dom); err == nil {
		return GetProfile(dom.OS.Type.Arch)
	}
	return GetProfile(ArchX8664)
}

// SupportedArchs 返回所有已注册架构标识
func SupportedArchs() []string {
	keys := make([]string, 0, len(profiles))
	for k := range profiles {
		keys = append(keys, k)
	}
	return keys
}
