package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"qvmhub/config"
	"qvmhub/logger"
)

// 路径级危险模式
var dangerousPathPatterns = []string{
	"..",
	"%2e",
	"%252e",
	"%00",
	"\\",
}

// 扫描器探测路径（精确前缀匹配，lower-cased）
var probePaths = []string{
	"/.env",
	"/.git/",
	"/.htaccess",
	"/wp-admin",
	"/wp-login",
	"/phpinfo",
	"/actuator",
	"/debug/pprof",
	"/cgi-bin/",
	"/admin/config",
}

// 请求体危险模式
var dangerousBodyPatterns = []string{
	"../",
	"..\\",
	"/etc/passwd",
	"/etc/shadow",
	"/proc/self",
	"c:\\windows",
}

// 白名单路径（跳过 body 过滤）
var bodyFilterSkipPaths = []string{
	"/api/vm/",
	"/api/settings",
	"/api/storage-pool",
	"/api/template",
}

const (
	maxQueryParamValueLen = 2048
	maxBodySizeForFilter  = 1 << 20 // 1MB
)

// RequestFilterMiddleware 请求过滤中间件
func RequestFilterMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 开关检查
		if !config.GlobalConfig.RequestFilterEnabled {
			c.Next()
			return
		}

		// 2. URL 路径过滤
		path := c.Request.URL.Path
		rawPath := c.Request.URL.RawPath
		if rawPath == "" {
			rawPath = path
		}

		// 危险模式检查
		for _, pattern := range dangerousPathPatterns {
			if strings.Contains(strings.ToLower(rawPath), pattern) {
				rejectRequest(c, "路径包含危险模式: "+pattern)
				return
			}
		}

		// 双斜杠检查
		if strings.Contains(path, "//") {
			rejectRequest(c, "路径包含双斜杠")
			return
		}

		// 扫描器探测路径检查
		lowerPath := strings.ToLower(path)
		for _, probe := range probePaths {
			if strings.HasPrefix(lowerPath, probe) {
				rejectRequest(c, "扫描器探测路径: "+probe)
				return
			}
		}

		// 3. Query 参数过滤
		rawQuery := c.Request.URL.RawQuery
		lowerQuery := strings.ToLower(rawQuery)
		if strings.Contains(rawQuery, "../") ||
			strings.Contains(rawQuery, "..\\") ||
			strings.Contains(lowerQuery, "%2e%2e") ||
			strings.Contains(rawQuery, "%00") {
			rejectRequest(c, "查询参数包含路径遍历或空字节")
			return
		}

		// 单个参数值长度检查
		query := c.Request.URL.Query()
		for _, values := range query {
			for _, v := range values {
				if len(v) > maxQueryParamValueLen {
					rejectRequest(c, "查询参数值超长")
					return
				}
			}
		}

		// 4. 请求体过滤
		contentType := c.GetHeader("Content-Type")
		if strings.HasPrefix(contentType, "application/json") && c.Request.ContentLength > 0 && c.Request.ContentLength < maxBodySizeForFilter {
			// 白名单路径跳过
			shouldSkip := false
			for _, skip := range bodyFilterSkipPaths {
				if strings.HasPrefix(path, skip) {
					shouldSkip = true
					break
				}
			}

			if !shouldSkip {
				bodyBytes, err := io.ReadAll(c.Request.Body)
				if err != nil {
					// 读取失败不做阻断，回写空 body 继续
					c.Request.Body = io.NopCloser(bytes.NewReader(nil))
				} else {
					// 回写 body 供后续 handler 使用
					c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

					bodyStr := string(bodyBytes)
					lowerBody := strings.ToLower(bodyStr)
					for _, pattern := range dangerousBodyPatterns {
						if strings.Contains(lowerBody, strings.ToLower(pattern)) {
							rejectRequest(c, "请求体包含危险模式: "+pattern)
							return
						}
					}
				}
			}
		}

		// 5. Header 过滤
		for _, values := range c.Request.Header {
			for _, v := range values {
				if strings.Contains(v, "\x00") {
					rejectRequest(c, "请求头包含空字节")
					return
				}
			}
		}

		// Host 头检查
		host := c.Request.Header.Get("Host")
		if strings.ContainsAny(host, `<>"'`) {
			rejectRequest(c, "Host头包含非法字符")
			return
		}

		c.Next()
	}
}

// rejectRequest 统一拒绝请求：记录日志并返回 403
func rejectRequest(c *gin.Context, reason string) {
	logger.App.Warn("请求被过滤",
		"ip", c.ClientIP(),
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"reason", reason,
	)
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "Forbidden"})
}
