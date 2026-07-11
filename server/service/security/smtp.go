package security

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"qvmhub/config"
)

// SMTPConfigView 前端展示用的 SMTP 配置
type SMTPConfigView struct {
	Host               string `json:"smtp_host"`
	Port               int    `json:"smtp_port"`
	Username           string `json:"smtp_username"`
	FromName           string `json:"smtp_from_name"`
	FromAddress        string `json:"smtp_from_address"`
	Security           string `json:"smtp_security"`
	TimeoutSeconds     int    `json:"smtp_timeout_seconds"`
	PasswordConfigured bool   `json:"smtp_password_configured"`
	Configured         bool   `json:"smtp_configured"`
}

// GetSMTPConfigView 获取当前 SMTP 配置视图
func GetSMTPConfigView() SMTPConfigView {
	cfg := config.GlobalConfig
	return SMTPConfigView{
		Host:               cfg.SMTPHost,
		Port:               cfg.SMTPPort,
		Username:           cfg.SMTPUsername,
		FromName:           cfg.SMTPFromName,
		FromAddress:        cfg.SMTPFromAddress,
		Security:           normalizeSMTPSecurity(cfg.SMTPSecurity),
		TimeoutSeconds:     cfg.SMTPTimeoutSeconds,
		PasswordConfigured: strings.TrimSpace(cfg.SMTPPasswordEnc) != "",
		Configured:         IsSMTPConfigured(),
	}
}

// IsSMTPConfigured 判断 SMTP 是否已完成基础配置
func IsSMTPConfigured() bool {
	cfg := config.GlobalConfig
	return strings.TrimSpace(cfg.SMTPHost) != "" &&
		cfg.SMTPPort > 0 &&
		strings.TrimSpace(cfg.SMTPFromAddress) != ""
}

// SMTPTestConfig 测试邮件时使用的 SMTP 配置（无需持久化）
type SMTPTestConfig struct {
	Host           string `json:"smtp_host"`
	Port           int    `json:"smtp_port"`
	Username       string `json:"smtp_username"`
	Password       string `json:"smtp_password"`
	FromName       string `json:"smtp_from_name"`
	FromAddress    string `json:"smtp_from_address"`
	Security       string `json:"smtp_security"`
	TimeoutSeconds int    `json:"smtp_timeout_seconds"`
}

// Validate 校验测试配置必填字段
func (c *SMTPTestConfig) Validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("SMTP 主机不能为空")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("SMTP 端口无效")
	}
	if strings.TrimSpace(c.FromAddress) == "" {
		return fmt.Errorf("发件邮箱不能为空")
	}
	return nil
}

// SendEmailWithConfig 使用传入的 SMTP 配置发送邮件（不依赖全局配置）
func SendEmailWithConfig(cfg SMTPTestConfig, to, subject, body string) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	host := strings.TrimSpace(cfg.Host)
	addr := net.JoinHostPort(host, strconv.Itoa(cfg.Port))
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	fromAddress := strings.TrimSpace(cfg.FromAddress)
	headers := []string{
		fmt.Sprintf("From: %s <%s>", encodeMailHeader(cfg.FromName), fromAddress),
		fmt.Sprintf("To: %s", strings.TrimSpace(to)),
		fmt.Sprintf("Subject: %s", encodeMailHeader(subject)),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}
	message := []byte(strings.Join(headers, "\r\n"))

	var auth smtp.Auth
	if strings.TrimSpace(cfg.Username) != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, host)
	}

	securityMode := normalizeSMTPSecurity(cfg.Security)
	switch securityMode {
	case "ssl":
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", addr, &tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		})
		if err != nil {
			return fmt.Errorf("连接 SMTP SSL 失败: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return fmt.Errorf("创建 SMTP 客户端失败: %w", err)
		}
		defer client.Close()
		if err := smtpSend(client, auth, fromAddress, strings.TrimSpace(to), message); err != nil {
			return err
		}
	default:
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			return fmt.Errorf("连接 SMTP 失败: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return fmt.Errorf("创建 SMTP 客户端失败: %w", err)
		}
		defer client.Close()

		if securityMode == "starttls" {
			if ok, _ := client.Extension("STARTTLS"); ok {
				if err := client.StartTLS(&tls.Config{
					ServerName: host,
					MinVersion: tls.VersionTLS12,
				}); err != nil {
					return fmt.Errorf("启动 STARTTLS 失败: %w", err)
				}
			}
		}
		if err := smtpSend(client, auth, fromAddress, strings.TrimSpace(to), message); err != nil {
			return err
		}
	}

	return nil
}

// SetSMTPPassword 更新运行时 SMTP 密码
func SetSMTPPassword(plainPassword string) error {
	plainPassword = strings.TrimSpace(plainPassword)
	if plainPassword == "" {
		return nil
	}
	encrypted, err := EncryptSecurityText(plainPassword)
	if err != nil {
		return err
	}
	config.GlobalConfig.SMTPPasswordEnc = encrypted
	return nil
}

func getSMTPPassword() (string, error) {
	if strings.TrimSpace(config.GlobalConfig.SMTPPasswordEnc) == "" {
		return "", nil
	}
	plain, err := DecryptSecurityText(config.GlobalConfig.SMTPPasswordEnc)
	if err != nil {
		return "", fmt.Errorf("SMTP 密码解密失败，请前往「系统设置」页面重新输入 SMTP 密码以完成凭据更新（加密密钥可能已变更）")
	}
	return plain, nil
}

// SendEmail 发送纯文本邮件
func SendEmail(to, subject, body string) error {
	if !IsSMTPConfigured() {
		return fmt.Errorf("SMTP 尚未配置")
	}

	cfg := config.GlobalConfig
	password, err := getSMTPPassword()
	if err != nil {
		return fmt.Errorf("读取 SMTP 密码失败: %w", err)
	}

	fromAddress := strings.TrimSpace(cfg.SMTPFromAddress)
	host := strings.TrimSpace(cfg.SMTPHost)
	addr := net.JoinHostPort(host, strconv.Itoa(cfg.SMTPPort))
	timeout := time.Duration(cfg.SMTPTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	headers := []string{
		fmt.Sprintf("From: %s <%s>", encodeMailHeader(cfg.SMTPFromName), fromAddress),
		fmt.Sprintf("To: %s", strings.TrimSpace(to)),
		fmt.Sprintf("Subject: %s", encodeMailHeader(subject)),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}
	message := []byte(strings.Join(headers, "\r\n"))

	var auth smtp.Auth
	if strings.TrimSpace(cfg.SMTPUsername) != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUsername, password, host)
	}

	securityMode := normalizeSMTPSecurity(cfg.SMTPSecurity)
	switch securityMode {
	case "ssl":
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", addr, &tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		})
		if err != nil {
			return fmt.Errorf("连接 SMTP SSL 失败: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return fmt.Errorf("创建 SMTP 客户端失败: %w", err)
		}
		defer client.Close()
		if err := smtpSend(client, auth, fromAddress, strings.TrimSpace(to), message); err != nil {
			return err
		}
	default:
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			return fmt.Errorf("连接 SMTP 失败: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return fmt.Errorf("创建 SMTP 客户端失败: %w", err)
		}
		defer client.Close()

		if securityMode == "starttls" {
			if ok, _ := client.Extension("STARTTLS"); ok {
				if err := client.StartTLS(&tls.Config{
					ServerName: host,
					MinVersion: tls.VersionTLS12,
				}); err != nil {
					return fmt.Errorf("启动 STARTTLS 失败: %w", err)
				}
			}
		}
		if err := smtpSend(client, auth, fromAddress, strings.TrimSpace(to), message); err != nil {
			return err
		}
	}

	return nil
}

func smtpSend(client *smtp.Client, auth smtp.Auth, from, to string, message []byte) error {
	if auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("SMTP 认证失败: %w", err)
			}
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("设置收件人失败: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("写入邮件正文失败: %w", err)
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("发送邮件失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("发送邮件失败: %w", err)
	}
	return client.Quit()
}

func normalizeSMTPSecurity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ssl", "tls":
		return "ssl"
	case "none":
		return "none"
	default:
		return "starttls"
	}
}

func encodeMailHeader(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "=?UTF-8?B?5bCP5pyxRUE弹性云?="
	}
	return fmt.Sprintf("=?UTF-8?B?%s?=", encodeBase64(value))
}

func encodeBase64(value string) string {
	return base64.StdEncoding.EncodeToString([]byte(value))
}
