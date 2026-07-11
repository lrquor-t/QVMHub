package handler

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"qvmhub/config"
	"qvmhub/model"
	"qvmhub/service"
	"qvmhub/taskqueue"
)

type relocateLXCReq struct {
	NewLxcPath string `json:"new_lxc_path" binding:"required"`
	Migrate    bool   `json:"migrate"`
}

// LXCRelocateStorage 切换 LXC 容器目录：
//   - 无任何待搬目录 → 同步轻量切换（写 lxc.conf + 更新 config + 级联 import_dir + 同步缓存）。
//   - 有目录且 migrate=false → 返回 need_migrate（含容器/模板数）。
//   - 有目录且 migrate=true → 提交后台迁移任务，返回 task_id。
func LXCRelocateStorage(c *gin.Context) {
	var req relocateLXCReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: new_lxc_path 必填"})
		return
	}
	newPath := filepath.Clean(strings.TrimSpace(req.NewLxcPath))
	if !filepath.IsAbs(newPath) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "新容器目录必须是绝对路径"})
		return
	}
	cfg := config.GlobalConfig
	if newPath == filepath.Clean(cfg.LXCLxcPath) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "新容器目录与当前相同"})
		return
	}

	oldLxcPath := cfg.LXCLxcPath
	newImportDir := service.LXCCascadeImportDir(oldLxcPath, newPath, cfg.LXCTemplateImportDir)

	containers, templates, totalDirs, err := service.LXCEstimateRelocateTargets()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "探测容器/模板失败: " + err.Error()})
		return
	}

	// 无任何容器/模板目录 → 轻量切换
	if totalDirs == 0 {
		if err := service.LXCSwitchLxcPath(newPath, newImportDir); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "切换 LXC 目录失败: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 200, "message": "LXC 容器目录已切换", "data": gin.H{"migrated": false}})
		return
	}

	// 有数据但未确认迁移 → 返回 need_migrate
	if !req.Migrate {
		c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": gin.H{
			"need_migrate": true,
			"containers":   containers,
			"templates":    templates,
		}})
		return
	}

	// 确认迁移 → 提交后台任务
	username, _ := c.Get("username")
	params := service.LXCRelocateParams{
		NewLxcPath:   newPath,
		NewImportDir: newImportDir,
		OldLxcPath:   oldLxcPath,
		OldImportDir: cfg.LXCTemplateImportDir,
	}
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeLXCLxcRelocate, params, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "提交迁移任务失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "迁移任务已提交", "data": gin.H{"task_id": task.ID}})
}

// LXCStorageBackingInfo 返回 lxc 目录路径、是否在 zfs 上、默认 backing，供前端导入页给提示。
func LXCStorageBackingInfo(c *gin.Context) {
	cfg := config.GlobalConfig
	c.JSON(http.StatusOK, gin.H{
		"code": 200, "message": "ok",
		"data": gin.H{
			"lxcpath":         cfg.LXCLxcPath,
			"is_zfs":          service.LXCIsLxcpathZfs(cfg.LXCLxcPath),
			"default_backing": cfg.LXCDefaultBacking,
		},
	})
}
