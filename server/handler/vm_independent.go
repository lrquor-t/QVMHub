package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"qvmhub/model"
	"qvmhub/service"
	"qvmhub/taskqueue"
)

// MakeVMIndependent 将链式克隆虚拟机转为独立虚拟机（管理员接口，异步任务）
func MakeVMIndependent(c *gin.Context) {
	if !requireMaintenanceModeDisabled(c, "转为独立虚拟机") {
		return
	}

	vmName := c.Param("name")
	if vmName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "虚拟机名称不能为空",
		})
		return
	}

	// 二次验证（敏感操作）
	if !requireHighRiskVerification(c, "make_vm_independent") {
		return
	}

	params := &service.MakeVMIndependentParams{
		VMName: vmName,
	}

	username, _ := c.Get("username")
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeMakeVMIndependent, params, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交转为独立虚拟机任务失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "转为独立虚拟机任务已提交",
		"data": gin.H{
			"task_id": task.ID,
		},
	})
}
