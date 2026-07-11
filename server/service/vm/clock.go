package vm

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"qvmhub/utils"
)

const (
	VMRTCOffsetUTC       = "utc"
	VMRTCOffsetLocaltime = "localtime"
	VMRTCOffsetAbsolute  = "absolute"
	VMRTCStartDateNow    = "now"
)

var (
	clockOpenTagRegex = regexp.MustCompile(`<clock\b[^>]*>`)
	clockOffsetRegex  = regexp.MustCompile(`\boffset=['"][^'"]*['"]`)
	clockStartRegex   = regexp.MustCompile(`\bstart=['"][^'"]*['"]`)
)

// NormalizeRTCOffset 规范化 RTC 偏移值
func NormalizeRTCOffset(offset string) string {
	switch strings.ToLower(strings.TrimSpace(offset)) {
	case VMRTCOffsetUTC:
		return VMRTCOffsetUTC
	case VMRTCOffsetLocaltime:
		return VMRTCOffsetLocaltime
	case VMRTCOffsetAbsolute:
		return VMRTCOffsetAbsolute
	default:
		return ""
	}
}

// DefaultRTCOffsetForGuestType 根据系统类型返回推荐的 RTC 偏移值
func DefaultRTCOffsetForGuestType(guestType string) string {
	if strings.EqualFold(strings.TrimSpace(guestType), "windows") {
		return VMRTCOffsetLocaltime
	}
	return VMRTCOffsetUTC
}

// ResolveRTCOffset 解析最终使用的 RTC 偏移值
func ResolveRTCOffset(offset, guestType string) string {
	if normalized := NormalizeRTCOffset(offset); normalized != "" {
		return normalized
	}
	return DefaultRTCOffsetForGuestType(guestType)
}

// NormalizeRTCStartDate 规范化 RTC 起始日期
func NormalizeRTCStartDate(startDate string) string {
	normalized := strings.TrimSpace(startDate)
	if normalized == "" || strings.EqualFold(normalized, VMRTCStartDateNow) {
		return VMRTCStartDateNow
	}
	return normalized
}

// ParseRTCOffsetFromDomainXML 从 domain XML 中解析 RTC 偏移值
func ParseRTCOffsetFromDomainXML(xmlContent string) string {
	openTag := clockOpenTagRegex.FindString(xmlContent)
	if openTag == "" {
		return VMRTCOffsetUTC
	}
	match := clockOffsetRegex.FindString(openTag)
	if match == "" {
		return VMRTCOffsetUTC
	}
	parts := strings.SplitN(match, "=", 2)
	if len(parts) != 2 {
		return VMRTCOffsetUTC
	}
	value := strings.Trim(parts[1], `'"`)
	if normalized := NormalizeRTCOffset(value); normalized != "" {
		return normalized
	}
	return VMRTCOffsetUTC
}

// ParseRTCStartDateFromDomainXML 从 domain XML 中解析 RTC 起始日期
func ParseRTCStartDateFromDomainXML(xmlContent string) string {
	openTag := clockOpenTagRegex.FindString(xmlContent)
	if openTag == "" {
		return VMRTCStartDateNow
	}
	match := clockStartRegex.FindString(openTag)
	if match == "" {
		return VMRTCStartDateNow
	}
	parts := strings.SplitN(match, "=", 2)
	if len(parts) != 2 {
		return VMRTCStartDateNow
	}
	value := strings.Trim(parts[1], `'"`)
	if value == "" {
		return VMRTCStartDateNow
	}
	epoch, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return value
	}
	return time.Unix(epoch, 0).In(time.Local).Format("2006-01-02 15:04:05")
}

// ParseRTCStartDateToEpoch 将输入时间转换为 epoch 秒
func ParseRTCStartDateToEpoch(startDate string) (string, error) {
	normalized := NormalizeRTCStartDate(startDate)
	if normalized == VMRTCStartDateNow {
		return "", nil
	}
	if _, err := strconv.ParseInt(normalized, 10, 64); err == nil {
		return normalized, nil
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04",
		"2006-01-02",
	}

	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, normalized, time.Local)
		if err == nil {
			return strconv.FormatInt(t.Unix(), 10), nil
		}
	}

	return "", fmt.Errorf("RTC 开始日期格式无效，请使用 now、Unix 时间戳、RFC3339 或 YYYY-MM-DD HH:mm:ss")
}

func applyClockAttribute(openTag, name, value string) string {
	regex := clockOffsetRegex
	switch name {
	case "start":
		regex = clockStartRegex
	}
	attr := fmt.Sprintf("%s='%s'", name, value)
	if regex.MatchString(openTag) {
		return regex.ReplaceAllString(openTag, attr)
	}
	if strings.Contains(openTag, "<clock ") {
		return strings.Replace(openTag, "<clock ", "<clock "+attr+" ", 1)
	}
	return strings.Replace(openTag, "<clock", "<clock "+attr, 1)
}

func removeClockAttribute(openTag, name string) string {
	regex := clockOffsetRegex
	switch name {
	case "start":
		regex = clockStartRegex
	}
	updated := regex.ReplaceAllString(openTag, "")
	updated = regexp.MustCompile(`\s+>`).ReplaceAllString(updated, ">")
	return updated
}

// ApplyRTCConfigToDomainXML 将 RTC 配置写入 domain XML
func ApplyRTCConfigToDomainXML(xmlContent, offset, startDate, guestType string) (string, error) {
	resolvedStartDate := NormalizeRTCStartDate(startDate)
	resolvedOffset := ResolveRTCOffset(offset, guestType)
	if resolvedStartDate != VMRTCStartDateNow {
		epoch, err := ParseRTCStartDateToEpoch(resolvedStartDate)
		if err != nil {
			return "", err
		}
		resolvedOffset = VMRTCOffsetAbsolute
		openTag := clockOpenTagRegex.FindString(xmlContent)
		if openTag != "" {
			updatedTag := applyClockAttribute(openTag, "offset", resolvedOffset)
			updatedTag = applyClockAttribute(updatedTag, "start", epoch)
			return strings.Replace(xmlContent, openTag, updatedTag, 1), nil
		}

		clockXML := fmt.Sprintf("  <clock offset='%s' start='%s'/>\n", resolvedOffset, epoch)
		if strings.Contains(xmlContent, "<on_poweroff>") {
			return strings.Replace(xmlContent, "<on_poweroff>", clockXML+"  <on_poweroff>", 1), nil
		}
		if strings.Contains(xmlContent, "<devices>") {
			return strings.Replace(xmlContent, "<devices>", clockXML+"  <devices>", 1), nil
		}
		if strings.Contains(xmlContent, "</features>") {
			return strings.Replace(xmlContent, "</features>", "</features>\n"+clockXML, 1), nil
		}
		return xmlContent, nil
	}

	openTag := clockOpenTagRegex.FindString(xmlContent)
	if openTag != "" {
		updatedTag := applyClockAttribute(openTag, "offset", resolvedOffset)
		updatedTag = removeClockAttribute(updatedTag, "start")
		return strings.Replace(xmlContent, openTag, updatedTag, 1), nil
	}

	clockXML := fmt.Sprintf("  <clock offset='%s'/>\n", resolvedOffset)
	if strings.Contains(xmlContent, "<on_poweroff>") {
		return strings.Replace(xmlContent, "<on_poweroff>", clockXML+"  <on_poweroff>", 1), nil
	}
	if strings.Contains(xmlContent, "<devices>") {
		return strings.Replace(xmlContent, "<devices>", clockXML+"  <devices>", 1), nil
	}
	if strings.Contains(xmlContent, "</features>") {
		return strings.Replace(xmlContent, "</features>", "</features>\n"+clockXML, 1), nil
	}
	return xmlContent, nil
}

// SetVMRTCConfig 修改虚拟机 RTC 配置
func SetVMRTCConfig(name, offset, startDate string) error {
	xmlResult := utils.ExecCommand("virsh", "dumpxml", name, "--inactive")
	if xmlResult.Error != nil {
		return fmt.Errorf("获取虚拟机 XML 失败: %s", xmlResult.Stderr)
	}

	newXML, err := ApplyRTCConfigToDomainXML(xmlResult.Stdout, offset, startDate, "")
	if err != nil {
		return err
	}
	xmlPath := fmt.Sprintf("/tmp/_rtc-%s.xml", name)
	if err := os.WriteFile(xmlPath, []byte(newXML), 0644); err != nil {
		return fmt.Errorf("写入 RTC 配置文件失败: %w", err)
	}
	defer os.Remove(xmlPath)

	defineResult := utils.ExecCommand("virsh", "define", xmlPath)
	if defineResult.Error != nil {
		return fmt.Errorf("设置 RTC 偏移值失败: %s", defineResult.Stderr)
	}
	return nil
}
