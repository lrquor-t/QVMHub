package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"qvmhub/service"
)

// VMMonitorCommandRequest QEMU Monitor 命令请求
type VMMonitorCommandRequest struct {
	Command string `json:"command" binding:"required"`
}

// GetVMMonitorStatus 获取虚拟机 QEMU Monitor 状态
func GetVMMonitorStatus(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "虚拟机名称不能为空",
		})
		return
	}

	status, err := service.GetVMMonitorStatus(name)
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
		"data":    status,
	})
}

// ExecuteVMMonitorCommand 执行虚拟机 QEMU Monitor 命令
func ExecuteVMMonitorCommand(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "虚拟机名称不能为空",
		})
		return
	}

	var req VMMonitorCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请提供要执行的监视器命令",
		})
		return
	}

	result, err := service.ExecuteVMMonitorCommand(name, req.Command)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    http.StatusBadRequest,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "监视器命令执行成功",
		"data":    result,
	})
}
