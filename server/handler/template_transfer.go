package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"qvmhub/model"
	"qvmhub/service"
	"qvmhub/taskqueue"
)

// ExportTemplateHandler 导出模板
func ExportTemplateHandler(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少模板名称",
		})
		return
	}

	username, _ := c.Get("username")
	params := &service.ExportTemplateParams{
		TemplateName: name,
		Scope:        strings.TrimSpace(c.Query("scope")),
	}
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeTemplateExport, params, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交模板导出任务失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "模板导出任务已提交，成功后会生成 tar.gz 包并覆盖上一次导出文件",
		"data": gin.H{
			"task_id": task.ID,
		},
	})
}

func saveTemplateImportSource(c *gin.Context) (*service.ImportTemplateParams, *multipartFileInfo, error) {
	sourcePath := strings.TrimSpace(c.PostForm("source_path"))
	params := &service.ImportTemplateParams{
		TemplateName:  strings.TrimSpace(c.PostForm("template_name")),
		Type:          strings.TrimSpace(c.PostForm("type")),
		RootPassword:  c.PostForm("root_password"),
		TemplateUser:  c.PostForm("template_user"),
		CleanupSource: false,
	}
	file, header, err := c.Request.FormFile("file")
	if err == nil {
		defer file.Close()
		uploadName := filepath.Base(header.Filename)
		if strings.Contains(uploadName, "..") || strings.Contains(uploadName, "/") || strings.Contains(uploadName, "\\") {
			return nil, nil, fmt.Errorf("非法文件名")
		}
		if err := service.ValidateTemplateImportName(uploadName); err != nil {
			return nil, nil, err
		}
		importDir := service.GetTemplateImportTempDir()
		if err := os.MkdirAll(importDir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("创建导入目录失败: %w", err)
		}
		tempFile, err := os.CreateTemp(importDir, "template-upload-*")
		if err != nil {
			return nil, nil, fmt.Errorf("创建导入临时文件失败: %w", err)
		}
		tempPath := tempFile.Name()
		if _, err := io.Copy(tempFile, file); err != nil {
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
			return nil, nil, fmt.Errorf("保存模板文件失败: %w", err)
		}
		if err := tempFile.Close(); err != nil {
			_ = os.Remove(tempPath)
			return nil, nil, fmt.Errorf("关闭导入临时文件失败: %w", err)
		}
		params.SourcePath = tempPath
		params.SourceName = uploadName
		params.CleanupSource = true
		return params, &multipartFileInfo{Name: uploadName, Size: header.Size}, nil
	}
	if sourcePath == "" {
		return nil, nil, fmt.Errorf("请上传模板文件或填写主机绝对路径")
	}
	if !filepath.IsAbs(sourcePath) {
		return nil, nil, fmt.Errorf("主机导入路径必须为绝对路径")
	}
	if err := service.ValidateTemplateImportName(filepath.Base(sourcePath)); err != nil {
		return nil, nil, err
	}
	params.SourcePath = filepath.Clean(sourcePath)
	params.SourceName = filepath.Base(params.SourcePath)
	return params, nil, nil
}

type multipartFileInfo struct {
	Name string
	Size int64
}

// PreviewImportTemplateHandler 预览模板包导入链路
func PreviewImportTemplateHandler(c *gin.Context) {
	params, fileInfo, err := saveTemplateImportSource(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}
	preview, err := service.PreviewImportTemplate(c.Request.Context(), params)
	if err != nil {
		if params.CleanupSource {
			_ = os.Remove(params.SourcePath)
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}
	data := gin.H{"preview": preview}
	if fileInfo != nil {
		data["file"] = fmt.Sprintf("%s (%d bytes)", fileInfo.Name, fileInfo.Size)
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "模板包解析完成，请确认链路后导入",
		"data":    data,
	})
}

// ConfirmImportTemplateHandler 确认模板包导入
func ConfirmImportTemplateHandler(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少导入预览 token",
		})
		return
	}
	params, err := service.ResolveImportPreviewToken(req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}
	username, _ := c.Get("username")
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeTemplateImport, params, username.(string))
	if err != nil {
		if params.CleanupSource {
			_ = os.Remove(params.SourcePath)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交模板导入任务失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "模板导入任务已提交",
		"data": gin.H{
			"task_id": task.ID,
		},
	})
}

// DeleteExportedTemplateHandler 删除模板导出文件
func DeleteExportedTemplateHandler(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少模板名称",
		})
		return
	}

	if !service.HasExportedTemplate(name) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "模板导出文件不存在",
		})
		return
	}

	if err := service.DeleteExportedTemplate(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除模板导出文件失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "模板导出文件已删除",
	})
}

// ImportTemplateHandler 导入模板
func ImportTemplateHandler(c *gin.Context) {
	// 旧接口保留兼容：直接提交导入任务；新版页面使用 preview + confirm。
	templateName := strings.TrimSpace(c.PostForm("template_name"))
	if templateName != "" {
		if err := service.ValidateTemplateName(templateName); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": err.Error(),
			})
			return
		}
	}

	params, header, err := saveTemplateImportSource(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}
	if strings.HasSuffix(strings.ToLower(params.SourceName), ".qcow2") && templateName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "兼容导入 qcow2 时必须填写模板名称",
		})
		return
	}
	params.TemplateName = templateName

	username, _ := c.Get("username")
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeTemplateImport, params, username.(string))
	if err != nil {
		if params.CleanupSource {
			_ = os.Remove(params.SourcePath)
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "提交模板导入任务失败: " + err.Error(),
		})
		return
	}

	if header != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "模板导入任务已提交",
			"data": gin.H{
				"task_id": task.ID,
				"file":    fmt.Sprintf("%s (%d bytes)", header.Name, header.Size),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "模板导入任务已提交",
		"data": gin.H{
			"task_id":     task.ID,
			"source_path": params.SourcePath,
		},
	})
}

// DownloadTemplateExportHandler 下载导出的模板文件
func DownloadTemplateExportHandler(c *gin.Context) {
	filename := filepath.Base(strings.TrimSpace(c.Param("filename")))
	if filename == "" || filename == "." {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的文件名",
		})
		return
	}

	filePath := filepath.Join(service.GetTemplateExportDir(), filename)
	if _, err := os.Stat(filePath); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "导出文件不存在或已过期",
		})
		return
	}

	c.FileAttachment(filePath, filename)
}
