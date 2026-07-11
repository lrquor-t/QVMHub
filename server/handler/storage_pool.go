package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"qvmhub/model"
	"qvmhub/service"
	"qvmhub/taskqueue"
)

// GetStoragePoolList 获取宿主机硬盘存储池列表
func GetStoragePoolList(c *gin.Context) {
	pools, err := service.ListStoragePools()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取存储池列表失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": pools})
}

// GetStoragePoolDetail 获取单个宿主机硬盘详情
func GetStoragePoolDetail(c *gin.Context) {
	id := c.Param("id")
	pool, err := service.GetStoragePool(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": pool})
}

// GetAllISOs 获取全局 ISO（聚合）
func GetAllISOs(c *gin.Context) {
	isos, err := service.GetAllISOs()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取 ISO 列表失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": isos})
}

// UpdateStoragePoolConfig 更新显示名称和启用状态
func UpdateStoragePoolConfig(c *gin.Context) {
	id := c.Param("id")
	var req service.UpdateHostStoragePoolConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if err := service.UpdateHostStoragePoolConfig(id, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "存储池配置已更新"})
}

// SetDefaultStoragePool 设置默认虚拟机存储位置
func SetDefaultStoragePool(c *gin.Context) {
	id := c.Param("id")
	if err := service.SetDefaultHostStoragePool(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已设为默认存储位置"})
}

// FormatMountStoragePool 提交格式化并挂载任务
func FormatMountStoragePool(c *gin.Context) {
	if !requireHighRiskVerification(c, "format_storage_pool") {
		return
	}
	id := c.Param("id")
	var req struct {
		FSType string `json:"fstype"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// 兼容旧版本不带 body 的请求
		req.FSType = ""
	}
	if req.FSType == "" {
		req.FSType = "ext4"
	}
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeStorageFormat, gin.H{"id": id, "fstype": req.FSType}, usernameStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交格式化任务失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "格式化并挂载任务已提交",
		"data":    gin.H{"task_id": task.ID},
	})
}

// CreateStoragePartition 提交创建分区任务
func CreateStoragePartition(c *gin.Context) {
	if !requireHighRiskVerification(c, "create_storage_partition") {
		return
	}
	id := c.Param("id")
	var req struct {
		SizeGB int `json:"size_gb"` // 分区大小(GB)，0 表示使用全部剩余空间
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeStorageCreatePartition, gin.H{
		"id":      id,
		"size_gb": req.SizeGB,
	}, usernameStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交创建分区任务失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "创建分区任务已提交",
		"data":    gin.H{"task_id": task.ID},
	})
}

// DeleteStoragePartitions 提交删除所有分区任务
func DeleteStoragePartitions(c *gin.Context) {
	if !requireHighRiskVerification(c, "delete_storage_partitions") {
		return
	}
	id := c.Param("id")
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeStorageDeletePartitions, gin.H{"id": id}, usernameStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交删除分区任务失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "删除分区任务已提交",
		"data":    gin.H{"task_id": task.ID},
	})
}

// GetAvailablePVTargets 获取可供 LVM 使用的磁盘列表
func GetAvailablePVTargets(c *gin.Context) {
	pools, err := service.GetAvailablePVTargets()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取可用磁盘列表失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": pools})
}

// CreateStorageVolume 提交创建 LVM 存储卷任务
func CreateStorageVolume(c *gin.Context) {
	if !requireHighRiskVerification(c, "create_storage_volume") {
		return
	}
	var req service.LVMVolumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)

	paramsJSON, _ := json.Marshal(req)
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeStorageCreateLVMVolume, gin.H{"params": string(paramsJSON)}, usernameStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交创建存储卷任务失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "创建 LVM 存储卷任务已提交",
		"data":    gin.H{"task_id": task.ID},
	})
}

// DeleteStorageVolume 提交删除 LVM 存储卷任务
func DeleteStorageVolume(c *gin.Context) {
	if !requireHighRiskVerification(c, "delete_storage_volume") {
		return
	}
	var req struct {
		VGName string `json:"vg_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	if strings.TrimSpace(req.VGName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "卷组名称不能为空"})
		return
	}
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeStorageDeleteLVMVolume, gin.H{"vg_name": req.VGName}, usernameStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交删除存储卷任务失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "删除 LVM 存储卷任务已提交",
		"data":    gin.H{"task_id": task.ID},
	})
}

// GetVMStorageTargets 获取创建虚拟机可选存储位置
func GetVMStorageTargets(c *gin.Context) {
	role, _ := c.Get("role")
	targets, err := service.ListVMStorageTargets(role == "admin")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取虚拟机存储位置失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": targets})
}

// GetZFSStatus 检测宿主机 ZFS 可用性
func GetZFSStatus(c *gin.Context) {
	available := service.ZFSAvailable()
	reason := "ok"
	if !available {
		reason = "未检测到 zfsutils-linux，请先安装并加载 ZFS 内核模块"
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": gin.H{
		"available": available,
		"reason":    reason,
	}})
}

// CreateZFSPool 提交创建 ZFS 存储池任务
func CreateZFSPool(c *gin.Context) {
	if !requireHighRiskVerification(c, "create_zfs_pool") {
		return
	}
	var req service.ZFSPoolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)

	paramsJSON, _ := json.Marshal(req)
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeStorageCreateZFSPool, gin.H{"params": string(paramsJSON)}, usernameStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交创建 ZFS 存储池任务失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "创建 ZFS 存储池任务已提交",
		"data":    gin.H{"task_id": task.ID},
	})
}

type createZFSDatasetReq struct {
	Pool string `json:"pool" binding:"required"`
	Name string `json:"name" binding:"required"`
}

// CreateZFSDataset 在已有 ZFS 存储池下创建数据集（如 zp01/vm-storage）。
// POST /api/storage-pool/zfs-dataset
func CreateZFSDataset(c *gin.Context) {
	var req createZFSDatasetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: 需要指定 pool 和 name"})
		return
	}
	if err := service.CreateZFSDataset(req.Pool, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "数据集 " + req.Pool + "/" + req.Name + " 已创建"})
}

type deleteZFSDatasetReq struct {
	Name string `json:"name" binding:"required"`
}

// DeleteZFSDataset 删除 ZFS 数据集。
// DELETE /api/storage-pool/zfs-dataset
func DeleteZFSDataset(c *gin.Context) {
	var req deleteZFSDatasetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: 需要指定 name"})
		return
	}
	if err := service.DeleteZFSDataset(req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "数据集 " + req.Name + " 已删除"})
}

// DeleteZFSPool 提交销毁 ZFS 存储池任务
func DeleteZFSPool(c *gin.Context) {
	if !requireHighRiskVerification(c, "delete_zfs_pool") {
		return
	}
	var req struct {
		PoolName string `json:"pool_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	if strings.TrimSpace(req.PoolName) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "存储池名称不能为空"})
		return
	}
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeStorageDeleteZFSPool, gin.H{"pool_name": req.PoolName}, usernameStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交销毁 ZFS 存储池任务失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "销毁 ZFS 存储池任务已提交",
		"data":    gin.H{"task_id": task.ID},
	})
}

type expandZFSPoolReq struct {
	Pool      string   `json:"pool" binding:"required"`
	VdevType  string   `json:"vdev_type" binding:"required"`
	DeviceIDs []string `json:"device_ids"`
}

// ExpandZFSPool 给已存在 zpool 加同类型 vdev 扩容（同步，高风险验证）。
// POST /api/storage-pool/expand-zfs-pool
func ExpandZFSPool(c *gin.Context) {
	if !requireHighRiskVerification(c, "expand_zfs_pool") {
		return
	}
	var req expandZFSPoolReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: 需指定 pool/vdev_type/device_ids"})
		return
	}
	if err := service.AddZFSVdevs(req.Pool, req.VdevType, req.DeviceIDs); err != nil {
		// zpool 命令失败（服务端）→ 500；其余（池不存在/类型不一致/磁盘不足/已用）→ 400
		if strings.HasPrefix(err.Error(), "zpool add 失败") {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "存储池 " + req.Pool + " 已扩容"})
}

// zfsPoolNameReq scrub/clear 动作的 pool 参数。
type zfsPoolNameReq struct {
	Pool string `json:"pool" binding:"required"`
}

func resolvePoolFromQuery(c *gin.Context) (string, bool) {
	pool := strings.TrimSpace(c.Query("pool"))
	if pool == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少 pool 参数"})
		return "", false
	}
	return pool, true
}

// GetZFSScrubStatus GET /api/storage-pool/zfs-scrub/status?pool=
func GetZFSScrubStatus(c *gin.Context) {
	pool, ok := resolvePoolFromQuery(c)
	if !ok {
		return
	}
	status, err := service.GetZFSScrubStatus(pool)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": status})
}

// StartZFSScrub POST /api/storage-pool/zfs-scrub/start
func StartZFSScrub(c *gin.Context) {
	var req zfsPoolNameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: 需要指定 pool"})
		return
	}
	if err := service.StartZFSScrub(req.Pool); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "scrub 已启动: " + req.Pool})
}

// StopZFSScrub POST /api/storage-pool/zfs-scrub/stop
func StopZFSScrub(c *gin.Context) {
	var req zfsPoolNameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: 需要指定 pool"})
		return
	}
	if err := service.StopZFSScrub(req.Pool); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "scrub 已停止: " + req.Pool})
}

// ClearZFSErrors POST /api/storage-pool/zfs-clear-errors
func ClearZFSErrors(c *gin.Context) {
	var req zfsPoolNameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: 需要指定 pool"})
		return
	}
	if err := service.ClearZFSErrors(req.Pool); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "错误计数已清除: " + req.Pool})
}

// GetZFSErrors GET /api/storage-pool/zfs-errors?pool=
func GetZFSErrors(c *gin.Context) {
	pool, ok := resolvePoolFromQuery(c)
	if !ok {
		return
	}
	list, err := service.GetZFSErrors(pool)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": list})
}

// GetZFSPropertiesHandler GET /api/storage-pool/zfs-property?dataset=
func GetZFSPropertiesHandler(c *gin.Context) {
	ds := strings.TrimSpace(c.Query("dataset"))
	if ds == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少 dataset 参数"})
		return
	}
	info, err := service.GetZFSProperties(ds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": info})
}

type setZFSPropertyReq struct {
	Dataset  string `json:"dataset" binding:"required"`
	Property string `json:"property" binding:"required"`
	Value    string `json:"value" binding:"required"`
}

// SetZFSPropertyHandler PUT /api/storage-pool/zfs-property
func SetZFSPropertyHandler(c *gin.Context) {
	var req setZFSPropertyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: 需 dataset/property/value"})
		return
	}
	// 先校验（→400），再执行（→500）：校验错与命令错分别给码
	if err := service.ValidateZFSProperty(req.Property, req.Value); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := service.SetZFSProperty(req.Dataset, req.Property, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": req.Property + " 已更新"})
}
