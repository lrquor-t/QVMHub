package template

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// generateTemplateID generates a unique ID with the given prefix.
func generateTemplateID(prefix string) string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	}
	return prefix + "_" + hex.EncodeToString(buf[:])
}

// ValidateTemplateName validates a template name for file system safety.
func ValidateTemplateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("模板名称不能为空")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("模板名称不能包含连续的点")
	}
	if !templateNamePatternForTransfer.MatchString(name) {
		return fmt.Errorf("模板名称只能包含字母、数字、点、下划线和短横线")
	}
	return nil
}
