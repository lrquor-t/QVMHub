package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
	"time"

	"qvmhub/model"
)

// IssueEmailChallenge 生成并发送邮箱验证码
func IssueEmailChallenge(user *model.User, purpose, targetEmail, subject, contentPrefix string) (*model.SecurityChallenge, error) {
	if user == nil {
		return nil, fmt.Errorf("用户不存在")
	}
	targetEmail = strings.TrimSpace(targetEmail)
	if targetEmail == "" {
		return nil, fmt.Errorf("邮箱不能为空")
	}

	code, err := generateNumericCode(6)
	if err != nil {
		return nil, err
	}
	if err := invalidateChallenges(user.ID, purpose); err != nil {
		return nil, err
	}

	challenge := &model.SecurityChallenge{
		UserID:    user.ID,
		Purpose:   purpose,
		Method:    ChallengeMethodEmail,
		Target:    targetEmail,
		CodeHash:  hashCode(code),
		ExpiresAt: time.Now().Add(EmailCodeTTL),
	}
	if err := model.DB.Create(challenge).Error; err != nil {
		return nil, err
	}

	body := fmt.Sprintf("%s\n\n验证码：%s\n有效期：10 分钟\n如果不是您本人操作，请忽略此邮件。", contentPrefix, code)
	if err := SendEmail(targetEmail, subject, body); err != nil {
		_ = model.DB.Delete(challenge).Error
		return nil, err
	}
	return challenge, nil
}

// IssuePublicEmailChallenge 生成无需登录态的邮箱验证码。
func IssuePublicEmailChallenge(purpose, targetEmail, subject, contentPrefix string) (*model.SecurityChallenge, error) {
	targetEmail = strings.TrimSpace(strings.ToLower(targetEmail))
	if targetEmail == "" {
		return nil, fmt.Errorf("邮箱不能为空")
	}

	code, err := generateNumericCode(6)
	if err != nil {
		return nil, err
	}
	if err := invalidatePublicChallenges(targetEmail, purpose); err != nil {
		return nil, err
	}

	challenge := &model.SecurityChallenge{
		UserID:    0,
		Purpose:   purpose,
		Method:    ChallengeMethodEmail,
		Target:    targetEmail,
		CodeHash:  hashCode(code),
		ExpiresAt: time.Now().Add(EmailCodeTTL),
	}
	if err := model.DB.Create(challenge).Error; err != nil {
		return nil, err
	}

	body := fmt.Sprintf("%s\n\n验证码：%s\n有效期：10 分钟\n如果不是您本人操作，请忽略此邮件。", contentPrefix, code)
	if err := SendEmail(targetEmail, subject, body); err != nil {
		_ = model.DB.Delete(challenge).Error
		return nil, err
	}
	return challenge, nil
}

// VerifyEmailChallenge 校验邮箱验证码
func VerifyEmailChallenge(userID uint, challengeID uint, purpose, code string) (*model.SecurityChallenge, error) {
	challenge, err := findValidEmailChallenge(
		map[string]interface{}{
			"id":      challengeID,
			"user_id": userID,
			"purpose": purpose,
			"method":  ChallengeMethodEmail,
		},
		code,
	)
	if err != nil {
		return nil, err
	}
	return consumeEmailChallenge(challenge)
}

// VerifyPublicEmailChallenge 校验公共邮箱验证码。
func VerifyPublicEmailChallenge(challengeID uint, purpose, targetEmail, code string) (*model.SecurityChallenge, error) {
	targetEmail = strings.TrimSpace(strings.ToLower(targetEmail))
	challenge, err := findValidEmailChallenge(
		map[string]interface{}{
			"id":      challengeID,
			"user_id": 0,
			"purpose": purpose,
			"method":  ChallengeMethodEmail,
			"target":  targetEmail,
		},
		code,
	)
	if err != nil {
		return nil, err
	}
	return consumeEmailChallenge(challenge)
}

func invalidateChallenges(userID uint, purpose string) error {
	now := time.Now()
	return model.DB.Model(&model.SecurityChallenge{}).
		Where("user_id = ? AND purpose = ? AND consumed_at IS NULL", userID, purpose).
		Updates(map[string]interface{}{"consumed_at": &now}).Error
}

func invalidatePublicChallenges(targetEmail, purpose string) error {
	now := time.Now()
	return model.DB.Model(&model.SecurityChallenge{}).
		Where("user_id = ? AND target = ? AND purpose = ? AND consumed_at IS NULL", 0, targetEmail, purpose).
		Updates(map[string]interface{}{"consumed_at": &now}).Error
}

func findValidEmailChallenge(filters map[string]interface{}, code string) (*model.SecurityChallenge, error) {
	var challenge model.SecurityChallenge
	if err := model.DB.Where(filters).First(&challenge).Error; err != nil {
		return nil, fmt.Errorf("验证码不存在或已失效")
	}
	if challenge.ConsumedAt != nil {
		return nil, fmt.Errorf("验证码已失效")
	}
	if time.Now().After(challenge.ExpiresAt) {
		return nil, fmt.Errorf("验证码已过期")
	}
	if challenge.CodeHash != hashCode(code) {
		return nil, fmt.Errorf("验证码错误")
	}
	return &challenge, nil
}

func consumeEmailChallenge(challenge *model.SecurityChallenge) (*model.SecurityChallenge, error) {
	if challenge == nil {
		return nil, fmt.Errorf("验证码不存在或已失效")
	}
	now := time.Now()
	if err := model.DB.Model(challenge).Update("consumed_at", &now).Error; err != nil {
		return nil, err
	}
	challenge.ConsumedAt = &now
	return challenge, nil
}

func generateNumericCode(length int) (string, error) {
	var builder strings.Builder
	for range make([]struct{}, length) {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		builder.WriteByte(byte('0' + n.Int64()))
	}
	return builder.String(), nil
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(code)))
	return base64.StdEncoding.EncodeToString(sum[:])
}
