package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"qvmhub/model"
	"qvmhub/service"
	"qvmhub/taskqueue"
)

type cloneLXCReq struct {
	Snap    string `json:"snap" binding:"required"`
	DstName string `json:"dst_name" binding:"required"`
	Remark  string `json:"remark"`
}

// CloneFromContainer 从源容器的指定快照克隆出新容器（异步任务）。
// 同步预校验（源存在/快照存在/名合法不重名/仅 zfs）后派发任务；大 rootfs 克隆走任务队列避免超时。
// POST /api/lxc/:name/clone
func CloneFromContainer(c *gin.Context) {
	var req cloneLXCReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: snap 与 dst_name 必填"})
		return
	}
	srcName := c.Param("name")
	if err := service.LXCValidateCloneFromSnapshot(srcName, req.Snap, req.DstName); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	username, _ := c.Get("username")
	role, _ := c.Get("role")
	// 非 admin 校验配额：克隆继承源规格，按源 CPU/Mem 计配额
	if role != "admin" {
		cpu, memMB, err := service.LXCContainerSpecForQuota(srcName)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "查询源容器规格失败: " + err.Error()})
			return
		}
		if err := service.LXCCheckQuota(username.(string), cpu, memMB); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
			return
		}
	}
	params := &service.LXCCloneParams{
		SrcName: srcName,
		Snap:    req.Snap,
		DstName: req.DstName,
		Remark:  req.Remark,
	}
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeLXCClone, params, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "提交克隆任务失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "克隆任务已提交", "data": gin.H{"task_id": task.ID}})
}
