package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"qvmhub/service"
)

// AddShareRequest 添加共享目录请求
type AddShareRequest struct {
	HostPath      string `json:"host_path" binding:"required"`
	Tag           string `json:"tag" binding:"required"`
	SecurityModel string `json:"security_model"` // mapped/passthrough
	ReadOnly      bool   `json:"readonly"`
}

// GetShareList 获取共享目录列表
func GetShareList(c *gin.Context) {
	name := c.Param("name")

	shares, err := service.ListShares(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "ok",
		"data":    shares,
	})
}

// AddShare 添加共享目录
func AddShare(c *gin.Context) {
	name := c.Param("name")
	var req AddShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
		})
		return
	}

	if err := service.AddShare(name, req.HostPath, req.Tag, req.SecurityModel, req.ReadOnly); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "共享目录添加成功",
	})
}

// DeleteShare 移除共享目录
func DeleteShare(c *gin.Context) {
	name := c.Param("name")
	tag := c.Param("tag")

	if err := service.RemoveShare(name, tag); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "共享目录已移除",
	})
}
