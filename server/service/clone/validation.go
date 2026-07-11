package clone

import (
	crand "crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"qvmhub/service/storage/disk"
	"qvmhub/service/vm_xml"
	"qvmhub/utils"
)

const strongPasswordMinLength = 12
const windowsCloneDefaultUsername = "administrator"

var (
	cloneHostnameRegexp       = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)
	CloneUsernameRegexp       = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	cloneUsernameRegexp       = CloneUsernameRegexp
	clonePasswordAllowedRegex = regexp.MustCompile(`^[A-Za-z0-9!@#$%^&*_\-+=?]+$`)
)

// ValidateCloneCredentials 校验模板克隆使用的主机名、用户名和密码
func ValidateCloneCredentials(hostname, username, password string, requireCredentials bool) error {
	trimmedHostname := strings.TrimSpace(hostname)
	if trimmedHostname != "" && !cloneHostnameRegexp.MatchString(trimmedHostname) {
		return fmt.Errorf("主机名只能包含字母、数字和短横线，且不能以短横线开头或结尾")
	}

	trimmedUsername := strings.TrimSpace(username)
	if requireCredentials && trimmedUsername == "" {
		return fmt.Errorf("请输入用户名")
	}
	if trimmedUsername != "" && !cloneUsernameRegexp.MatchString(trimmedUsername) {
		return fmt.Errorf("用户名只能以小写字母或下划线开头，且只能包含小写字母、数字、下划线和短横线")
	}

	if requireCredentials && password == "" {
		return fmt.Errorf("请输入密码")
	}
	if password != "" {
		return ValidateStrongPassword(password)
	}

	return nil
}

// NormalizeCloneUsernameForTemplate 根据模板类型补全默认用户名
func NormalizeCloneUsernameForTemplate(templateType, username string) string {
	trimmedTemplateType := strings.ToLower(strings.TrimSpace(templateType))
	trimmedUsername := strings.TrimSpace(username)
	if trimmedTemplateType == "windows" && trimmedUsername == "" {
		return windowsCloneDefaultUsername
	}
	if trimmedTemplateType == "openwrt" {
		return "root"
	}
	return trimmedUsername
}

// ValidateCloneCredentialsForTemplate 校验模板克隆使用的主机名、用户名和密码
func ValidateCloneCredentialsForTemplate(templateType, hostname, username, password string, requireCredentials bool) error {
	trimmedTemplateType := strings.ToLower(strings.TrimSpace(templateType))
	// OpenWrt 模板不需要用户名/密码校验（只需 root 密码可选 + 静态 IP）
	if trimmedTemplateType == "openwrt" {
		trimmedHostname := strings.TrimSpace(hostname)
		if trimmedHostname != "" && !cloneHostnameRegexp.MatchString(trimmedHostname) {
			return fmt.Errorf("主机名只能包含字母、数字和短横线，且不能以短横线开头或结尾")
		}
		if password != "" {
			return ValidateStrongPassword(password)
		}
		return nil
	}
	normalizedUsername := NormalizeCloneUsernameForTemplate(trimmedTemplateType, username)
	if trimmedTemplateType == "windows" && normalizedUsername != windowsCloneDefaultUsername {
		return fmt.Errorf("Windows 模板用户名固定为 administrator，不支持修改")
	}
	return ValidateCloneCredentials(hostname, normalizedUsername, password, requireCredentials)
}

// ValidateStrongPassword 统一密码强度校验（当前暂不校验，后续完善）
func ValidateStrongPassword(password string) error {
	return nil
}

// GenerateRandomCloneHostname 生成默认主机名
func GenerateRandomCloneHostname() string {
	return fmt.Sprintf("vm-%s", randomStringFromCharset("abcdefghijklmnopqrstuvwxyz0123456789", 8))
}

// RandomStringFromCharset exports randomStringFromCharset for service root
func RandomStringFromCharset(charset string, length int) string {
	return randomStringFromCharset(charset, length)
}

func randomStringFromCharset(charset string, length int) string {
	if length <= 0 || charset == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(length)

	max := big.NewInt(int64(len(charset)))
	for index := 0; index < length; index++ {
		randomIndex, err := crand.Int(crand.Reader, max)
		if err != nil {
			fallbackIndex := int((time.Now().UnixNano() + int64(index)) % int64(len(charset)))
			if fallbackIndex < 0 {
				fallbackIndex = -fallbackIndex
			}
			builder.WriteByte(charset[fallbackIndex])
			continue
		}
		builder.WriteByte(charset[randomIndex.Int64()])
	}

	return builder.String()
}

// GenerateRandomStrongPassword 生成随机强密码（至少12位，含大小写字母、数字和符号）
func GenerateRandomStrongPassword() string {
	upper := "ABCDEFGHIJKLMNPQRSTUVWXYZ"
	lower := "abcdefghijkmnpqrstuvwxyz"
	digits := "23456789"
	symbols := "!@#$%^&*_+=?"

	charsets := []string{upper, lower, digits, symbols}
	var parts []string
	for _, cs := range charsets {
		parts = append(parts, randomStringFromCharset(cs, 3))
	}

	return strings.Join(parts, "")
}

// ---- 重装验证相关 ----

// ValidateOpenWrtStaticIP 校验 OpenWrt 静态 IP 地址（CIDR 格式）
func ValidateOpenWrtStaticIP(staticIP string) error {
	staticIP = strings.TrimSpace(staticIP)
	if staticIP == "" {
		return fmt.Errorf("OpenWrt 模板克隆需要指定静态 IP 地址")
	}
	// 支持 CIDR 格式（如 192.168.1.100/24）或纯 IP
	ip := staticIP
	if strings.Contains(staticIP, "/") {
		parts := strings.SplitN(staticIP, "/", 2)
		ip = parts[0]
		mask, err := strconv.Atoi(parts[1])
		if err != nil || mask < 1 || mask > 32 {
			return fmt.Errorf("子网掩码无效，应为 1-32 之间的数字")
		}
	}
	// 简单校验 IPv4 格式
	octets := strings.Split(ip, ".")
	if len(octets) != 4 {
		return fmt.Errorf("静态 IP 地址格式无效，应为 IPv4 CIDR 格式（如 192.168.1.100/24）")
	}
	for _, octet := range octets {
		val, err := strconv.Atoi(octet)
		if err != nil || val < 0 || val > 255 {
			return fmt.Errorf("静态 IP 地址格式无效，应为 IPv4 CIDR 格式（如 192.168.1.100/24）")
		}
	}
	return nil
}

type reinstallSystemDiskInfo struct {
	Path      string
	Device    string
	Bus       string
	SizeGB    int
	SizeBytes int64
}

// NormalizeReinstallDiskSizeGB 规范化重装系统盘大小
func NormalizeReinstallDiskSizeGB(requestedDiskSize, currentDiskSize, minDiskSize int) int {
	resolved := requestedDiskSize
	if resolved <= 0 {
		resolved = currentDiskSize
	}
	if resolved <= 0 {
		resolved = minDiskSize
	}
	if minDiskSize > 0 && resolved < minDiskSize {
		resolved = minDiskSize
	}
	return resolved
}

// ResolveReinstallDiskSizeGB 解析重装系统盘大小
func ResolveReinstallDiskSizeGB(vmName, templateName string, requestedDiskSize int) (int, error) {
	minDiskSize, err := D.GetTemplateMinDiskSizeGB(templateName)
	if err != nil {
		return 0, err
	}
	currentDiskSize, err := getVMSystemDiskSizeGB(vmName)
	if err != nil {
		return 0, err
	}
	resolved := NormalizeReinstallDiskSizeGB(requestedDiskSize, currentDiskSize, minDiskSize)
	if resolved <= 0 {
		return 0, fmt.Errorf("无法确定重装后的系统盘大小")
	}
	return resolved, nil
}

func getVMSystemDiskSizeGB(vmName string) (int, error) {
	info, err := inspectVMSystemDisk(vmName)
	if err != nil {
		return 0, err
	}
	return info.SizeGB, nil
}

func inspectVMSystemDisk(vmName string) (*reinstallSystemDiskInfo, error) {
	disks, err := disk.ListDisks(vmName)
	if err != nil {
		return nil, err
	}

	for _, d := range disks {
		if d.DeviceType == "cdrom" || strings.TrimSpace(d.Path) == "" {
			continue
		}
		info := &reinstallSystemDiskInfo{
			Path:   strings.TrimSpace(d.Path),
			Device: strings.TrimSpace(d.Device),
			Bus:    D.NormalizeVMDiskBus(d.Bus),
		}
		qemuInfo := utils.ExecCommand("qemu-img", "info", "--output=json", "-U", info.Path)
		if qemuInfo.Error == nil {
			info.SizeBytes = parseQemuInfoBytes(qemuInfo.Stdout, "virtual-size")
			info.SizeGB = bytesToCeilGB(info.SizeBytes)
		}
		if info.SizeGB <= 0 {
			info.SizeGB = parseCapacityGBString(d.CapacityGB)
		}
		if info.Bus == "" {
			info.Bus = "virtio"
		}
		if info.Path == "" {
			break
		}
		return info, nil
	}

	fallback := D.GetVMDiskInfo(vmName)
	if strings.TrimSpace(fallback.Path) == "" {
		return nil, fmt.Errorf("未找到虚拟机系统盘")
	}
	info := &reinstallSystemDiskInfo{
		Path:   strings.TrimSpace(fallback.Path),
		Device: strings.TrimSpace(fallback.Device),
		Bus:    "virtio",
		SizeGB: parseCapacityGBString(fallback.Size),
	}
	qemuInfo := utils.ExecCommand("qemu-img", "info", "--output=json", "-U", info.Path)
	if qemuInfo.Error == nil {
		info.SizeBytes = parseQemuInfoBytes(qemuInfo.Stdout, "virtual-size")
		info.SizeGB = bytesToCeilGB(info.SizeBytes)
	}
	return info, nil
}

func parseCapacityGBString(raw string) int {
	normalized := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "GB"))
	if normalized == "" {
		return 0
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(normalized), 64)
	if err != nil || value <= 0 {
		return 0
	}
	rounded := int(value)
	if float64(rounded) < value {
		rounded++
	}
	return rounded
}

func bytesToCeilGB(sizeBytes int64) int {
	if sizeBytes <= 0 {
		return 0
	}
	const gib = int64(1 << 30)
	sizeGB := sizeBytes / gib
	if sizeBytes%gib != 0 {
		sizeGB++
	}
	return int(sizeGB)
}

func normalizeBootFamily(bootType string) string {
	switch vm_xml.NormalizeVMBootType(bootType) {
	case vm_xml.VMBootTypeUEFI, vm_xml.VMBootTypeUEFISecure:
		return "uefi"
	case vm_xml.VMBootTypeBIOS:
		return "bios"
	default:
		return ""
	}
}

// IsReinstallBootFamilyCompatible 检查重装时启动族是否兼容
func IsReinstallBootFamilyCompatible(currentBootType, templateBootType string) bool {
	currentFamily := normalizeBootFamily(currentBootType)
	templateFamily := normalizeBootFamily(templateBootType)
	if currentFamily == "" || templateFamily == "" {
		return true
	}
	return currentFamily == templateFamily
}

func detectTemplateBootTypeForReinstall(templateName string, meta *TemplateMeta) (string, error) {
	if meta == nil {
		meta = &TemplateMeta{}
	}
	bootType := vm_xml.NormalizeVMBootType(D.NormalizeTemplateBootType(meta.BootType))
	if bootType != "" {
		return bootType, nil
	}
	templatePath, err := D.EnsureTemplatePath(templateName)
	if err != nil {
		return "", err
	}
	if D.DetectTemplateBootType(templatePath) == vm_xml.VMBootTypeUEFI {
		return vm_xml.VMBootTypeUEFI, nil
	}
	return vm_xml.VMBootTypeBIOS, nil
}

// parseQemuInfoBytes is a helper to parse qemu-img info JSON output
func parseQemuInfoBytes(stdout, key string) int64 {
	// Simple JSON parsing for "virtual-size" field
	re := regexp.MustCompile(`"` + key + `"\s*:\s*(\d+)`)
	matches := re.FindStringSubmatch(stdout)
	if len(matches) > 1 {
		val, _ := strconv.ParseInt(matches[1], 10, 64)
		return val
	}
	return 0
}
