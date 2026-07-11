package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"qvmhub/service"
)

// GetLXCSchedules 获取容器定时任务列表。
func GetLXCSchedules(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "容器名称不能为空",
		})
		return
	}

	list, err := service.LXCListSchedules(name)
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

// CreateLXCSchedule 创建容器定时任务。
func CreateLXCSchedule(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "容器名称不能为空",
		})
		return
	}

	var req service.LXCScheduleInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
		})
		return
	}

	if shouldVerifyDeleteSchedule(req.Action, req.Enabled, true) && !requireHighRiskVerification(c, "create_lxc_schedule_delete") {
		return
	}

	username, _ := c.Get("username")
	item, err := service.LXCCreateLXCSchedule(name, username.(string), req)
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

// UpdateLXCSchedule 更新容器定时任务。
func UpdateLXCSchedule(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	scheduleID, ok := parseScheduleID(c)
	if !ok {
		return
	}

	var req service.LXCScheduleInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
		})
		return
	}

	if shouldVerifyDeleteSchedule(req.Action, req.Enabled, false) && !requireHighRiskVerification(c, "update_lxc_schedule_delete") {
		return
	}

	item, err := service.LXCUpdateLXCSchedule(name, scheduleID, req)
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

// DeleteLXCSchedule 删除容器定时任务。
func DeleteLXCSchedule(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	scheduleID, ok := parseScheduleID(c)
	if !ok {
		return
	}

	if err := service.LXCDeleteLXCSchedule(name, scheduleID); err != nil {
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
