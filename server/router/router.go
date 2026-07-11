package router

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"qvmhub/config"
	"qvmhub/handler"
	"qvmhub/logger"
	"qvmhub/middleware"
)

// Setup 初始化路由
func Setup() *gin.Engine {
	r := gin.New()

	// 配置可信代理
	if len(config.GlobalConfig.TrustedProxies) > 0 {
		r.SetTrustedProxies(config.GlobalConfig.TrustedProxies)
	} else {
		r.SetTrustedProxies(nil) // 不信任任何代理头
	}

	r.Use(middleware.RequestLoggerMiddleware(), middleware.SafeRecoveryMiddleware())

	// 全局中间件
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.SecurityHeadersMiddleware())
	r.Use(middleware.RequestFilterMiddleware())
	r.Use(middleware.RequestGuardMiddleware())

	// 全局 API 限频
	rlConfig := middleware.RateLimitConfig{
		PublicPerMinute: config.GlobalConfig.RateLimitPublicPerMin,
		AuthPerMinute:   config.GlobalConfig.RateLimitAuthPerMin,
		CleanupInterval: 5 * time.Minute,
	}
	rateLimiter := middleware.NewRateLimiter(rlConfig)
	r.Use(middleware.RateLimitMiddleware(rateLimiter))

	// API 路由组（controller-native：仅 public / auth / settings / nodes / user）
	api := r.Group("/api")
	{
		api.GET("/public/settings", handler.GetPublicSettings)
		api.GET("/public/version", handler.GetVersion)

		// ==================== 认证（无需登录）：login / 找回密码 / 邀请 / breach ====================
		auth := api.Group("/auth")
		{
			auth.POST("/login", handler.Login)
			auth.GET("/invite", handler.GetInviteInfo)
			auth.POST("/invite/complete", handler.CompleteInvite)
			auth.POST("/password/forgot", handler.ForgotPassword)
			auth.POST("/password/forgot/send-code", handler.ForgotPasswordSendCode)
			auth.POST("/password/forgot/verify-code", handler.ForgotPasswordVerifyCode)
			auth.POST("/password/forgot/select-account", handler.ForgotPasswordSelectAccount)
			auth.POST("/password/reset", handler.ResetPasswordByEmail)
			auth.POST("/check-password", handler.CheckPasswordBreach)
		}

		// ==================== 登录中间态验证 ====================
		loginAuth := auth.Group("/login")
		loginAuth.Use(middleware.TokenTypeMiddleware("login"))
		{
			loginAuth.POST("/email/send", handler.SendLoginEmailCode)
			loginAuth.POST("/verify", handler.VerifyLoginStage)
		}

		// ==================== 安全初始化 / 绑定 / 2FA ====================
		secureAuth := auth.Group("")
		secureAuth.Use(middleware.JWTTokenTypeMiddleware("access", "bootstrap"))
		{
			secureAuth.POST("/email/code/send", handler.SendEmailCode)
			secureAuth.POST("/email/bind", handler.BindEmail)
			secureAuth.POST("/2fa/setup", handler.SetupTOTP)
			secureAuth.POST("/2fa/enable", handler.EnableTOTP)
			secureAuth.POST("/2fa/disable", handler.DisableTOTP)
			secureAuth.POST("/2fa/recovery/regen", handler.RegenRecoveryCodes)
			secureAuth.POST("/skip-bootstrap", handler.SkipBootstrap) // 管理员跳过安全初始化
		}

		// ==================== 高风险验证（本人已登录） ====================
		highRiskAuth := auth.Group("")
		highRiskAuth.Use(middleware.AuthMiddleware())
		highRiskAuth.Use(middleware.ForcePasswordChangeMiddleware())
		{
			highRiskAuth.GET("/info", handler.GetUserInfo)
			highRiskAuth.GET("/api-key", handler.GetAPIKeyInfo)
			highRiskAuth.POST("/api-key", handler.RotateAPIKey)
			highRiskAuth.DELETE("/api-key", handler.RevokeAPIKey)
			highRiskAuth.PUT("/password", handler.ChangePassword)
			highRiskAuth.PUT("/username", handler.ChangeUsername)
			highRiskAuth.POST("/high-risk/verify", handler.VerifyHighRisk)
		}

		// ==================== 系统设置（运维：access/bootstrap 均可） ====================
		settings := api.Group("/settings")
		settings.Use(middleware.TokenTypeMiddleware("access", "bootstrap"), middleware.AdminMiddleware())
		{
			settings.GET("", handler.GetSettings)
			settings.PUT("", handler.UpdateSettings)
			settings.POST("/jwt-secret/rotate", handler.RotateJWTSecret)
			settings.GET("/log/status", handler.GetLogStatus)
			settings.POST("/log/delete", handler.DeleteLogs)
			settings.POST("/log/export", handler.ExportLogs)
		}

		// ==================== 需要认证的路由 ====================
		authorized := api.Group("")
		authorized.Use(middleware.AuthMiddleware())
		authorized.Use(middleware.ForcePasswordChangeMiddleware())
		{
			// 总览页:所有登录用户可读(跨节点只读,viewer 亦可见);只读健康缓存 + DB 名册。
			authorized.GET("/overview", handler.GetOverview)

			// 通用代理 /api/n/{nodeId}/* → 透传到节点(JWT 已由 authorized 组强制);
			// P5:代理层 RBAC(admin 全权;非 admin 跨节点只读,控制台 WS 拒绝)。
			nodeProxy := authorized.Group("/n")
			nodeProxy.Use(middleware.ProxyRBACMiddleware())
			nodeProxy.Any("/:nodeId/*proxyPath", handler.ProxyNode)

			// ==================== 节点注册表（管理员） ====================
			// 复用 QVMConsole 的 HostNode CRUD；P1 起被探活/健康缓存复用。
			nodes := authorized.Group("/nodes")
			nodes.Use(middleware.AdminMiddleware())
			{
				nodes.GET("", handler.ListHostNodes)
				nodes.POST("", handler.CreateHostNode)
				nodes.GET("/:id/migration-options", handler.GetNodeMigrationOptions)
				nodes.PUT("/:id", handler.UpdateHostNode)
				nodes.DELETE("/:id", handler.DeleteHostNode)
				// HTTP-only 健康探活(替代原 SSH 能力探活;QVMHub 从不 SSH,§5.1)。
				nodes.POST("/:id/probe", handler.RefreshNodeHealth)
			}

			// ==================== 运维用户管理（管理员） ====================
			// 管 QVMHub 自有运维账号；不是节点侧 elastic/lightweight 用户。
			user := authorized.Group("/user")
			user.Use(middleware.AdminMiddleware())
			{
				user.GET("/list", handler.GetUserList)
				user.POST("", handler.CreateUser)
				user.PUT("/:username/status", handler.UpdateUserStatus)
				// 注：brief 提到的 admin 重置用户密码路由因无对应 handler 暂未注册（P0 不强求）。
				user.DELETE("/:username", handler.DeleteUser)
			}
		}
	}

	// ==================== 前端静态文件服务（生产环境） ====================
	setupStaticFileServing(r)

	return r
}

// setupStaticFileServing 配置前端静态文件服务
// 当 web-dist 目录存在时，自动提供前端文件，支持 Vue SPA 路由回退
func setupStaticFileServing(r *gin.Engine) {
	// 获取可执行文件所在目录
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	execDir := filepath.Dir(execPath)
	webDistDir := filepath.Join(execDir, "web-dist")

	// 检查 web-dist 目录是否存在
	if _, err := os.Stat(webDistDir); os.IsNotExist(err) {
		// 也尝试相对于工作目录查找
		webDistDir = "web-dist"
		if _, err := os.Stat(webDistDir); os.IsNotExist(err) {
			logger.App.Info("未找到 web-dist 目录，跳过前端静态文件服务（开发环境请使用 vite dev）")
			return
		}
	}

	absWebDistDir, _ := filepath.Abs(webDistDir)
	logger.App.Info("启用前端静态文件服务", "dir", absWebDistDir)

	// 提供静态资源文件（CSS/JS/图片等）—— 带 hash 的资源可长缓存
	r.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		}
		c.Next()
	})
	r.Static("/assets", filepath.Join(absWebDistDir, "assets"))

	// 提供根目录下的静态文件（favicon 等）
	r.StaticFile("/favicon.svg", filepath.Join(absWebDistDir, "favicon.svg"))
	r.StaticFile("/icons.svg", filepath.Join(absWebDistDir, "icons.svg"))

	// SPA 回退：所有非 API 路由都返回 index.html
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// API 路由不回退
		if strings.HasPrefix(path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Not Found"})
			return
		}

		// 安全路径校验：null byte 检查（必须在 Clean 之前）
		if strings.ContainsRune(path, 0) {
			c.JSON(http.StatusForbidden, gin.H{"error": "非法路径"})
			return
		}
		cleanPath := filepath.Clean(path)
		safePath := filepath.Join(absWebDistDir, cleanPath)
		if !strings.HasPrefix(safePath, absWebDistDir+string(filepath.Separator)) && safePath != absWebDistDir {
			c.JSON(http.StatusForbidden, gin.H{"error": "非法路径"})
			return
		}

		// 尝试提供静态文件
		if _, err := os.Stat(safePath); err == nil {
			c.File(safePath)
			return
		}

		// SPA 回退到 index.html
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.File(filepath.Join(absWebDistDir, "index.html"))
	})
}
