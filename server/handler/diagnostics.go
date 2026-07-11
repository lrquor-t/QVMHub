package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"qvmhub/service/diagnostics"
)

// DiagnosisExportRequest 诊断导出请求
type DiagnosisExportRequest struct {
	Categories []string `json:"categories" binding:"required"`
}

// GetDiagnosticCategories 返回可用诊断类别
func GetDiagnosticCategories(c *gin.Context) {
	categories := diagnostics.GetAvailableCategories()
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "ok",
		"data":    categories,
	})
}

// ExportDiagnostics 收集诊断信息并返回 ZIP
func ExportDiagnostics(c *gin.Context) {
	var req DiagnosisExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误：请选择要收集的诊断类别"})
		return
	}

	if len(req.Categories) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "至少需要选择一个诊断类别"})
		return
	}

	// 收集诊断信息
	buf, err := diagnostics.CollectDiagnostics(req.Categories)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "收集诊断信息失败: " + err.Error()})
		return
	}

	// 生成导出文件名
	exportName := fmt.Sprintf("qvmconsole-diagnostics-%s.zip", time.Now().Format("20060102-150405"))

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, exportName))
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}
