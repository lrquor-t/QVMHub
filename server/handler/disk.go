package handler

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"qvmhub/model"
	"qvmhub/service"
	"qvmhub/service/vm/vmimport"
	"qvmhub/taskqueue"
)

// AddDiskRequest 添加磁盘请求
type AddDiskRequest struct {
	SizeGB int    `json:"size_gb" binding:"required"`
	Format string `json:"format"` // qcow2/raw
	Bus    string `json:"bus"`    // 磁盘总线: virtio/scsi/sata/ide
}

// ResizeDiskRequest 扩容请求
type ResizeDiskRequest struct {
	SizeGB int `json:"size_gb" binding:"required"`
}

// DeleteDiskRequest 删除磁盘请求
type DeleteDiskRequest struct {
	DeleteFile bool `json:"delete_file"` // 是否删除文件
	Transfer   bool `json:"transfer"`    // 是否转移到用户存储
}

// MigrateDiskRequest 迁移硬盘请求
type MigrateDiskRequest struct {
	TargetStoragePoolID string `json:"target_storage_pool_id" binding:"required"`
}

// DeleteDisk 删除磁盘
func DeleteDisk(c *gin.Context) {
	name := c.Param("name")
	dev := c.Param("dev")
	if err := service.EnsureVMNotMigrating(name, "删除磁盘"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}

	// VM 存在性检查
	if !service.DomainExists(name) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "虚拟机不存在"})
		return
	}

	var req DeleteDiskRequest
	c.ShouldBindJSON(&req)
	if req.DeleteFile {
		if !requireHighRiskVerification(c, "delete_disk_file") {
			return
		}
	}

	// 转移模式：卸载后将磁盘文件异步转移到用户存储
	if req.Transfer {
		username, _ := c.Get("username")
		usernameStr := username.(string)

		// 获取磁盘文件路径（卸载前获取）
		diskPath := service.GetDiskFilePath(name, dev)
		if diskPath == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "无法获取磁盘文件路径",
			})
			return
		}

		// 检查用户存储池是否已初始化
		if !service.IsStorageInitialized(usernameStr) {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "您尚未开通「我的存储」，无法转移磁盘。请先前往「我的存储」页面初始化存储池。",
			})
			return
		}

		// 检查存储配额
		_, err := service.CheckDiskTransferQuota(usernameStr, []string{diskPath})
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": err.Error(),
			})
			return
		}

		// 先同步卸载磁盘（不删除文件）
		if err := service.RemoveDisk(name, dev, false); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": err.Error(),
			})
			return
		}

		// 提交异步任务转移文件
		params := map[string]string{
			"disk_path": diskPath,
			"username":  usernameStr,
			"device":    dev,
		}
		task, err := taskqueue.SubmitWithStruct(model.TaskTypeDiskTransfer, params, usernameStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "磁盘已卸载，但提交转移任务失败: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "磁盘已卸载，转移任务已提交",
			"data": gin.H{
				"task_id": task.ID,
			},
		})
		return
	}

	// 常规模式：删除或仅卸载
	if err := service.RemoveDisk(name, dev, req.DeleteFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "磁盘已删除",
	})
}

// GetDiskList 获取磁盘列表
func GetDiskList(c *gin.Context) {
	name := c.Param("name")

	disks, err := service.ListDisks(name)
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
		"data":    disks,
	})
}

// GetDiskMigrationOptions 获取本机硬盘迁移表单选项
func GetDiskMigrationOptions(c *gin.Context) {
	name := c.Param("name")
	options, err := service.GetVMDiskMigrationOptions(name)
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
		"data":    options,
	})
}

// MigrateDisk 提交本机硬盘迁移任务
func MigrateDisk(c *gin.Context) {
	name := c.Param("name")
	dev := c.Param("dev")
	if !requireHighRiskVerification(c, "migrate_vm_disk") {
		return
	}
	if err := service.EnsureVMNotMigrating(name, "迁移硬盘"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}
	var req MigrateDiskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请选择目标存储",
		})
		return
	}
	username, _ := c.Get("username")
	params := service.VMDiskMigrationTaskParams{
		VMName:              name,
		Device:              dev,
		TargetStoragePoolID: req.TargetStoragePoolID,
	}
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeVMDiskMigrate, params, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交硬盘迁移任务失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "硬盘迁移任务已提交",
		"data":    task,
	})
}

// AddDisk 添加磁盘
func AddDisk(c *gin.Context) {
	name := c.Param("name")
	if err := service.EnsureVMNotMigrating(name, "添加磁盘"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}
	var req AddDiskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请指定磁盘大小（GB）",
		})
		return
	}

	// 非管理员用户检查磁盘配额，且只能使用 qcow2 格式
	role, _ := c.Get("role")
	if role != "admin" {
		username, _ := c.Get("username")
		usernameStr, _ := username.(string)
		if err := service.CheckQuotaForEdit(usernameStr, 0, 0, req.SizeGB); err != nil {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": err.Error(),
			})
			return
		}
		// 普通用户只能使用 qcow2 格式
		req.Format = "qcow2"
	}

	bus := req.Bus
	if bus == "" {
		bus = "virtio"
	}
	dev, err := service.AddDiskWithBus(name, req.SizeGB, req.Format, bus)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "磁盘添加成功",
		"data": gin.H{
			"device": dev,
		},
	})
}

// AttachDiskRequest 挂载已有磁盘请求
type AttachDiskRequest struct {
	Path string `json:"path" binding:"required"` // 磁盘文件路径
	Bus  string `json:"bus"`                     // 总线类型: virtio/scsi/sata/ide
}

// AttachDisk 挂载已有磁盘文件
func AttachDisk(c *gin.Context) {
	name := c.Param("name")
	if err := service.EnsureVMNotMigrating(name, "挂载磁盘"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}
	var req AttachDiskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请指定磁盘文件路径",
		})
		return
	}

	dev, err := service.AttachExistingDisk(name, req.Path, req.Bus)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "磁盘挂载成功",
		"data": gin.H{
			"device": dev,
		},
	})
}

// ResizeDisk 磁盘扩容
func ResizeDisk(c *gin.Context) {
	name := c.Param("name")
	dev := c.Param("dev")
	if err := service.EnsureVMNotMigrating(name, "扩容磁盘"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}

	var req ResizeDiskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请指定新大小（GB）",
		})
		return
	}

	// 非管理员用户检查磁盘配额（扩容差值 = 目标大小 - 当前总磁盘）
	role, _ := c.Get("role")
	if role != "admin" {
		username, _ := c.Get("username")
		usernameStr, _ := username.(string)
		// 扩容是设置目标大小，计算与当前使用量的差值
		// 这里直接用请求的 SizeGB 作为增量（因为 ResizeDisk 接收的是目标大小，但配额检查只需要知道增量）
		// 我们通过 GetUserQuotaUsage 已经统计了当前所有磁盘大小，所以只需要检查增量
		currentDiskGB := service.GetVMDiskDevCapacityGB(name, dev)
		deltaDisk := req.SizeGB - currentDiskGB
		if deltaDisk > 0 {
			if err := service.CheckQuotaForEdit(usernameStr, 0, 0, deltaDisk); err != nil {
				c.JSON(http.StatusForbidden, gin.H{
					"code":    403,
					"message": err.Error(),
				})
				return
			}
		}
	}

	if err := service.ResizeDisk(name, dev, req.SizeGB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "磁盘扩容成功",
	})
}

// ChangeDiskBusRequest 修改磁盘驱动类型请求
type ChangeDiskBusRequest struct {
	Bus string `json:"bus" binding:"required"` // 新的总线类型: virtio/scsi/sata/ide
}

// ChangeDiskBus 修改磁盘驱动类型
func ChangeDiskBus(c *gin.Context) {
	name := c.Param("name")
	dev := c.Param("dev")
	if err := service.EnsureVMNotMigrating(name, "修改磁盘驱动类型"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}

	var req ChangeDiskBusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请指定驱动类型",
		})
		return
	}

	if err := service.SetDiskBus(name, dev, req.Bus); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "磁盘驱动类型修改成功",
	})
}

// ==================== CD/DVD 管理 ====================

// ChangeCDROMRequest 更换/插入 CD/DVD 请求
type ChangeCDROMRequest struct {
	ISOPath  string `json:"iso_path" binding:"required"`
	Device   string `json:"device"`    // 可选，不填自动查找
	ForceNew bool   `json:"force_new"` // 为 true 时强制新增光驱设备
}

// ChangeCDROM 更换/插入 CD/DVD
func ChangeCDROM(c *gin.Context) {
	name := c.Param("name")
	if err := service.EnsureVMNotMigrating(name, "更换光盘"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}
	var req ChangeCDROMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请指定 ISO 路径",
		})
		return
	}

	if req.ISOPath != "" {
		if _, err := os.Stat(req.ISOPath); os.IsNotExist(err) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": fmt.Sprintf("ISO文件不存在: %s", req.ISOPath)})
			return
		}
	}

	if err := service.ChangeCDROM(name, req.ISOPath, req.Device, req.ForceNew); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "光盘已插入",
	})
}

// EjectCDROM 弹出 CD/DVD
func EjectCDROM(c *gin.Context) {
	name := c.Param("name")
	device := c.Query("device") // 可选参数
	if err := service.EnsureVMNotMigrating(name, "弹出光盘"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}

	if err := service.EjectCDROM(name, device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "光盘已弹出",
	})
}

// RemoveCDROMHandler 移除 CD/DVD 设备
func RemoveCDROMHandler(c *gin.Context) {
	name := c.Param("name")
	device := c.Query("device") // 可选参数
	if err := service.EnsureVMNotMigrating(name, "移除光驱"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}

	if err := service.RemoveCDROM(name, device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "光驱已移除",
	})
}

// ==================== 软盘管理 ====================

// ChangeFloppyRequest 更换/插入软盘请求
type ChangeFloppyRequest struct {
	ImagePath string `json:"image_path" binding:"required"`
	Device    string `json:"device"`    // 可选，不填自动查找
	ForceNew  bool   `json:"force_new"` // 为 true 时强制新增软盘设备
}

// ChangeFloppy 更换/插入软盘
func ChangeFloppy(c *gin.Context) {
	name := c.Param("name")
	if err := service.EnsureVMNotMigrating(name, "更换软盘"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}
	var req ChangeFloppyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请指定软盘镜像路径",
		})
		return
	}

	if req.ImagePath != "" {
		if _, err := os.Stat(req.ImagePath); os.IsNotExist(err) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": fmt.Sprintf("软盘镜像文件不存在: %s", req.ImagePath)})
			return
		}
	}

	if err := service.ChangeFloppy(name, req.ImagePath, req.Device, req.ForceNew); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "软盘已插入",
	})
}

// EjectFloppy 弹出软盘
func EjectFloppy(c *gin.Context) {
	name := c.Param("name")
	device := c.Query("device") // 可选参数
	if err := service.EnsureVMNotMigrating(name, "弹出软盘"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}

	if err := service.EjectFloppy(name, device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "软盘已弹出",
	})
}

// RemoveFloppyHandler 移除软盘设备
func RemoveFloppyHandler(c *gin.Context) {
	name := c.Param("name")
	device := c.Query("device") // 可选参数
	if err := service.EnsureVMNotMigrating(name, "移除软盘"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}

	if err := service.RemoveFloppy(name, device); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "软盘已移除",
	})
}

// ImportDiskForVMRequest 为已有虚拟机导入磁盘请求
type ImportDiskForVMRequest struct {
	DiskPath       string `json:"disk_path"`
	DiskFile       string `json:"disk_file"`
	DiskSourceType string `json:"disk_source_type"`
	StoragePoolID  string `json:"storage_pool_id"`
	CopyDisk       bool   `json:"copy_disk"`
	Bus            string `json:"bus"`
}

// ImportDiskForVM 为已有虚拟机通过绝对路径导入磁盘（异步任务）
func ImportDiskForVM(c *gin.Context) {
	name := c.Param("name")
	if err := service.EnsureVMNotMigrating(name, "导入磁盘"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": err.Error()})
		return
	}
	var req ImportDiskForVMRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
		})
		return
	}

	username, _ := c.Get("username")
	usernameStr := username.(string)

	if req.Bus == "" {
		req.Bus = "virtio"
	}

	params := &vmimport.ImportDiskForExistingVMParams{
		VMName:         name,
		DiskPath:       req.DiskPath,
		DiskFile:       req.DiskFile,
		DiskSourceType: req.DiskSourceType,
		StoragePoolID:  req.StoragePoolID,
		CopyDisk:       req.CopyDisk,
		Bus:            req.Bus,
		Username:       usernameStr,
	}

	task, err := taskqueue.SubmitWithStruct(model.TaskTypeImportDiskAttach, params, usernameStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交导入磁盘任务失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "导入磁盘任务已提交",
		"data": gin.H{
			"task_id": task.ID,
		},
	})
}

// ==================== 磁盘 IOPS 限制 ====================

// SetDiskIOPSRequest 设置磁盘 IOPS 限制请求
type SetDiskIOPSRequest struct {
	TotalIopsSec int `json:"total_iops_sec"` // 总 IOPS 限制（0 表示不限制）
	ReadIopsSec  int `json:"read_iops_sec"`  // 读 IOPS 限制（0 表示不限制）
	WriteIopsSec int `json:"write_iops_sec"` // 写 IOPS 限制（0 表示不限制）
}

// SetDiskIOPS 设置虚拟机磁盘的 IOPS 限制（仅管理员）
func SetDiskIOPS(c *gin.Context) {
	name := c.Param("name")
	dev := c.Param("dev")
	if name == "" || dev == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "虚拟机名称和设备名不能为空",
		})
		return
	}

	var req SetDiskIOPSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	iops := &service.DiskIOPSTune{
		TotalIopsSec: req.TotalIopsSec,
		ReadIopsSec:  req.ReadIopsSec,
		WriteIopsSec: req.WriteIopsSec,
	}

	// 三个值都为 0 时清除 IOPS 限制
	if req.TotalIopsSec == 0 && req.ReadIopsSec == 0 && req.WriteIopsSec == 0 {
		iops = nil
	}

	if err := service.SetDiskIOPSTune(name, dev, iops); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "设置磁盘 IOPS 限制失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "磁盘 IOPS 限制已更新",
	})
}

// GetDiskIOPS 获取虚拟机磁盘的 IOPS 限制信息
func GetDiskIOPS(c *gin.Context) {
	name := c.Param("name")
	dev := c.Param("dev")
	if name == "" || dev == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "虚拟机名称和设备名不能为空",
		})
		return
	}

	iops, err := service.GetDiskIOPSTune(name, dev)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取磁盘 IOPS 信息失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "ok",
		"data":    iops,
	})
}
