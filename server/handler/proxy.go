package handler

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"qvmhub/model"
	"qvmhub/service/host"
	"qvmhub/service/nodereg"
)

// proxyClient 是通用代理使用的 HTTP 客户端。45s 超时与 CallNodeAPI 一致(§7)。
// proxyStreamClient 无超时,用于 SSE/流式:依赖请求 context 在浏览器断开时取消。
var (
	proxyClient       = &http.Client{Timeout: 45 * time.Second}
	proxyStreamClient = &http.Client{} // SSE/长流:不设整体超时
)

// loadProxyNode 解析 nodeId、加载节点、校验启用、内存解密 admin Key。
// 失败时写入 HTTP 错误并返回 ok=false(成功前响应未被劫持,可安全写 JSON)。
func loadProxyNode(c *gin.Context) (*model.HostNode, string, bool) {
	nodeID, err := strconv.ParseUint(c.Param("nodeId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的节点 ID"})
		return nil, "", false
	}
	node, err := host.GetHostNode(uint(nodeID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "节点不存在", "node_id": nodeID})
		return nil, "", false
	}
	if !node.Enabled {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": "节点已禁用", "node_id": node.ID})
		return nil, "", false
	}
	apiKey, err := host.DecryptHostNodeAPIKey(*node)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "节点凭证解密失败"})
		return nil, "", false
	}
	// 记录 Key 被使用(仅内存,§5.6 last_used_at 可见性)。
	nodereg.GlobalHealthCache.TouchUsed(node.ID)
	return node, apiKey, true
}

// ProxyNode 是通用反向代理:剥 /api/n/{nodeId} 前缀、附节点 admin API Key、转发并把响应原样回传。
// 设计 §3.1:浏览器把请求打到 /api/n/{nodeId}/api/...,控制器透传到节点。
// 控制台路径(/api/vm/<name>/vnc/ws、/api/lxc/<name>/terminal/ws)走 WS 中继(§6.1/§6.3)。
func ProxyNode(c *gin.Context) {
	proxyPath := c.Param("proxyPath")

	if isConsoleWSPath(proxyPath) {
		node, apiKey, ok := loadProxyNode(c)
		if !ok {
			return
		}
		relayConsoleWS(c, node, apiKey, proxyPath)
		return
	}

	node, apiKey, ok := loadProxyNode(c)
	if !ok {
		return
	}

	// proxyPath 形如 "/api/vm/list"(含前导斜杠),直接拼到节点 base URL 之后。
	target := strings.TrimRight(node.APIBaseURL, "/") + proxyPath
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

	// SSE/流式:用无超时客户端 + 分块刷新,避免 45s 超时切断长连接与缓冲滞后(§3.2/§7)。
	streaming := isStreamRequest(c.Request)
	client := proxyClient
	if streaming {
		client = proxyStreamClient
	}

	resp, err := client.Do(outReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"code": 502, "message": "节点不可达", "node_id": node.ID, "error": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	// 原样回传:状态码 + 响应头 + body。Blob/JSON 用缓冲拷贝;SSE 用分块刷新。
	dst := c.Writer.Header()
	for k, vs := range resp.Header {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
	// 流式响应也以节点返回的 Content-Type 为准(通常 text/event-stream)。
	c.Writer.WriteHeader(resp.StatusCode)
	copyResponse(c.Writer, resp.Body, streaming)
}

// copyResponse 把节点响应体拷给浏览器。flush=true 时按小块读取并逐块 Flush(SSE 实时性)。
func copyResponse(w http.ResponseWriter, body io.Reader, flush bool) {
	if !flush {
		_, _ = io.Copy(w, body)
		return
	}
	fl, _ := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
			if fl != nil {
				fl.Flush()
			}
		}
		if err != nil {
			break
		}
	}
}

// isStreamRequest 判定是否按流式处理:路径含 /sse、/events,或客户端 Accept text/event-stream。
func isStreamRequest(r *http.Request) bool {
	if strings.Contains(r.URL.Path, "/sse") || strings.Contains(r.URL.Path, "/events") {
		return true
	}
	return strings.Contains(r.Header.Get("Accept"), "text/event-stream")
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
