package handler

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"qvmhub/service/host"
)

// proxyClient 是通用代理使用的 HTTP 客户端。45s 超时与 CallNodeAPI 一致;
// SSE/长连接在 P3 用独立无超时客户端处理。
var proxyClient = &http.Client{Timeout: 45 * time.Second}

// ProxyNode 是通用反向代理:剥 /api/n/{nodeId} 前缀、附节点 admin API Key、转发并把响应原样回传。
// 设计 §3.1:浏览器把请求打到 /api/n/{nodeId}/api/...,控制器透传到节点。
func ProxyNode(c *gin.Context) {
	nodeID, err := strconv.ParseUint(c.Param("nodeId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的节点 ID"})
		return
	}

	node, err := host.GetHostNode(uint(nodeID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "节点不存在", "node_id": nodeID})
		return
	}
	if !node.Enabled {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": "节点已禁用", "node_id": node.ID})
		return
	}

	apiKey, err := host.DecryptHostNodeAPIKey(*node)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "节点凭证解密失败"})
		return
	}

	// proxyPath 形如 "/api/vm/list"(含前导斜杠),直接拼到节点 base URL 之后。
	target := strings.TrimRight(node.APIBaseURL, "/") + c.Param("proxyPath")
	if raw := c.Request.URL.RawQuery; raw != "" {
		target += "?" + raw
	}

	outReq, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, target, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "构造代理请求失败"})
		return
	}
	copyProxyHeaders(outReq.Header, c.Request.Header)
	outReq.Header.Set("X-API-Key-ID", node.APIKeyID)
	outReq.Header.Set("X-API-Key", apiKey)

	resp, err := proxyClient.Do(outReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"code": 502, "message": "节点不可达", "node_id": node.ID, "error": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	// 原样回传:状态码 + 响应头 + body(JSON 透传;流式/SSE 在 P3 增加分块刷新)。
	dst := c.Writer.Header()
	for k, vs := range resp.Header {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}

// copyProxyHeaders 拷贝出站需要的请求头,丢弃控制器侧鉴权头(JWT/Cookie 不得泄露给节点)
// 与 hop-by-hop 头;X-API-Key* 由代理显式覆写。
func copyProxyHeaders(dst, src http.Header) {
	for k, vs := range src {
		if isProxyHopHeader(k) {
			continue
		}
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

// isProxyHopHeader 判断该头是否不应转发到节点。
func isProxyHopHeader(h string) bool {
	switch http.CanonicalHeaderKey(h) {
	case "Authorization", "Cookie",
		"X-Api-Key", "X-Api-Key-Id", // 代理自行覆写
		"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
		"Te", "Trailers", "Transfer-Encoding", "Upgrade", "Host":
		return true
	}
	return false
}
