package security

import (
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"qvmhub/config"
)

// EncryptSecurityText 使用账户安全密钥加密敏感文本
func EncryptSecurityText(plainText string) (string, error) {
	block, err := aes.NewCipher(buildSecurityKey())
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(crand.Reader, nonce); err != nil {
		return "", err
	}

	cipherText := gcm.Seal(nil, nonce, []byte(plainText), nil)
	payload := append(nonce, cipherText...)
	return base64.StdEncoding.EncodeToString(payload), nil
}

// DecryptSecurityText 解密敏感文本（兼容旧密钥过渡期）
func DecryptSecurityText(cipherText string) (string, error) {
	plain, _, err := DecryptSecurityTextAutoUpgrade(cipherText)
	return plain, err
}

// DecryptSecurityTextAutoUpgrade 解密并在旧密钥解密成功后自动用新密钥重加密
// 返回值：plainText, upgradedCipher（空字符串表示无需升级）, error
func DecryptSecurityTextAutoUpgrade(cipherText string) (string, string, error) {
	raw, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", "", err
	}

	// 优先使用新密钥解密
	plain, newKeyErr := decryptRawWithKey(raw, buildSecurityKey())
	if newKeyErr == nil {
		return plain, "", nil
	}

	// 回退到旧密钥（过渡期兼容）
	if config.GlobalConfig.LegacySecuritySecret != "" {
		if plain, legacyErr := decryptRawWithKey(raw, buildSecurityKeyWithSecret(config.GlobalConfig.LegacySecuritySecret)); legacyErr == nil {
			// 旧密钥解密成功，用新密钥重新加密
			if upgraded, encErr := EncryptSecurityText(plain); encErr == nil {
				return plain, upgraded, nil
			}
			// 重加密失败不影响解密结果
			return plain, "", nil
		}
	}

	// 两种密钥均失败，返回明确错误提示
	return "", "", fmt.Errorf("解密失败：当前密钥与旧密钥均无法解密该数据，可能是加密密钥已变更，请重新设置相关凭据")
}

// decryptRawWithKey 使用指定密钥解密原始字节
func decryptRawWithKey(raw []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", fmt.Errorf("密文格式无效")
	}

	nonce := raw[:nonceSize]
	payload := raw[nonceSize:]
	plain, err := gcm.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", err
	}

	return string(plain), nil
}

func buildSecurityKey() []byte {
	return buildSecurityKeyWithSecret(config.GlobalConfig.SecuritySecret)
}

func buildSecurityKeyWithSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}
