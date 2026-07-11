package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"qvmhub/service"
	"qvmhub/service/nodereg"
)

// GetOverview 返回所有节点的健康一览(只读内存缓存 + DB 节点名册,零请求时扇出)。
// 设计 §5.3:节点列表 + 在线状态 + 资源概要;挂掉节点显示 last_seen。
func GetOverview(c *gin.Context) {
	nodes, err := service.ListHostNodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取节点列表失败: " + err.Error()})
		return
	}

	out := make([]nodereg.NodeHealth, 0, len(nodes))
	online := 0
	for _, nv := range nodes {
		h, ok := nodereg.GlobalHealthCache.Get(nv.ID)
		if !ok {
			// 尚未被后台探活命中:用 DB 侧状态合成一条(通常 status=unknown)。
			h = nodereg.NodeHealth{
				NodeID:     nv.ID,
				Name:       nv.Name,
				APIBaseURL: nv.APIBaseURL,
				Enabled:    nv.Enabled,
				Status:     nv.Status,
			}
		}
		// 身份/启用态以 DB 为准(可能刚被改过);禁用节点一律按 offline 显示。
		h.Name = nv.Name
		h.APIBaseURL = nv.APIBaseURL
		h.Enabled = nv.Enabled
		if !nv.Enabled {
			h.Online = false
			h.Status = nodereg.StatusDisabled
		}
		// §5.6:admin Key 最近被代理使用的时刻(仅内存)。
		if lu, ok := nodereg.GlobalHealthCache.LastUsed(nv.ID); ok {
			lu := lu
			h.LastUsedAt = &lu
		}
		if h.Online {
			online++
		}
		out = append(out, h)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"nodes":  out,
			"total":  len(out),
			"online": online,
		},
	})
}

// RefreshNodeHealth 立即对单个节点做 HTTP-only 健康探活(管理员触发),返回最新快照。
func RefreshNodeHealth(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的节点 ID"})
		return
	}
	h, err := nodereg.GlobalScheduler.RefreshNodeByID(uint(id), true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "探活失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "节点探活完成", "data": h})
}
