package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"qvmhub/model"
	"qvmhub/service"
	"qvmhub/taskqueue"
)

func GetVMNetworkDiagnostics(c *gin.Context) {
	result, err := service.GetVMNetworkDiagnostics(c.Param("name"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": result})
}

func StartVMNetworkCapture(c *gin.Context) {
	if !requireHighRiskVerification(c, "network_capture") {
		return
	}
	var req service.NetworkCaptureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	vmName := c.Param("name")
	username, _ := c.Get("username")
	params := service.NetworkCaptureParams{
		VMName:                vmName,
		NetworkCaptureRequest: req,
	}
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeNetworkCapture, params, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "提交抓包任务失败: " + err.Error()})
		return
	}
	service.InitNetworkCaptureSession(task.ID, vmName, req, username.(string))
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "网络抓包任务已提交",
		"data": gin.H{
			"task_id": task.ID,
		},
	})
}

func GetNetworkCaptureSession(c *gin.Context) {
	taskID, ok := parseTaskIDParam(c)
	if !ok {
		return
	}
	session, found := service.GetNetworkCaptureSession(taskID)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "抓包任务尚未开始或已过期"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": session})
}

func DownloadNetworkCapture(c *gin.Context) {
	taskID, ok := parseTaskIDParam(c)
	if !ok {
		return
	}
	filePath, fileName, err := service.NetworkCaptureFilePath(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.FileAttachment(filePath, fileName)
}

func DeleteNetworkCapture(c *gin.Context) {
	taskID, ok := parseTaskIDParam(c)
	if !ok {
		return
	}
	if err := service.DeleteNetworkCaptureFile(taskID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "pcap 文件已删除"})
}

func parseTaskIDParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("task_id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的任务 ID"})
		return 0, false
	}
	return uint(id), true
}
