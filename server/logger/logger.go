package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// 导出的 Logger 实例
var (
	App     *slog.Logger // 通用日志
	Request *slog.Logger // HTTP 请求日志
	CMD     *slog.Logger // 命令执行日志
	Libvirt *slog.Logger // go-libvirt RPC 日志
)

// 保留 lumberjack Writer 引用，用于轮转和关闭
var (
	appWriter     *lumberjack.Logger
	requestWriter *lumberjack.Logger
	cmdWriter     *lumberjack.Logger
	libvirtWriter *lumberjack.Logger
	allWriters    []*lumberjack.Logger
)

// Init 初始化日志系统
func Init(logDir string, level string, maxDays int, compress bool, console bool, maxSizeMB int) {
	InitWithConsoleConfig(logDir, level, maxDays, compress, console, "", "", maxSizeMB, 0)
}

// LogDir 返回日志目录路径（由 config 传入）
var logDirPath string

// GetLogDir 获取日志目录路径
func GetLogDir() string {
	return logDirPath
}

// InitWithConsoleConfig 初始化日志系统（支持终端输出类型和级别配置）
// consoleTypes: 逗号分隔的终端输出类型（app,request,cmd,libvirt），为空表示全部
// consoleLevel: 终端独立日志级别，为空则跟随文件级别
func InitWithConsoleConfig(logDir string, level string, maxDays int, compress bool, console bool, consoleTypes string, consoleLevel string, maxSizeMB int, maxBackups int) {
	// 记录日志目录路径
	logDirPath = logDir

	// 自动创建日志目录
	if err := os.MkdirAll(logDir, 0755); err != nil {
		panic("failed to create log directory: " + err.Error())
	}

	// 解析日志级别
	slogLevel := parseLevel(level)

	// 解析终端日志级别（为空则跟随文件级别）
	consoleSlogLevel := slogLevel
	if consoleLevel != "" {
		consoleSlogLevel = parseLevel(consoleLevel)
	}

	// 解析终端输出类型
	consoleTypeSet := parseConsoleTypes(consoleTypes)

	// 创建 lumberjack Writer
	appWriter = newLumberjackWriter(filepath.Join(logDir, "app.log"), maxSizeMB, maxDays, compress, maxBackups)
	requestWriter = newLumberjackWriter(filepath.Join(logDir, "request.log"), maxSizeMB, maxDays, compress, maxBackups)
	cmdWriter = newLumberjackWriter(filepath.Join(logDir, "cmd.log"), maxSizeMB, maxDays, compress, maxBackups)
	libvirtWriter = newLumberjackWriter(filepath.Join(logDir, "libvirt.log"), maxSizeMB, maxDays, compress, maxBackups)

	allWriters = []*lumberjack.Logger{appWriter, requestWriter, cmdWriter, libvirtWriter}

	// 创建 Logger 实例，根据类型配置决定是否输出到终端
	App = newLoggerWithConfig(appWriter, slogLevel, console && consoleTypeSet["app"], consoleSlogLevel)
	Request = newLoggerWithConfig(requestWriter, slogLevel, console && consoleTypeSet["request"], consoleSlogLevel)
	CMD = newLoggerWithConfig(cmdWriter, slogLevel, console && consoleTypeSet["cmd"], consoleSlogLevel)
	Libvirt = newLoggerWithConfig(libvirtWriter, slogLevel, console && consoleTypeSet["libvirt"], consoleSlogLevel)

	// 启动定时轮转
	startDailyRotation()
}

// newLumberjackWriter 创建一个配置好的 lumberjack Writer
func newLumberjackWriter(filename string, maxSizeMB int, maxAge int, compress bool, maxBackups int) *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxSizeMB,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   compress,
		LocalTime:  true,
	}
}

// newLoggerWithConfig 创建 Logger，支持终端独立级别
func newLoggerWithConfig(lj *lumberjack.Logger, fileLevel slog.Level, console bool, consoleLevel slog.Level) *slog.Logger {
	if !console {
		// 仅文件输出
		handler := slog.NewTextHandler(lj, &slog.HandlerOptions{
			Level: fileLevel,
		})
		return slog.New(handler)
	}

	// 同时输出到文件和终端，可能使用不同级别
	if consoleLevel == fileLevel {
		// 级别相同，用 MultiWriter
		writer := io.MultiWriter(os.Stdout, lj)
		handler := slog.NewTextHandler(writer, &slog.HandlerOptions{
			Level: fileLevel,
		})
		return slog.New(handler)
	}

	// 级别不同，用 multiHandler 分别控制
	fileHandler := slog.NewTextHandler(lj, &slog.HandlerOptions{
		Level: fileLevel,
	})
	consoleHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: consoleLevel,
	})
	return slog.New(&multiHandler{handlers: []slog.Handler{fileHandler, consoleHandler}})
}

// newLogger 创建一个 slog.Logger，根据 console 配置决定是否同时输出到控制台
func newLogger(lj *lumberjack.Logger, level slog.Level, console bool) *slog.Logger {
	return newLoggerWithConfig(lj, level, console, level)
}

// multiHandler 将日志同时发送到多个 Handler
type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Enabled(_ context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(nil, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, r.Level) {
			_ = handler.Handle(ctx, r)
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: newHandlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: newHandlers}
}

// parseConsoleTypes 解析终端输出类型配置
func parseConsoleTypes(types string) map[string]bool {
	result := map[string]bool{
		"app":     true,
		"request": true,
		"cmd":     true,
		"libvirt": true,
	}
	if types == "" || types == "all" {
		return result
	}
	// 重置为 false，只启用指定的
	for k := range result {
		result[k] = false
	}
	for _, t := range strings.Split(types, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			result[t] = true
		}
	}
	return result
}

// parseLevel 将字符串日志级别转换为 slog.Level
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Close 关闭所有 lumberjack Writer
func Close() {
	stopDailyRotation()
	for _, w := range allWriters {
		_ = w.Close()
	}
}
