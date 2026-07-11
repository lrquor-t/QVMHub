package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"qvmhub/model"
	"qvmhub/service"
	"qvmhub/taskqueue"
)

func GetOVSStatus(c *gin.Context) {
	status, err := service.GetOVSStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": status})
}

func GetOVSPorts(c *gin.Context) {
	ports, err := service.GetOVSPorts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": ports})
}

func GetOVSLeases(c *gin.Context) {
	leases, err := service.GetOVSLeasesStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": leases})
}

func CheckOVSNetwork(c *gin.Context) {
	result, err := service.CheckOVSNetwork()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": result})
}

func RepairOVSNetwork(c *gin.Context) {
	if !requireHighRiskVerification(c, "repair_ovs_network") {
		return
	}
	username, _ := c.Get("username")
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeOVSRepair, map[string]string{"action": "repair"}, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "OVS 网络修复任务已提交", "data": task})
}

func GetVMNetworkRuntimeStatus(c *gin.Context) {
	status, err := service.GetVMNetworkRuntimeStatus(c.Param("name"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": status})
}
