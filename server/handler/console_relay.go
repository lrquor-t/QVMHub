package handler

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"qvmhub/model"
)

// wsUpgrader 把浏览器连接升级为 WebSocket。与节点侧一致使用 binary 子协议(noVNC 需要)。
var wsUpgrader = websocket.Upgrader{
	Subprotocols:    []string{"binary"},
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// isConsoleWSPath 判定是否为需要 WS 中继的控制台路径(设计 §6.1/§6.3)。
//
//	/api/vm/<name>/vnc/ws        —— VNC
//	/api/lxc/<name>/terminal/ws  —— LXC 终端
func isConsoleWSPath(p string) bool {
	if strings.HasSuffix(p, "/vnc/ws") && strings.HasPrefix(p, "/api/vm/") {
		return true
	}
	if strings.HasSuffix(p, "/terminal/ws") && strings.HasPrefix(p, "/api/lxc/") {
		return true
	}
	return false
}

// relayConsoleWS 浏览器 ↔ 节点 的双向 WS 管道:升级浏览器端、以 WS 客户端拨向节点
// (附 admin Key 头,丢弃控制器的 ?token=)、双向转发 binary 帧。
func relayConsoleWS(c *gin.Context, node *model.HostNode, apiKey, proxyPath string) {
	browser, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade 已写入错误响应;不可再写 c.JSON。
		return
	}
	defer browser.Close()

	// 拨向节点:把 base URL 的 http(s):// 换成 ws(s)://,不转发控制器 query(token 仅控制器用)。
	nodeURL := toWSURL(node.APIBaseURL, proxyPath)
	header := http.Header{}
	header.Set("X-API-Key-ID", node.APIKeyID)
	header.Set("X-API-Key", apiKey)

	dialer := websocket.Dialer{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}
	remote, _, err := dialer.Dial(nodeURL, header)
	if err != nil {
		_ = browser.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "无法连接节点控制台: "+err.Error()))
		return
	}
	defer remote.Close()

	pipeWS(browser, remote)
}

// pipeWS 双向转发两个 WS 连接,任一方向断开即关闭两端。
func pipeWS(a, b *websocket.Conn) {
	var once sync.Once
	done := make(chan struct{})
	closeBoth := func() {
		once.Do(func() {
			_ = a.Close()
			_ = b.Close()
			close(done)
		})
	}
	go relayWSMessages(b, a, closeBoth) // a -> b
	go relayWSMessages(a, b, closeBoth) // b -> a
	<-done
}

// relayWSMessages 从 src 读消息写到 dst,出错时调用 onDone 关闭两端。
func relayWSMessages(dst, src *websocket.Conn, onDone func()) {
	defer onDone()
	for {
		mt, data, err := src.ReadMessage()
		if err != nil {
			return
		}
		if err := dst.WriteMessage(mt, data); err != nil {
			return
		}
	}
}

// toWSURL 把节点 HTTP base URL 换成 WS scheme 并拼上路径。
func toWSURL(base, path string) string {
	u := strings.TrimRight(base, "/") + path
	u = strings.Replace(u, "https://", "wss://", 1)
	u = strings.Replace(u, "http://", "ws://", 1)
	return u
}
