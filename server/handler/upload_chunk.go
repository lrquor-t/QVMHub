package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"qvmhub/service"
)

// maxChunkBody 单个分片请求体上限（含 multipart 字段开销）。防御恶意大请求。
const maxChunkBody = 4 << 20 // 4MB

// ==================== 用户存储（iso / share / disk）====================

type storageUploadInitReq struct {
	Category  string `json:"category"`
	FileName  string `json:"file_name"`
	TotalSize int64  `json:"total_size"`
	FileHash  string `json:"file_hash"`
}

// UserStorageUploadInit 创建/恢复用户存储分片上传会话（含秒传判断）
// POST /api/self/storage/upload/init
func UserStorageUploadInit(c *gin.Context) {
	var req storageUploadInitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: 需要 category/file_name/total_size/file_hash"})
		return
	}
	username, _ := c.Get("username")
	res, err := service.InitUserStorageUpload(username.(string), req.Category, req.FileName, req.TotalSize, req.FileHash)
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

// UserStorageUploadChunk 上传单个分片
// POST /api/self/storage/upload/chunk  (multipart: file, session_key, index)
func UserStorageUploadChunk(c *gin.Context) {
	if err := receiveChunk(c); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
}

// UserStorageUploadComplete 全部分片到齐后校验整文件并完成
// POST /api/self/storage/upload/complete
type uploadCompleteReq struct {
	SessionKey string `json:"session_key"`
	FileHash   string `json:"file_hash"`
}

func UserStorageUploadComplete(c *gin.Context) {
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
		// 分片未到齐：返回缺失清单，前端补传后重试，而非直接失败
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"completed": false, "missing": res.Missing}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "文件上传成功", "data": gin.H{"completed": true}})
}

// UserStorageUploadStatus 查询上传进度（断点续传/继续上传用）
// GET /api/self/storage/upload/status?path=<session_key>
func UserStorageUploadStatus(c *gin.Context) {
	username, _ := c.Get("username")
	res, err := service.GetUploadStatus(c.Query("path"), username.(string))
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"exists":         res.Exists,
		"status":         res.Status,
		"total_chunks":   res.TotalChunks,
		"chunk_size":     res.ChunkSize,
		"received":       res.Received,
		"uploaded_bytes": res.UploadedBytes,
	}})
}

// UserStorageUploadCancel 取消上传，删除未完成文件与会话
// DELETE /api/self/storage/upload?path=<session_key>
func UserStorageUploadCancel(c *gin.Context) {
	username, _ := c.Get("username")
	if err := service.CancelUpload(c.Query("path"), username.(string)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已取消上传"})
}

// UserStorageUploadPending 列出用户存储未完成的上传会话（主动恢复用）
// GET /api/self/storage/upload/pending
func UserStorageUploadPending(c *gin.Context) {
	username, _ := c.Get("username")
	list, err := service.ListUserStoragePending(username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "查询未完成上传失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": list})
}

// ==================== 模板导入 ====================

type templateUploadInitReq struct {
	FileName  string `json:"file_name"`
	TotalSize int64  `json:"total_size"`
	FileHash  string `json:"file_hash"`
}

// TemplateUploadInit 创建/恢复模板包分片上传会话（目标为导入临时目录）
// POST /api/template/upload/init
func TemplateUploadInit(c *gin.Context) {
	var req templateUploadInitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: 需要 file_name/total_size/file_hash"})
		return
	}
	username, _ := c.Get("username")
	res, err := service.InitTemplateUpload(username.(string), req.FileName, req.TotalSize, req.FileHash)
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

// TemplateUploadChunk 上传单个模板分片
// POST /api/template/upload/chunk
func TemplateUploadChunk(c *gin.Context) {
	if err := receiveChunk(c); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
}

// TemplateUploadComplete 完成模板包上传，返回临时路径供 preview/confirm 使用
// POST /api/template/upload/complete
func TemplateUploadComplete(c *gin.Context) {
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
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "模板包上传成功", "data": gin.H{
		"completed":   true,
		"session_key": req.SessionKey,
	}})
}

// TemplateUploadCancel 清理已上传的模板包临时文件与会话（预览后未导入而关闭对话框时调用）
// DELETE /api/template/upload?path=<session_key>
func TemplateUploadCancel(c *gin.Context) {
	username, _ := c.Get("username")
	if err := service.RemoveUpload(c.Query("path"), username.(string)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
}

// ==================== 共用：接收分片 ====================

// receiveChunk 解析单个分片请求并写入目标文件。
func receiveChunk(c *gin.Context) error {
	// 限制单次分片请求体大小，防止滥用
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxChunkBody)

	sessionKey := c.PostForm("session_key")
	indexStr := c.PostForm("index")
	if sessionKey == "" || indexStr == "" {
		return errors.New("缺少 session_key 或 index")
	}
	index, err := strconv.Atoi(indexStr)
	if err != nil || index < 0 {
		return errors.New("无效的分片号")
	}
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		return errors.New("未收到分片数据: " + err.Error())
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return errors.New("读取分片失败: " + err.Error())
	}

	username, _ := c.Get("username")
	return service.SaveUploadChunk(sessionKey, username.(string), index, 0, data)
}
