package handler

import (
	"context"
	"io"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"qvmhub/logger"
	"qvmhub/service"
	"qvmhub/utils"
)

// WebSocket 升级器
var upgrader = websocket.Upgrader{
	Subprotocols: []string{"binary"}, // noVNC 需要 binary 子协议
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源（生产环境应限制）
	},
}

// VncPasswordRequest VNC 密码请求
type VncPasswordRequest struct {
	Password string `json:"password"`
}

// GetVncStatus 获取 VNC 状态
func GetVncStatus(c *gin.Context) {
	name := c.Param("name")

	info, err := service.GetVncStatus(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "ok",
		"data":    info,
	})
}

// EnableVnc 开启 VNC
func EnableVnc(c *gin.Context) {
	name := c.Param("name")
	var req VncPasswordRequest
	c.ShouldBindJSON(&req) // 密码可选

	if err := service.EnableVnc(name, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "开启 VNC 失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "VNC 已开启",
	})
}

// DisableVnc 关闭 VNC
func DisableVnc(c *gin.Context) {
	name := c.Param("name")

	if err := service.DisableVnc(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "关闭 VNC 失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "VNC 已关闭",
	})
}

// ChangeVncPassword 修改 VNC 密码
func ChangeVncPassword(c *gin.Context) {
	name := c.Param("name")
	var req VncPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请输入新密码",
		})
		return
	}

	if err := service.ChangeVncPassword(name, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "修改密码失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "VNC 密码已修改",
	})
}

// VncExposeRequest VNC 暴露请求
type VncExposeRequest struct {
	Expose bool `json:"expose"`
}

// ExposeVnc 切换 VNC 对外暴露状态
func ExposeVnc(c *gin.Context) {
	name := c.Param("name")
	var req VncExposeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数无效",
		})
		return
	}

	if err := service.ExposeVnc(name, req.Expose); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "操作失败: " + err.Error(),
		})
		return
	}

	msg := "VNC 已关闭对外暴露"
	if req.Expose {
		msg = "VNC 已开启对外暴露"
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": msg,
	})
}

// VncWebSocket noVNC WebSocket 代理
// 前端 noVNC 通过 WebSocket 连接此接口 → 后端转发到 VM 的 VNC（Unix Socket 或 TCP）
func VncWebSocket(c *gin.Context) {
	name := c.Param("name")

	// 获取 VNC 连接信息（自动检测 Unix Socket / TCP）
	connInfo, err := service.GetVncConnInfo(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	// 升级为 WebSocket
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.App.Warn("WebSocket 升级失败", "error", err)
		return
	}
	defer ws.Close()

	// 连接 VNC（支持 unix socket 和 tcp）
	vncConn, err := net.Dial(connInfo.Network, connInfo.Address)
	if err != nil {
		logger.App.Warn("连接 VNC 失败", "error", err, "network", connInfo.Network, "addr", connInfo.Address)
		ws.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "无法连接 VNC"))
		return
	}
	defer vncConn.Close()

	logger.App.Info("VNC WebSocket 代理已建立", "vm", name, "network", connInfo.Network, "addr", connInfo.Address)

	// 创建可取消上下文，任一方向断开时取消另一方向
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// 双向转发
	errChan := make(chan error, 2)

	// WebSocket → VNC
	go func() {
		defer utils.RecoverAndLog("vnc-ws-to-vnc")
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			_, message, err := ws.ReadMessage()
			if err != nil {
				select {
				case errChan <- err:
				default:
				}
				return
			}
			if _, err := vncConn.Write(message); err != nil {
				select {
				case errChan <- err:
				default:
				}
				return
			}
		}
	}()

	// VNC → WebSocket
	go func() {
		defer utils.RecoverAndLog("vnc-vnc-to-ws")
		defer cancel()
		buf := make([]byte, 32*1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := vncConn.Read(buf)
			if err != nil {
				if err != io.EOF {
					select {
					case errChan <- err:
					default:
					}
				}
				return
			}
			if err := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				select {
				case errChan <- err:
				default:
				}
				return
			}
		}
	}()

	// 等待任一方向断开
	<-errChan
	logger.App.Info("VNC WebSocket 代理断开", "vm", name)
}
