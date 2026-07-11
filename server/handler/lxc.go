package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"qvmhub/model"
	"qvmhub/service"
	"qvmhub/taskqueue"
)

// ListLXCContainers 列出当前用户可见的 LXC 容器。
func ListLXCContainers(c *gin.Context) {
	username, _ := c.Get("username")
	role, _ := c.Get("role")
	rows, err := service.LXCListContainers(username.(string), role == "admin")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取容器列表失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": rows})
}

// GetLXCDetail 获取容器详情。
func GetLXCDetail(c *gin.Context) {
	name := c.Param("name")
	d, err := service.LXCGetContainerDetail(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取容器详情失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": d})
}

type updateLXCConfigReq struct {
	CPUShares int    `json:"cpu_shares"`
	MemoryMB  int    `json:"memory_mb"`
	Autostart *bool  `json:"autostart"`
	Remark    string `json:"remark"`
	GroupName string `json:"group_name"`
}

// UpdateLXCConfig 编辑容器配置（cgroup CPU/内存、autostart、备注、分组）。
func UpdateLXCConfig(c *gin.Context) {
	var req updateLXCConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if err := service.LXCUpdateContainerConfig(c.Param("name"), service.LXCContainerConfigUpdate{
		CPUShares: req.CPUShares, MemoryMB: req.MemoryMB, Autostart: req.Autostart,
		Remark: req.Remark, GroupName: req.GroupName,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
}

// GetLXCDiskLimit GET /api/lxc/:name/disk-limit
func GetLXCDiskLimit(c *gin.Context) {
	gb, err := service.LXCGetDiskLimit(c.Param("name"))
	if err != nil {
		// 非 zfs 容器也算业务错误，返 400 让前端静默/隐藏字段
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "非 zfs") {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"code": code, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": gin.H{"gb": gb}})
}

type setLXCDiskLimitReq struct {
	GB int `json:"gb"`
}

// SetLXCDiskLimit PUT /api/lxc/:name/disk-limit
func SetLXCDiskLimit(c *gin.Context) {
	var req setLXCDiskLimitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if err := service.LXCSetDiskLimit(c.Param("name"), req.GB); err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "非 zfs") {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"code": code, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "磁盘配额已更新"})
}

// ListLXCSnapshots 列出容器快照。
func ListLXCSnapshots(c *gin.Context) {
	snaps, err := service.LXCListSnapshots(c.Param("name"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": snaps})
}

type createLXCSnapReq struct {
	Comment string `json:"comment"`
}

// CreateLXCSnapshot 提交异步快照任务（大 rootfs 快照可能耗时）。
func CreateLXCSnapshot(c *gin.Context) {
	name := c.Param("name")
	var req createLXCSnapReq
	_ = c.ShouldBindJSON(&req) // body 可选：无备注时可空 body
	username, _ := c.Get("username")
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeLXCSnapshot, service.LXCSnapshotParams{Name: name, Comment: req.Comment}, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "提交快照任务失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "快照任务已提交", "data": gin.H{"task_id": task.ID}})
}

type restoreLXCSnapReq struct {
	Snap string `json:"snap" binding:"required"`
}

// RestoreLXCSnapshot 从指定快照恢复容器。
func RestoreLXCSnapshot(c *gin.Context) {
	var req restoreLXCSnapReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if err := service.LXCRestoreSnapshot(c.Param("name"), req.Snap); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
}

// DeleteLXCSnapshot 删除指定快照。
func DeleteLXCSnapshot(c *gin.Context) {
	if err := service.LXCDeleteSnapshot(c.Param("name"), c.Param("snap")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
}
