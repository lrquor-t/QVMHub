package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"qvmhub/service"
)

// LockVM 锁定虚拟机
func LockVM(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "虚拟机名称不能为空",
		})
		return
	}

	username, _ := c.Get("username")
	usernameStr, _ := username.(string)

	if err := service.LockVM(name, usernameStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "锁定虚拟机失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "虚拟机已锁定",
	})
}

// UnlockVM 解锁虚拟机（需要二次验证）
func UnlockVM(c *gin.Context) {
	if !requireHighRiskVerification(c, "unlock_vm") {
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "虚拟机名称不能为空",
		})
		return
	}

	username, _ := c.Get("username")
	usernameStr, _ := username.(string)

	if err := service.UnlockVM(name, usernameStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "解锁虚拟机失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "虚拟机已解锁",
	})
}

// GetVMLockStatus 获取虚拟机锁定状态
func GetVMLockStatus(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "虚拟机名称不能为空",
		})
		return
	}

	lockInfo := service.GetVMLockInfo(name)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "ok",
		"data":    lockInfo,
	})
}
