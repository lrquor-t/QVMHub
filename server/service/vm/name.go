package vm

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	vmNameRegexp       = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)
	vmNamePrefixRegexp = regexp.MustCompile(`^[a-zA-Z0-9]+$`)
)

// ValidateVMName 校验虚拟机名称，允许字母、数字和短横线（不能以短横线开头或结尾）。
func ValidateVMName(name string) error {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return fmt.Errorf("虚拟机名称不能为空")
	}
	if !vmNameRegexp.MatchString(trimmedName) {
		return fmt.Errorf("虚拟机名称只能包含字母、数字和短横线，且不能以短横线开头或结尾")
	}
	return nil
}

// ValidateVMNamePrefix 校验批量克隆名称前缀，仅允许字母和数字。
func ValidateVMNamePrefix(prefix string) error {
	trimmedPrefix := strings.TrimSpace(prefix)
	if trimmedPrefix == "" {
		return fmt.Errorf("虚拟机前缀不能为空")
	}
	if !vmNamePrefixRegexp.MatchString(trimmedPrefix) {
		return fmt.Errorf("虚拟机前缀只能包含字母和数字")
	}
	return nil
}

// GenerateRandomVMName 生成默认虚拟机名称。
func GenerateRandomVMName() string {
	return "vm" + D.RandomStringFromCharset("abcdefghijklmnopqrstuvwxyz0123456789", 8)
}
