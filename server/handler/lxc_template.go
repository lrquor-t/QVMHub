package handler

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"qvmhub/model"
	"qvmhub/service"
	"qvmhub/service/lxc/template"
	"qvmhub/taskqueue"
)

type finalizeLXCTemplateReq struct {
	Name              string `json:"name" binding:"required"`
	DisplayName       string `json:"display_name"`
	Distro            string `json:"distro"`
	Release           string `json:"release"`
	Arch              string `json:"arch"`
	Description       string `json:"description"`
	SourcePath        string `json:"source_path"` // 上传落地的临时 tarball 路径
	HostPath          string `json:"host_path"`   // 或主机绝对路径
	PostCreateCommand string `json:"post_create_command"`
	Backing           string `json:"backing"` // dir / overlay / zfs（空=全局默认）
}

// FinalizeLXCTemplate 提交异步导入任务（由上传或主机路径的 tarball 创建模板）。
// 2GB 级 rootfs 的校验+解包耗时较长，走任务队列避免 HTTP 超时；进度见任务中心。
func FinalizeLXCTemplate(c *gin.Context) {
	var req finalizeLXCTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: name 必填"})
		return
	}
	src := req.SourcePath
	if src == "" {
		src = req.HostPath
	}
	if src != "" && req.HostPath != "" {
		if !filepath.IsAbs(req.HostPath) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "主机导入路径必须为绝对路径"})
			return
		}
		src = filepath.Clean(req.HostPath)
	}
	username, _ := c.Get("username")
	params := &template.ImportParams{
		Name: req.Name, DisplayName: req.DisplayName, Distro: req.Distro,
		Release: req.Release, Arch: req.Arch, Description: req.Description,
		SourcePath: src, PostCreateCommand: req.PostCreateCommand,
		Backing: req.Backing, OwnerUsername: username.(string),
	}
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeLXCTemplateImport, params, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "提交导入任务失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "导入任务已提交", "data": gin.H{"task_id": task.ID}})
}

type makeLXCTemplateReq struct {
	SrcName           string `json:"src_name" binding:"required"`
	Name              string `json:"name" binding:"required"`
	DisplayName       string `json:"display_name"`
	Description       string `json:"description"`
	PostCreateCommand string `json:"post_create_command"`
}

// MakeLXCTemplateFromContainer 从已停止的容器制作 LXC 模板（管理员）。
// 同步预校验（源存在/已停止/名合法不重名）后派发异步任务；大 rootfs 克隆走任务队列避免超时。
// POST /api/lxc/template/from-container
func MakeLXCTemplateFromContainer(c *gin.Context) {
	var req makeLXCTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: src_name 与 name 必填"})
		return
	}
	if err := template.ValidateMakeFromContainer(req.SrcName, req.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	username, _ := c.Get("username")
	params := &template.MakeTemplateParams{
		SrcName: req.SrcName, Name: req.Name, DisplayName: req.DisplayName,
		Description: req.Description, PostCreateCommand: req.PostCreateCommand,
		OwnerUsername: username.(string),
	}
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeLXCMkTemplate, params, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "提交制作模板任务失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "模板制作任务已提交", "data": gin.H{"task_id": task.ID, "template_name": req.Name}})
}

// ==================== LXC 模板分片上传 ====================

type lxcTemplateUploadInitReq struct {
	FileName  string `json:"file_name"`
	TotalSize int64  `json:"total_size"`
	FileHash  string `json:"file_hash"`
}

// LXCTemplateUploadInit 创建/恢复 LXC rootfs tarball 分片上传会话
// POST /api/lxc/template/upload/init
func LXCTemplateUploadInit(c *gin.Context) {
	var req lxcTemplateUploadInitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: 需要 file_name/total_size/file_hash"})
		return
	}
	username, _ := c.Get("username")
	res, err := service.InitLXCTemplateUpload(username.(string), req.FileName, req.TotalSize, req.FileHash)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": gin.H{
		"session_key":    res.SessionKey,
		"total_chunks":   res.TotalChunks,
		"chunk_size":     res.ChunkSize,
		"received":       res.Received,
		"uploaded_bytes": res.UploadedBytes,
		"instant":        res.Instant,
		"completed":      res.Completed,
	}})
}

// LXCTemplateUploadChunk 上传单个分片（复用通用 receiveChunk）
// POST /api/lxc/template/upload/chunk
func LXCTemplateUploadChunk(c *gin.Context) {
	if err := receiveChunk(c); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
}

// LXCTemplateUploadComplete 完成上传，返回 session_key(=落地路径) 供 finalize 使用
// POST /api/lxc/template/upload/complete
func LXCTemplateUploadComplete(c *gin.Context) {
	var req uploadCompleteReq
	if err := c.ShouldBindJSON(&req); err != nil || req.SessionKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少 session_key"})
		return
	}
	username, _ := c.Get("username")
	res, err := service.CompleteUpload(req.SessionKey, username.(string), req.FileHash)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if !res.Completed {
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"completed": false, "missing": res.Missing}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "上传成功", "data": gin.H{
		"completed":   true,
		"session_key": req.SessionKey,
	}})
}

// LXCTemplateUploadCancel 清理已上传的临时包与会话（关弹窗未导入时调用）
// POST /api/lxc/template/upload/cancel?path=<session_key>
// 用 POST 而非 DELETE：lxcTmpl 组已有 DELETE /:name，DELETE /upload 会与之通配冲突。
func LXCTemplateUploadCancel(c *gin.Context) {
	username, _ := c.Get("username")
	if err := service.RemoveUpload(c.Query("path"), username.(string)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
}

// ==================== Probe ====================

type probeLXCTemplateReq struct {
	SourcePath string `json:"source_path"` // 上传落地的临时 tarball 路径
	HostPath   string `json:"host_path"`   // 或主机绝对路径
}

// ProbeLXCTemplate 校验 tarball 结构并解析 os-release，供前端回填 distro/release
// POST /api/lxc/template/probe
func ProbeLXCTemplate(c *gin.Context) {
	var req probeLXCTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	src := strings.TrimSpace(req.SourcePath)
	if src == "" {
		src = strings.TrimSpace(req.HostPath)
		if src != "" && !filepath.IsAbs(src) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "主机导入路径必须为绝对路径"})
			return
		}
	}
	if src == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少 source_path 或 host_path"})
		return
	}
	// 快速探测：仅校验 rootfs/etc/os-release 存在并解析（不算 sha256、不遍历整包），大包秒级返回。
	distro, release, size, err := template.ProbeRootfsTarball(src)
	if err != nil {
		// 校验失败用 200 + ok=false 返回，便于前端读取中文错误
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"ok": false, "error": err.Error()}})
		return
	}
	// 架构由宿主机决定（跟随宿主机），随 probe 一并回填前端
	hostArch, _ := template.HostArchLXC()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"ok": true, "distro": distro, "release": release,
		"size_bytes": size, "arch": hostArch, "error": "",
	}})
}

func ListLXCTemplates(c *gin.Context) {
	rows, err := template.ListTemplates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取模板列表失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": rows})
}

func GetLXCTemplateDetail(c *gin.Context) {
	tpl, err := template.GetTemplate(c.Param("name"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": tpl})
}

func DeleteLXCTemplate(c *gin.Context) {
	if err := template.DeleteTemplate(c.Param("name")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
}

type updateLXCTemplateReq struct {
	DisplayName       string `json:"display_name"`
	Description       string `json:"description"`
	PostCreateCommand string `json:"post_create_command"`
	CloneVisible      bool   `json:"clone_visible"`
	Disabled          bool   `json:"disabled"`
}

// UpdateLXCTemplate 更新模板展示/管理元数据（管理员）
// PUT /api/lxc/template/:name
func UpdateLXCTemplate(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少模板名称"})
		return
	}
	var req updateLXCTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	// 输入校验在 handler 边界 → 400
	if len(strings.TrimSpace(req.DisplayName)) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "显示名称不能超过 255 字符"})
		return
	}
	if len(strings.TrimSpace(req.Description)) > 512 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "描述不能超过 512 字符"})
		return
	}
	params := &template.UpdateParams{
		DisplayName:       req.DisplayName,
		Description:       req.Description,
		PostCreateCommand: req.PostCreateCommand,
		CloneVisible:      req.CloneVisible,
		Disabled:          req.Disabled,
	}
	if err := template.UpdateTemplate(name, params); err != nil {
		if errors.Is(err, template.ErrTemplateNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新模板设置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "模板设置已更新"})
}
