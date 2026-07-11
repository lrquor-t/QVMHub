package logger

import (
	"fmt"
	"time"
)

// rotationTimer 定时轮转使用的 timer，用于停止时取消
var rotationTimer *time.Timer

// rotationDone 用于通知轮转 goroutine 退出
var rotationDone chan struct{}

// startDailyRotation 启动每天凌晨 00:00 的定时轮转 goroutine
func startDailyRotation() {
	rotationDone = make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				App.Error("panic recovered", "scope", "log-rotation", "panic", fmt.Sprintf("%v", r))
			}
		}()
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			duration := next.Sub(now)

			rotationTimer = time.NewTimer(duration)

			select {
			case <-rotationTimer.C:
				// 凌晨触发轮转
				rotateAll()
			case <-rotationDone:
				// 收到停止信号
				rotationTimer.Stop()
				return
			}
		}
	}()
}

// rotateAll 对所有 lumberjack Writer 执行 Rotate
func rotateAll() {
	for _, w := range allWriters {
		_ = w.Rotate()
	}
}

// stopDailyRotation 停止定时轮转 goroutine
func stopDailyRotation() {
	if rotationDone != nil {
		close(rotationDone)
		rotationDone = nil
	}
}
