package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"qvmhub/router"
	"qvmhub/service"
)

// wsEchoStub 起一个「节点」WS 桩:升级后原样回发收到的消息,并把请求头投递到 hdrCh(供断言 admin Key)。
func wsEchoStub(t *testing.T) (*httptest.Server, <-chan http.Header) {
	t.Helper()
	hdrCh := make(chan http.Header, 1)
	up := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case hdrCh <- r.Header.Clone():
		default:
		}
		ws, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		for {
			mt, data, err := ws.ReadMessage()
			if err != nil {
				return
			}
			if err := ws.WriteMessage(mt, data); err != nil {
				return
			}
		}
	}))
	return srv, hdrCh
}

// TestConsoleWSRelay 是 P4 的端到端验收:浏览器 WS → 控制器中继 → 节点 WS,
// 验证 VNC 路径的双向 binary 帧透传 + 节点收到 admin Key(且不收到控制器 ?token)。
func TestConsoleWSRelay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initTestStack(t)

	stub, hdrCh := wsEchoStub(t)
	defer stub.Close()

	view, err := service.CreateHostNode(service.HostNodeRequest{
		Name: "ws-node", APIBaseURL: stub.URL, APIKeyID: "kid", APIKey: "plaintext-admin-key",
	})
	require.NoError(t, err)
	nodeID := view.ID

	r := router.Setup()
	ts := httptest.NewServer(r)
	defer ts.Close()
	token := loginAsAdmin(t, ts)

	// 浏览器以 ?token= 连控制器 VNC WS。
	wsURL := strings.Replace(ts.URL, "http://", "ws://", 1) +
		"/api/n/" + itoa(nodeID) + "/api/vm/test-vm/vnc/ws?token=" + token
	browser, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "WS 中继升级应成功")
	defer browser.Close()

	// 发一帧 binary,经控制器中继到节点桩,再原样回。
	payload := []byte("hello-vnc")
	require.NoError(t, browser.WriteMessage(websocket.BinaryMessage, payload))
	_, data, err := browser.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, payload, data)

	// 节点应看到 admin Key,且不得看到控制器 JWT(?token 不转发给节点)。
	var seen http.Header
	select {
	case seen = <-hdrCh:
	default:
		t.Fatal("节点未收到 WS 握手")
	}
	require.Equal(t, "kid", seen.Get("X-Api-Key-Id"))
	require.Equal(t, "plaintext-admin-key", seen.Get("X-Api-Key"))
	require.Empty(t, seen.Get("Authorization"))
}

// TestConsoleWSRelayLXCTerminal 验证 LXC 终端路径同样命中 WS 中继(isConsoleWSPath)。
func TestConsoleWSRelayLXCTerminal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initTestStack(t)

	stub, _ := wsEchoStub(t)
	defer stub.Close()

	view, err := service.CreateHostNode(service.HostNodeRequest{
		Name: "ws-node-lxc", APIBaseURL: stub.URL, APIKeyID: "kid", APIKey: "k",
	})
	require.NoError(t, err)

	r := router.Setup()
	ts := httptest.NewServer(r)
	defer ts.Close()
	token := loginAsAdmin(t, ts)

	wsURL := strings.Replace(ts.URL, "http://", "ws://", 1) +
		"/api/n/" + itoa(view.ID) + "/api/lxc/ct1/terminal/ws?token=" + token
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer c.Close()

	require.NoError(t, c.WriteMessage(websocket.BinaryMessage, []byte("ls")))
	_, data, err := c.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, []byte("ls"), data)
}
