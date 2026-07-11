package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"qvmhub/service"
)

// GetVMSchedules 获取虚拟机定时任务列表。
func GetVMSchedules(c *gin.Context) {
	vmName := strings.TrimSpace(c.Param("name"))
	if vmName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "虚拟机名称不能为空",
		})
		return
	}

	list, err := service.ListVMSchedules(vmName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取定时任务列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "ok",
		"data":    list,
	})
}

// CreateVMSchedule 创建虚拟机定时任务。
func CreateVMSchedule(c *gin.Context) {
	vmName := strings.TrimSpace(c.Param("name"))
	if vmName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "虚拟机名称不能为空",
		})
		return
	}

	var req service.VMScheduleInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
		})
		return
	}

	if shouldVerifyDeleteSchedule(req.Action, req.Enabled, true) && !requireHighRiskVerification(c, "create_vm_schedule_delete") {
		return
	}

	username, _ := c.Get("username")
	item, err := service.CreateVMSchedule(vmName, username.(string), req)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{
			"code":    status,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "定时任务已创建",
		"data":    item,
	})
}

// UpdateVMSchedule 更新虚拟机定时任务。
func UpdateVMSchedule(c *gin.Context) {
	vmName := strings.TrimSpace(c.Param("name"))
	scheduleID, ok := parseScheduleID(c)
	if !ok {
		return
	}

	var req service.VMScheduleInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
		})
		return
	}

	if shouldVerifyDeleteSchedule(req.Action, req.Enabled, false) && !requireHighRiskVerification(c, "update_vm_schedule_delete") {
		return
	}

	item, err := service.UpdateVMSchedule(vmName, scheduleID, req)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{
			"code":    status,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "定时任务已更新",
		"data":    item,
	})
}

// DeleteVMSchedule 删除虚拟机定时任务。
func DeleteVMSchedule(c *gin.Context) {
	vmName := strings.TrimSpace(c.Param("name"))
	scheduleID, ok := parseScheduleID(c)
	if !ok {
		return
	}

	if err := service.DeleteVMSchedule(vmName, scheduleID); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{
			"code":    status,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "定时任务已删除",
	})
}

func parseScheduleID(c *gin.Context) (uint, bool) {
	vmName := strings.TrimSpace(c.Param("name"))
	if vmName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "虚拟机名称不能为空",
		})
		return 0, false
	}

	rawID := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "定时任务 ID 不合法",
		})
		return 0, false
	}
	return uint(id), true
}

func shouldVerifyDeleteSchedule(action string, enabled *bool, isCreate bool) bool {
	if strings.ToLower(strings.TrimSpace(action)) != service.VMScheduleActionDelete {
		return false
	}
	if isCreate {
		return enabled == nil || *enabled
	}
	if enabled == nil {
		return true
	}
	return *enabled
}
