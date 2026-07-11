package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ProxyRBACMiddleware 强制代理层 RBAC 策略(设计 §5.5)。
//
//	- admin:全部方法透传。
//	- 非 admin(viewer 等):仅 GET/HEAD/OPTIONS(跨节点只读);写方法 → 403。
//	- 控制台 WS(VNC / LXC 终端)即便以 GET 升级也视为「控制」操作,对非 admin 一律 403
//	  —— 只读账号不得拿到交互式控制台。
//
// 节点侧新增接口自动落入「写方法受限」策略,无需逐接口登记。
func ProxyRBACMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role == "admin" {
			c.Next()
			return
		}
		if isConsoleWSRequestPath(c.Request.URL.Path) {
			denyProxy(c, "只读账号不允许访问节点控制台")
			return
		}
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
		default:
			denyProxy(c, "只读账号(viewer)不允许执行写操作")
		}
	}
}

func denyProxy(c *gin.Context, msg string) {
	c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": msg})
	c.Abort()
}

// isConsoleWSRequestPath 判定完整请求路径是否为控制台 WS(/api/n/{id}/api/vm/.../vnc/ws 等)。
func isConsoleWSRequestPath(path string) bool {
	return (strings.HasSuffix(path, "/vnc/ws") && strings.Contains(path, "/api/vm/")) ||
		(strings.HasSuffix(path, "/console/ws") && strings.Contains(path, "/api/lxc/"))
}
