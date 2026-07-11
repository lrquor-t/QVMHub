package vm_xml

import (
	"encoding/xml"
	"regexp"
	"strings"
)

var VMXMLTempNameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type VMXMLDomainEnvelope struct {
	XMLName xml.Name `xml:"domain"`
	Name    string   `xml:"name"`
}

// NormalizeDomainXMLForEdit 将 domain XML 中的 \r\n 替换为 \n。
func NormalizeDomainXMLForEdit(xmlContent string) string {
	return strings.ReplaceAll(xmlContent, "\r\n", "\n")
}
