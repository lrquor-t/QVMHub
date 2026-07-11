package middleware

import (
	"crypto/sha256"
	"encoding/base64"
	"net"
	"strings"

	"qvmhub/model"
)

// GenerateSessionFingerprint 根据客户端 IP 和 User-Agent 生成会话指纹。
// 取 IP 前缀（IPv4 前3段 / IPv6 前3段）与 User-Agent 拼接后做 SHA256，
// 截取前 12 字节并使用 RawURLEncoding base64 编码，返回 16 字符字符串。
func GenerateSessionFingerprint(ip, userAgent string) string {
	ipPrefix := extractIPPrefix(ip)
	material := ipPrefix + "|" + userAgent
	sum := sha256.Sum256([]byte(material))
	return base64.RawURLEncoding.EncodeToString(sum[:12])
}

// extractIPPrefix 提取 IP 地址前缀。
// IPv4（含 IPv4-mapped IPv6）按 "." 分割取前3段；IPv6 按 ":" 分割取前3段；
// 解析失败时返回原始字符串。
func extractIPPrefix(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip
	}
	// IPv4-mapped IPv6 地址（如 ::ffff:192.168.1.1）会被 To4() 转换
	if v4 := parsed.To4(); v4 != nil {
		parts := strings.Split(v4.String(), ".")
		if len(parts) >= 3 {
			return strings.Join(parts[:3], ".")
		}
		return v4.String()
	}
	// IPv6
	v6Str := parsed.String()
	parts := strings.Split(v6Str, ":")
	if len(parts) >= 3 {
		return strings.Join(parts[:3], ":")
	}
	return v6Str
}

// isSessionFingerprintEnabled 判断会话指纹绑定是否启用。
// 设置项 session_fingerprint_enabled 值为 "false" 时返回 false，
// 其他情况（包括不存在）默认返回 true（默认开启）。
func isSessionFingerprintEnabled() bool {
	val, ok := model.GetSetting("session_fingerprint_enabled")
	if !ok {
		return true
	}
	return val != "false"
}
