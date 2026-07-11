package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TOTPSetupInfo 2FA 绑定信息
type TOTPSetupInfo struct {
	Secret     string `json:"secret"`
	OtpauthURL string `json:"otpauth_url"`
}

// GenerateTOTPSetup 生成 2FA 配置
func GenerateTOTPSetup(username string) (*TOTPSetupInfo, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "QVMConsole",
		AccountName: strings.TrimSpace(username),
		Algorithm:   otp.AlgorithmSHA1,
		Digits:      otp.DigitsSix,
		Period:      30,
		SecretSize:  20,
	})
	if err != nil {
		return nil, err
	}
	return &TOTPSetupInfo{
		Secret:     key.Secret(),
		OtpauthURL: key.URL(),
	}, nil
}

// ValidateTOTPCode 校验 TOTP 验证码
func ValidateTOTPCode(secret, code string) error {
	valid := totp.Validate(strings.TrimSpace(code), strings.TrimSpace(secret))
	if !valid {
		return fmt.Errorf("2FA 验证码错误")
	}
	return nil
}

// RecoveryCodesConfig 恢复码配置
const (
	RecoveryCodeCount = 10                                 // 生成恢复码数量
	RecoveryCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // 避免易混淆字符 0/O/1/I
	RecoveryCodeLen   = 16                                 // 每组恢复码长度
)

// TOTPRecoverySetup 恢复码结果（仅绑定/重新生成时返回一次）
type TOTPRecoverySetup struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

// generateRecoveryCode 生成单个恢复码
func generateRecoveryCode() (string, error) {
	code := make([]byte, RecoveryCodeLen)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(RecoveryCodeChars))))
		if err != nil {
			return "", err
		}
		code[i] = RecoveryCodeChars[n.Int64()]
	}
	return string(code), nil
}

// hashRecoveryCode 对恢复码做 SHA-256 哈希
func hashRecoveryCode(code string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(code)))
	return hex.EncodeToString(h[:])
}

// GenerateRecoveryCodes 生成一组恢复码，返回明文和哈希列表
func GenerateRecoveryCodes() ([]string, []string, error) {
	plain := make([]string, RecoveryCodeCount)
	hashed := make([]string, RecoveryCodeCount)
	for i := 0; i < RecoveryCodeCount; i++ {
		code, err := generateRecoveryCode()
		if err != nil {
			return nil, nil, fmt.Errorf("生成恢复码失败: %w", err)
		}
		plain[i] = code
		hashed[i] = hashRecoveryCode(code)
	}
	return plain, hashed, nil
}

// ValidateAndConsumeRecoveryCode 验证并消耗一个恢复码
// 传入 user，从加密字段解析哈希列表，匹配并移除
func ValidateAndConsumeRecoveryCode(encryptedEnc, inputCode string) (bool, string, error) {
	if strings.TrimSpace(encryptedEnc) == "" {
		return false, encryptedEnc, fmt.Errorf("没有可用的恢复码")
	}

	// 解密
	plainJSON, err := DecryptSecurityText(encryptedEnc)
	if err != nil {
		return false, encryptedEnc, fmt.Errorf("恢复码解密失败，加密密钥可能已变更，请重新生成恢复码")
	}

	var hashedCodes []string
	if err := json.Unmarshal([]byte(plainJSON), &hashedCodes); err != nil {
		return false, encryptedEnc, fmt.Errorf("解析恢复码失败")
	}

	inputHash := hashRecoveryCode(strings.TrimSpace(inputCode))

	// 常量时间比对，避免时序攻击
	found := false
	idx := -1
	for i, h := range hashedCodes {
		if subtle.ConstantTimeCompare([]byte(h), []byte(inputHash)) == 1 {
			found = true
			idx = i
			break
		}
	}

	if !found {
		return false, encryptedEnc, fmt.Errorf("恢复码无效或已使用")
	}

	// 移除已使用的恢复码
	newHashed := append(hashedCodes[:idx], hashedCodes[idx+1:]...)
	newJSON, err := json.Marshal(newHashed)
	if err != nil {
		return false, encryptedEnc, fmt.Errorf("更新恢复码失败")
	}

	// 重新加密
	newEnc, err := EncryptSecurityText(string(newJSON))
	if err != nil {
		return false, encryptedEnc, fmt.Errorf("加密恢复码失败")
	}

	return true, newEnc, nil
}

// GetRecoveryCodesRemaining 获取剩余恢复码数量
func GetRecoveryCodesRemaining(encryptedEnc string) int {
	if strings.TrimSpace(encryptedEnc) == "" {
		return 0
	}
	plainJSON, err := DecryptSecurityText(encryptedEnc)
	if err != nil {
		return 0
	}
	var hashedCodes []string
	if err := json.Unmarshal([]byte(plainJSON), &hashedCodes); err != nil {
		return 0
	}
	return len(hashedCodes)
}
