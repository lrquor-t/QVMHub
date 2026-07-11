package middleware

import (
	"time"

	"qvmhub/logger"

	"github.com/gin-gonic/gin"
)

// RequestLoggerMiddleware 自定义 GIN 请求日志中间件，使用 slog 按状态码分级记录
func RequestLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		// 从 context 获取用户名（JWT 中间件设置的）
		username, _ := c.Get("username")
		user := ""
		if username != nil {
			if u, ok := username.(string); ok {
				user = u
			}
		}

		if raw != "" {
			path = path + "?" + raw
		}

		// 根据状态码选择日志级别
		if status >= 500 {
			logger.Request.Error("请求处理",
				"method", method,
				"path", path,
				"status", status,
				"latency", latency.String(),
				"ip", clientIP,
				"user", user,
				"errors", c.Errors.ByType(gin.ErrorTypePrivate).String(),
			)
		} else if status >= 400 {
			logger.Request.Warn("请求处理",
				"method", method,
				"path", path,
				"status", status,
				"latency", latency.String(),
				"ip", clientIP,
				"user", user,
			)
		} else {
			logger.Request.Info("请求处理",
				"method", method,
				"path", path,
				"status", status,
				"latency", latency.String(),
				"ip", clientIP,
				"user", user,
			)
		}
	}
}
