package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"qvmhub/service"
)

// GetAPIKeyInfo 获取当前用户 API Key 元信息。
func GetAPIKeyInfo(c *gin.Context) {
	user := getCurrentUser(c)
	info, err := service.GetUserAPIKeyInfo(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "读取 API 凭证失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": info})
}

// RotateAPIKey 生成或重新生成当前用户 API Key。
func RotateAPIKey(c *gin.Context) {
	if !requireHighRiskVerification(c, "rotate_api_key") {
		return
	}
	user := getCurrentUser(c)
	key, err := service.RotateUserAPIKey(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "生成 API 凭证失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "API 凭证已生成，请立即复制保存 API Key", "data": key})
}

// RevokeAPIKey 撤销当前用户 API Key。
func RevokeAPIKey(c *gin.Context) {
	if !requireHighRiskVerification(c, "revoke_api_key") {
		return
	}
	user := getCurrentUser(c)
	if err := service.RevokeUserAPIKey(user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "撤销 API 凭证失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "API 凭证已撤销"})
}
