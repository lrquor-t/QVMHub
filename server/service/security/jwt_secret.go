package security

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/utils"
)

// 用于存储上一次轮换时写入 .env 的路径
var envFilePath string

func init() {
	envFilePath = config.EnvFilePath()
}

// GenerateRandomSecret 生成一个 48 字节的 base64 随机密钥
func GenerateRandomSecret() (string, error) {
	raw := make([]byte, 36)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("生成随机密钥失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// RotateJWTSecret 轮换 JWT 密钥并持久化到 .env 文件
// 轮换后所有旧 Token 立即失效
func RotateJWTSecret() (string, error) {
	newSecret, err := GenerateRandomSecret()
	if err != nil {
		return "", err
	}

	if err := writeJWTSecretToEnv(newSecret); err != nil {
		return "", fmt.Errorf("写入 .env 文件失败: %w", err)
	}

	// 运行时生效
	config.GlobalConfig.JWTSecret = newSecret

	// 持久化到数据库设置表
	if err := model.SetSetting("jwt_secret_last_rotated", time.Now().UTC().Format(time.RFC3339)); err != nil {
		// 非致命：数据库写入失败不影响运行时密钥
		logger.App.Warn("记录轮换时间到数据库失败", "component", "JWT", "error", err)
	}

	logger.App.Info("密钥轮换完成，所有旧 Token 已失效", "component", "JWT")
	return newSecret, nil
}

// writeJWTSecretToEnv 将 JWT 密钥写入 .env 文件
func writeJWTSecretToEnv(secret string) error {
	if envFilePath == "" {
		return fmt.Errorf(".env 文件路径未配置")
	}

	content, err := os.ReadFile(envFilePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "KVM_JWT_SECRET=") {
			lines[i] = "KVM_JWT_SECRET=" + secret
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, "KVM_JWT_SECRET="+secret)
	}

	// 过滤空行
	var cleanLines []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			cleanLines = append(cleanLines, l)
		}
	}

	data := strings.Join(cleanLines, "\n") + "\n"
	return os.WriteFile(envFilePath, []byte(data), 0600)
}

// StartJWTSecretRotator 启动 JWT 密钥定时轮换协程
// 仅在非开发模式下运行，且 rotateHours > 0 时生效
func StartJWTSecretRotator() {
	rotateHours := config.GlobalConfig.JWTSecretRotateHours
	if rotateHours <= 0 {
		logger.App.Info("密钥自动轮换已禁用", "component", "JWT", "rotate_hours", rotateHours)
		return
	}
	if config.GlobalConfig.DevelopmentMode {
		logger.App.Info("开发模式下跳过密钥自动轮换", "component", "JWT", "interval_hours", rotateHours)
		return
	}
	// 默认密钥不允许自动轮换（防止用户未配置时意外轮换为随机值导致 Token 全部失效）
	if config.GlobalConfig.JWTSecret == "kvm-console-secret-key-change-me" {
		logger.App.Info("检测到默认 JWT 密钥，跳过自动轮换", "component", "JWT")
		return
	}

	interval := time.Duration(rotateHours) * time.Hour
	logger.App.Info("密钥自动轮换已启动", "component", "JWT", "interval_hours", rotateHours)

	go func() {
		defer utils.RecoverAndLog("jwt-secret-rotator")
		for {
			time.Sleep(interval)
			cfg := config.GlobalConfig
			if cfg == nil {
				continue
			}
			// 检查是否被禁用
			if cfg.JWTSecretRotateHours <= 0 {
				continue
			}
			if _, err := RotateJWTSecret(); err != nil {
				logger.App.Warn("密钥自动轮换失败", "component", "JWT", "error", err)
			}
		}
	}()
}
