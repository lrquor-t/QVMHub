package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"qvmhub/config"
	"qvmhub/middleware"
	"qvmhub/model"
	"qvmhub/service"
)

func getCurrentUser(c *gin.Context) *model.User {
	user, _ := c.Get("current_user")
	currentUser, _ := user.(*model.User)
	return currentUser
}

func buildBaseURL(c *gin.Context) string {
	if configured := normalizeBaseURL(config.GlobalConfig.PublicBaseURL); configured != "" {
		return configured
	}

	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		if c.Request.TLS != nil || c.Request.URL.Scheme == "https" {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

func normalizeBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	return "http://" + trimmed
}

func requireHighRiskVerification(c *gin.Context, operation string) bool {
	return requireHighRiskVerificationWithOptions(c, operation, false)
}

func requireStrictHighRiskVerification(c *gin.Context, operation string) bool {
	return requireHighRiskVerificationWithOptions(c, operation, false)
}

func requireHighRiskVerificationWithOptions(c *gin.Context, operation string, allowAdminAPIKeyBypass bool) bool {
	if service.IsSecurityVerificationDisabled() {
		return true
	}
	user := getCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "用户未登录",
		})
		return false
	}
	if service.CanSkipHighRiskVerification(user) {
		return true
	}
	if allowAdminAPIKeyBypass {
		if authType, _ := c.Get("auth_type"); authType == "api_key" && user.Role == "admin" {
			return true
		}
	}

	// SMTP 未配置时，回退到 TOTP 验证（如果已启用）；TOTP 也未启用则跳过
	if !service.IsSMTPConfigured() {
		if user.TOTPEnabled {
			// TOTP 已启用，走 TOTP 验证流程（不在此处 return，让下面的 TOTP 逻辑处理）
		} else {
			// SMTP 和 TOTP 都不可用，无法进行二次验证
			return true
		}
	}

	if operation == "" {
		operation = "high_risk_operation"
	}

	if authType, _ := c.Get("auth_type"); authType == "api_key" {
		if token := strings.TrimSpace(c.GetHeader("X-High-Risk-Token")); token != "" {
			claims, err := middleware.ParseToken(token)
			if err == nil &&
				claims.TokenType == service.TokenTypeHighRisk &&
				claims.UserID == user.ID &&
				claims.Operation == operation {
				return true
			}
		}
	}

	if user.TOTPEnabled {
		if token := strings.TrimSpace(c.GetHeader("X-High-Risk-Token")); token != "" {
			claims, err := middleware.ParseToken(token)
			if err == nil &&
				claims.TokenType == service.TokenTypeHighRisk &&
				claims.UserID == user.ID &&
				claims.Operation == operation {
				return true
			}
		}
		respData := gin.H{
			"method":    service.ChallengeMethodTOTP,
			"operation": operation,
		}
		if service.HasRecoveryCodes(user) {
			respData["has_recovery"] = true
		}
		c.JSON(http.StatusPreconditionRequired, gin.H{
			"code":    http.StatusPreconditionRequired,
			"message": "当前操作需要 2FA 验证",
			"data":    respData,
		})
		return false
	}

	if user.EmailVerifiedAt == nil || strings.TrimSpace(user.Email) == "" {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "请先绑定并验证邮箱后再执行该操作",
		})
		return false
	}
	if !service.IsSMTPConfigured() {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "当前未配置 SMTP，暂时无法完成邮箱验证",
		})
		return false
	}
	challenge, err := service.IssueEmailChallenge(
		user,
		service.ChallengePurposeHighRiskEmail,
		user.Email,
		"高风险操作验证",
		fmt.Sprintf("您正在执行高风险操作（%s），请输入以下验证码继续。", operation),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "发送邮箱验证码失败: " + err.Error(),
		})
		return false
	}

	c.JSON(http.StatusPreconditionRequired, gin.H{
		"code":    http.StatusPreconditionRequired,
		"message": "当前操作需要邮箱验证",
		"data": gin.H{
			"method":       service.ChallengeMethodEmail,
			"operation":    operation,
			"challenge_id": challenge.ID,
			"masked_email": service.MaskEmail(user.Email),
			"expires_in":   int(service.EmailCodeTTL.Seconds()),
		},
	})
	return false
}

func requireMaintenanceModeDisabled(c *gin.Context, action string) bool {
	if err := service.EnsureMaintenanceModeDisabled(action); err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": err.Error(),
		})
		return false
	}
	return true
}
