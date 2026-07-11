package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"qvmhub/model"
)

// LXCAccessMiddleware 非 admin 用户操作容器时校验归属权限（键为容器名）。
// 与 VMAccessMiddleware 语义一致：admin 放行；否则按 LXCCache.OwnerUsername 比对当前用户。
func LXCAccessMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role == "admin" {
			c.Next()
			return
		}
		name := c.Param("name")
		// 没有 :name 的批量/列表类请求交给 handler 自行按属主过滤
		if name == "" {
			c.Next()
			return
		}
		username, _ := c.Get("username")
		var row model.LXCCache
		err := model.DB.Where("name = ?", name).First(&row).Error
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"code": 404, "message": "容器不存在"})
			return
		}
		if row.OwnerUsername != username.(string) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "无权操作该容器"})
			return
		}
		c.Next()
	}
}
