package main

import (
	"fmt"
	"os"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/router"
)

// Version 构建时注入:go build -ldflags="-s -w -X main.Version=v0.1.0"
var Version = "dev"

func main() {
	// 初始化配置
	config.Init()

	// 初始化日志
	logger.InitWithConsoleConfig(
		config.GlobalConfig.LogDir,
		config.GlobalConfig.LogLevel,
		config.GlobalConfig.LogMaxDays,
		config.GlobalConfig.LogCompress,
		config.GlobalConfig.LogConsole,
		config.GlobalConfig.LogConsoleTypes,
		config.GlobalConfig.LogConsoleLevel,
		config.GlobalConfig.LogMaxSizeMB,
		config.GlobalConfig.LogMaxBackups,
	)
	defer logger.Close()
	logger.App.Info("配置初始化完成")

	// 初始化数据库(含 AutoMigrate + 默认 admin seed)
	model.InitDB()

	// 从 DB 加载持久化系统设置,覆盖环境变量默认值
	if saved, err := model.GetAllSettings(); err == nil && len(saved) > 0 {
		config.GlobalConfig.LoadFromDB(saved)
		logger.App.Info("已从数据库加载持久化系统设置", "count", len(saved))
	}

	// 安全检查
	config.ValidateSecurity()

	// 路由 + 启动
	r := router.Setup()
	addr := fmt.Sprintf(":%d", config.GlobalConfig.Port)
	logger.App.Info("QVMHub 服务启动", "addr", addr, "version", Version)
	if err := r.Run(addr); err != nil {
		logger.App.Error("服务启动失败", "error", err)
		os.Exit(1)
	}
}
