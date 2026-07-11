package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"qvmhub/logger"
	"qvmhub/service/lxc"
	"qvmhub/utils"
)

// resizeFrame 终端 resize 控制帧（TextMessage JSON）。
type resizeFrame struct {
	Action string `json:"action"` // "resize"
	Cols   int    `json:"cols"`
	Rows   int    `json:"rows"`
}

// LxcConsoleWS LXC 容器 Web 终端：WS ↔ lxc-attach PTY 双向转发。
// 协议：TextMessage=控制帧(JSON resize)；BinaryMessage=原始字节（WS→Ptmx 为 stdin，Ptmx→WS 为输出）。
func LxcConsoleWS(c *gin.Context) {
	name := c.Param("name")

	sess, err := lxc.StartAttach(name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "启动容器终端失败: " + err.Error()})
		return
	}
	defer sess.Close()

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.App.Warn("LXC 终端 WebSocket 升级失败", "error", err)
		return
	}
	defer ws.Close()

	logger.App.Info("LXC 终端已建立", "container", name)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// WS → Ptmx（处理 TextMessage resize + BinaryMessage stdin）
	go func() {
		defer utils.RecoverAndLog("lxc-ws-to-pty")
		defer cancel()
		for {
			msgType, msg, err := ws.ReadMessage()
			if err != nil {
				return
			}
			switch msgType {
			case websocket.TextMessage:
				var f resizeFrame
				if json.Unmarshal(msg, &f) == nil && f.Action == "resize" {
					_ = sess.Resize(f.Cols, f.Rows)
				}
			case websocket.BinaryMessage:
				if _, err := sess.Ptmx.Write(msg); err != nil {
					return
				}
			}
		}
	}()

	// Ptmx → WS（PTY 输出）
	go func() {
		defer utils.RecoverAndLog("lxc-pty-to-ws")
		defer cancel()
		buf := make([]byte, 32*1024)
		for {
			n, err := sess.Ptmx.Read(buf)
			if n > 0 {
				if werr := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	<-ctx.Done()
	ws.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "终端关闭"))
	logger.App.Info("LXC 终端断开", "container", name)
}
