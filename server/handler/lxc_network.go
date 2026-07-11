package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"qvmhub/service"
)

// ListLXCInterfaces 列出容器全部网卡。
func ListLXCInterfaces(c *gin.Context) {
	list, err := service.LXCListContainerInterfaces(c.Param("name"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": list})
}

// AddLXCInterface 给容器追加一块网卡（仅管理员）。
func AddLXCInterface(c *gin.Context) {
	var req service.LXCAddInterfaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if err := service.LXCAddContainerInterface(c.Param("name"), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	_ = service.ApplyVPCACLRules() // SG 引擎重建（含新 SecurityGroupID）
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
}

// UpdateLXCInterface 编辑网卡（仅管理员）。
func UpdateLXCInterface(c *gin.Context) {
	order, err := strconv.Atoi(c.Param("order"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "order 参数错误"})
		return
	}
	var req service.LXCAddInterfaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if err := service.LXCUpdateContainerInterface(c.Param("name"), order, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	_ = service.ApplyVPCACLRules()
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
}

type removeLXCInterfaceReq struct {
	Force bool `json:"force"`
}

// RemoveLXCInterface 删除网卡（仅管理员；order==0 需 force）。
func RemoveLXCInterface(c *gin.Context) {
	order, err := strconv.Atoi(c.Param("order"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "order 参数错误"})
		return
	}
	var req removeLXCInterfaceReq
	_ = c.ShouldBindJSON(&req) // body 可选
	if err := service.LXCRemoveContainerInterface(c.Param("name"), order, req.Force); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	_ = service.ApplyVPCACLRules()
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
}
