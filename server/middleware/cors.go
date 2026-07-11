package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"qvmhub/config"
)

// CORSMiddleware 跨域中间件，支持白名单模式
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowedOrigin := getAllowedOrigin(origin)

		if allowedOrigin != "" {
			c.Header("Access-Control-Allow-Origin", allowedOrigin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, Accept, X-Requested-With, X-API-Key-ID, X-API-ID, X-API-Key, X-KVM-API-Key-ID, X-KVM-API-Key, X-High-Risk-Token")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type")
		c.Header("Access-Control-Max-Age", "86400")
		if allowedOrigin != "" && allowedOrigin != "*" {
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// getAllowedOrigin 根据配置决定允许的Origin
func getAllowedOrigin(origin string) string {
	// 开发模式或未配置白名单时保持 *
	if config.GlobalConfig.DevelopmentMode || config.GlobalConfig.CORSAllowedOrigins == "" {
		return "*"
	}
	allowed := strings.Split(config.GlobalConfig.CORSAllowedOrigins, ",")
	for _, a := range allowed {
		if strings.TrimSpace(a) == origin {
			return origin
		}
	}
	// 不匹配时不设置 Allow-Origin（浏览器会拒绝）
	return ""
}
