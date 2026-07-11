package vm

import (
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"qvmhub/config"
	"qvmhub/model"
)

// VMCredentialInfo 详情页展示的虚拟机凭据
type VMCredentialInfo struct {
	Username    string  `json:"username"`
	Password    string  `json:"password"`
	Source      string  `json:"source"`
	Operator    string  `json:"operator"`
	UpdatedAt   string  `json:"updated_at"`
	LastResetAt *string `json:"last_reset_at,omitempty"`
}

// SaveVMCredential 保存或更新虚拟机凭据
func SaveVMCredential(vmName, username, password, source, operator string, markReset bool) error {
	vmName = strings.TrimSpace(vmName)
	username = strings.TrimSpace(username)
	if vmName == "" || username == "" || password == "" {
		return nil
	}

	encrypted, err := encryptVMSecret(password)
	if err != nil {
		return fmt.Errorf("加密虚拟机密码失败: %w", err)
	}

	now := time.Now()
	updates := map[string]interface{}{
		"username":     username,
		"password_enc": encrypted,
		"source":       strings.TrimSpace(source),
		"operator":     strings.TrimSpace(operator),
		"updated_at":   now,
	}
	if markReset {
		updates["last_reset_at"] = &now
	}

	var record model.VMCredential
	err = model.DB.Where("vm_name = ?", vmName).First(&record).Error
	if err != nil {
		newRecord := &model.VMCredential{
			VMName:      vmName,
			Username:    username,
			PasswordEnc: encrypted,
			Source:      strings.TrimSpace(source),
			Operator:    strings.TrimSpace(operator),
		}
		if markReset {
			newRecord.LastResetAt = &now
		}
		if createErr := model.DB.Create(newRecord).Error; createErr != nil {
			return fmt.Errorf("保存虚拟机凭据失败: %w", createErr)
		}
		return nil
	}

	if updateErr := model.DB.Model(&record).Updates(updates).Error; updateErr != nil {
		return fmt.Errorf("更新虚拟机凭据失败: %w", updateErr)
	}
	return nil
}

// GetVMCredential 获取虚拟机凭据
func GetVMCredential(vmName string) (*VMCredentialInfo, error) {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" {
		return nil, nil
	}

	var record model.VMCredential
	if err := model.DB.Where("vm_name = ?", vmName).First(&record).Error; err != nil {
		return nil, nil
	}

	password, err := decryptVMSecret(record.PasswordEnc)
	if err != nil {
		return nil, fmt.Errorf("解密虚拟机密码失败: %w", err)
	}

	info := &VMCredentialInfo{
		Username:  record.Username,
		Password:  password,
		Source:    record.Source,
		Operator:  record.Operator,
		UpdatedAt: record.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	if record.LastResetAt != nil {
		formatted := record.LastResetAt.Format("2006-01-02 15:04:05")
		info.LastResetAt = &formatted
	}

	return info, nil
}

// DeleteVMCredential 删除虚拟机凭据
func DeleteVMCredential(vmName string) error {
	vmName = strings.TrimSpace(vmName)
	if vmName == "" {
		return nil
	}
	return model.DB.Where("vm_name = ?", vmName).Delete(&model.VMCredential{}).Error
}

// encryptVMSecret 使用 AES-GCM 加密凭据
func encryptVMSecret(plainText string) (string, error) {
	block, err := aes.NewCipher(buildCredentialKey())
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

// decryptVMSecret 解密凭据
func decryptVMSecret(cipherText string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(buildCredentialKey())
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

// buildCredentialKey 生成固定长度加密密钥
func buildCredentialKey() []byte {
	sum := sha256.Sum256([]byte(config.GlobalConfig.VMCredentialSecret))
	return sum[:]
}
