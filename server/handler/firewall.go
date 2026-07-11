package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"qvmhub/model"
	"qvmhub/service"
	"qvmhub/taskqueue"
)

func GetFirewallStatus(c *gin.Context) {
	status, err := service.GetFirewallStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": status})
}

func GetFirewallPolicy(c *gin.Context) {
	policy, err := service.GetFirewallPolicy()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": policy})
}

func SaveFirewallPolicy(c *gin.Context) {
	var policy service.FirewallPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if err := service.SaveFirewallPolicy(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "防火墙策略已保存"})
}

func PreviewFirewallPolicy(c *gin.Context) {
	var policy service.FirewallPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	rules, err := service.PreviewFirewallRules(&policy)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": gin.H{"rules": rules}})
}

type firewallApplyRequest struct {
	Policy *service.FirewallPolicy `json:"policy"`
}

func ApplyFirewallPolicy(c *gin.Context) {
	if !requireHighRiskVerification(c, "apply_firewall") {
		return
	}
	var req firewallApplyRequest
	_ = c.ShouldBindJSON(&req)
	policy := req.Policy
	if policy == nil {
		current, err := service.GetFirewallPolicy()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
			return
		}
		policy = current
	}
	username, _ := c.Get("username")
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeApplyFirewall, policy, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "防火墙应用任务已提交", "data": task})
}

func DisableFirewall(c *gin.Context) {
	if !requireHighRiskVerification(c, "disable_firewall") {
		return
	}
	username, _ := c.Get("username")
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeDisableFirewall, service.FirewallOperationParams{Action: "disable"}, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "防火墙禁用任务已提交", "data": task})
}

func RollbackFirewall(c *gin.Context) {
	if !requireHighRiskVerification(c, "rollback_firewall") {
		return
	}
	username, _ := c.Get("username")
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeRollbackFirewall, service.FirewallOperationParams{Action: "rollback"}, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "防火墙回滚任务已提交", "data": task})
}

func ImportFirewallRegion(c *gin.Context) {
	var req service.FirewallImportParams
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	policy, err := service.ImportFirewallRegionCIDRs(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "区域 CIDR 已导入", "data": policy})
}

func UpdateFirewallGeoIP(c *gin.Context) {
	var req service.FirewallGeoUpdateParams
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	username, _ := c.Get("username")
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeUpdateFirewallGeoIP, req, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "GeoIP 更新任务已提交", "data": task})
}

type portForwardFirewallRequest struct {
	Key    string `json:"key" binding:"required"`
	Exempt bool   `json:"exempt"`
}

func SetPortForwardFirewall(c *gin.Context) {
	var req portForwardFirewallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	policy, err := service.SetPortForwardFirewallExemption(req.Key, req.Exempt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "端口转发区域限制已更新", "data": policy})
}

func GetHostFirewallStatus(c *gin.Context) {
	status, err := service.GetHostFirewallStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": status})
}

func PreviewEnableHostFirewall(c *gin.Context) {
	var req service.HostFirewallEnableRequest
	_ = c.ShouldBindJSON(&req)
	status, err := service.PreviewEnableHostFirewall(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": status})
}

func EnableHostFirewall(c *gin.Context) {
	if !requireHighRiskVerification(c, "enable_host_firewall") {
		return
	}
	var req service.HostFirewallEnableRequest
	_ = c.ShouldBindJSON(&req)
	username, _ := c.Get("username")
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeEnableHostFirewall, req, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "宿主机防火墙启用任务已提交", "data": task})
}

func DisableHostFirewall(c *gin.Context) {
	if !requireHighRiskVerification(c, "disable_host_firewall") {
		return
	}
	username, _ := c.Get("username")
	task, err := taskqueue.SubmitWithStruct(model.TaskTypeDisableHostFirewall, service.FirewallOperationParams{Action: "disable_host"}, username.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "宿主机防火墙关闭任务已提交", "data": task})
}

func ListHostFirewallRules(c *gin.Context) {
	rules, err := service.ListHostFirewallRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": rules})
}

func CreateHostFirewallRule(c *gin.Context) {
	if !requireHighRiskVerification(c, "create_host_firewall_rule") {
		return
	}
	var req service.HostFirewallRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	rule, err := service.AddHostFirewallRule(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "宿主机防火墙规则已添加", "data": rule})
}

func UpdateHostFirewallRule(c *gin.Context) {
	if !requireHighRiskVerification(c, "update_host_firewall_rule") {
		return
	}
	var req service.HostFirewallRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	rule, err := service.UpdateHostFirewallRule(c.Param("id"), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "宿主机防火墙规则已更新", "data": rule})
}

func DeleteHostFirewallRule(c *gin.Context) {
	if !requireHighRiskVerification(c, "delete_host_firewall_rule") {
		return
	}
	if err := service.DeleteHostFirewallRule(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "宿主机防火墙规则已删除"})
}

func AddHostFirewallVNCDefaultRule(c *gin.Context) {
	if !requireHighRiskVerification(c, "add_host_firewall_vnc_default") {
		return
	}
	rule, err := service.AddHostFirewallVNCDefaultRule()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "VNC 默认端口范围已放通", "data": rule})
}

func PreviewHostFirewallConnections(c *gin.Context) {
	preview, err := service.PreviewHostFirewallConnections(c.Query("mode"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": preview})
}

func CloseHostFirewallConnections(c *gin.Context) {
	if !requireHighRiskVerification(c, "close_host_firewall_connections") {
		return
	}
	var req service.HostFirewallCloseConnectionsRequest
	_ = c.ShouldBindJSON(&req)
	count, err := service.CloseHostFirewallConnections(req.Mode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "连接关闭命令已执行", "data": gin.H{"count": count}})
}
