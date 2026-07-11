package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"qvmhub/service"
)

// LXCDownloadList 返回 lxc-create -t download 可用镜像清单（distro/release/arch）。
// GET /api/lxc/download/list
func LXCDownloadList(c *gin.Context) {
	list, err := service.LXCDownloadList()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": list})
}
