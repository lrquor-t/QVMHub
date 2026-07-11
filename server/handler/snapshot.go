package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"qvmhub/model"
	"qvmhub/service"
	"qvmhub/service/snapshot"
	"qvmhub/taskqueue"
)

// CreateSnapshotRequest 创建快照请求
type CreateSnapshotRequest struct {
	Name                   string `json:"name"`
	Description            string `json:"description"`
	IncludeMemory          bool   `json:"include_memory"`
	AutoFixNVRAM           bool   `json:"auto_fix_nvram"`
	PauseForMemorySnapshot *bool  `json:"pause_for_memory_snapshot"`
}

// SnapshotTaskParams 快照任务参数（用于异步任务队列）
type SnapshotTaskParams struct {
	VmName                 string `json:"vm_name"`
	SnapName               string `json:"snap_name"`
	Description            string `json:"description"`
	IncludeMemory          bool   `json:"include_memory"`
	AutoFixNVRAM           bool   `json:"auto_fix_nvram"`
	PauseForMemorySnapshot *bool  `json:"pause_for_memory_snapshot"`
	Action                 string `json:"action"` // create / revert / delete
}

// GetSnapshots 获取快照列表（保持同步，无需异步）
func GetSnapshots(c *gin.Context) {
	name := c.Param("name")
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	role, _ := c.Get("role")
	roleStr, _ := role.(string)

	snapshots, err := snapshot.ListSnapshots(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取快照列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "ok",
		"data":    snapshots,
		"quota":   snapshot.BuildVMSnapshotQuotaInfo(usernameStr, roleStr, name, len(snapshots)),
	})
}

// CreateSnapshot 创建快照（异步任务）
func CreateSnapshot(c *gin.Context) {
	vmName := c.Param("name")
	if err := service.EnsureVMNotMigrating(vmName, "创建快照"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}
	var req CreateSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数格式错误",
		})
		return
	}
	snapName, err := snapshot.NormalizeSnapshotName(req.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	params := &SnapshotTaskParams{
		VmName:                 vmName,
		SnapName:               snapName,
		Description:            req.Description,
		IncludeMemory:          req.IncludeMemory,
		AutoFixNVRAM:           req.AutoFixNVRAM,
		PauseForMemorySnapshot: req.PauseForMemorySnapshot,
		Action:                 "create",
	}

	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	role, _ := c.Get("role")
	roleStr, _ := role.(string)

	if err := snapshot.CheckVMSnapshotQuota(usernameStr, roleStr, vmName, 1); err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": err.Error(),
		})
		return
	}

	if req.IncludeMemory {
		unsupported, message, err := snapshot.CheckInternalSnapshotVirtFSUnsupported(vmName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "检查 VirtFS 共享目录状态失败: " + err.Error(),
			})
			return
		}
		if unsupported {
			c.JSON(http.StatusConflict, gin.H{
				"code":    409,
				"message": message,
			})
			return
		}
	}

	if req.IncludeMemory && !req.AutoFixNVRAM {
		required, message, err := snapshot.CheckInternalSnapshotNVRAMRepairRequired(vmName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "检查 UEFI NVRAM 快照兼容性失败: " + err.Error(),
			})
			return
		}
		if required {
			c.JSON(http.StatusConflict, gin.H{
				"code":    409,
				"message": message,
				"data": gin.H{
					"require_nvram_fix": true,
				},
			})
			return
		}
	}

	task, err := taskqueue.SubmitWithStruct(model.TaskTypeSnapshot, params, usernameStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交快照创建任务失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "快照创建任务已提交",
		"data": gin.H{
			"task_id": task.ID,
		},
	})
}

// RevertSnapshot 恢复快照（异步任务）
func RevertSnapshot(c *gin.Context) {
	vmName := c.Param("name")
	snapName := c.Param("snap")
	if err := service.EnsureVMNotMigrating(vmName, "恢复快照"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}

	params := &SnapshotTaskParams{
		VmName:   vmName,
		SnapName: snapName,
		Action:   "revert",
	}

	username, _ := c.Get("username")
	usernameStr, _ := username.(string)

	task, err := taskqueue.SubmitWithStruct(model.TaskTypeSnapshot, params, usernameStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交快照恢复任务失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "快照恢复任务已提交",
		"data": gin.H{
			"task_id": task.ID,
		},
	})
}

// DeleteSnapshot 删除快照（异步任务）
func DeleteSnapshot(c *gin.Context) {
	if !requireHighRiskVerification(c, "delete_snapshot") {
		return
	}
	vmName := c.Param("name")
	snapName := c.Param("snap")
	if err := service.EnsureVMNotMigrating(vmName, "删除快照"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}

	params := &SnapshotTaskParams{
		VmName:   vmName,
		SnapName: snapName,
		Action:   "delete",
	}

	username, _ := c.Get("username")
	usernameStr, _ := username.(string)

	task, err := taskqueue.SubmitWithStruct(model.TaskTypeSnapshot, params, usernameStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交快照删除任务失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "快照删除任务已提交",
		"data": gin.H{
			"task_id": task.ID,
		},
	})
}

// DeleteAllSnapshots 删除全部快照（异步任务）
func DeleteAllSnapshots(c *gin.Context) {
	if !requireHighRiskVerification(c, "delete_snapshot") {
		return
	}
	vmName := c.Param("name")
	if err := service.EnsureVMNotMigrating(vmName, "删除全部快照"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}

	params := &SnapshotTaskParams{
		VmName: vmName,
		Action: "delete_all",
	}

	username, _ := c.Get("username")
	usernameStr, _ := username.(string)

	task, err := taskqueue.SubmitWithStruct(model.TaskTypeSnapshot, params, usernameStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交全部快照删除任务失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "全部快照删除任务已提交",
		"data": gin.H{
			"task_id": task.ID,
		},
	})
}
