package logger

import (
	"fmt"
	"sync"
	"time"
)

// rotationMu 保护 rotationDone / rotationExited,避免 Init/Close 与轮转 goroutine 之间的数据竞争。
var (
	rotationMu     sync.Mutex
	rotationDone   chan struct{} // 通知轮转 goroutine 退出
	rotationExited chan struct{} // 轮转 goroutine 退出后关闭,供 stop 等待
)

// startDailyRotation 启动每天凌晨 00:00 的定时轮转 goroutine(幂等:已在运行则不重复启动)。
func startDailyRotation() {
	rotationMu.Lock()
	defer rotationMu.Unlock()
	if rotationDone != nil {
		return
	}
	done := make(chan struct{})
	exited := make(chan struct{})
	rotationDone = done
	rotationExited = exited
	go func() {
		defer close(exited)
		defer func() {
			if r := recover(); r != nil {
				App.Error("panic recovered", "scope", "log-rotation", "panic", fmt.Sprintf("%v", r))
			}
		}()
		// timer 为 goroutine 局部,避免与其它访问竞争。
		var timer *time.Timer
		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			timer = time.NewTimer(next.Sub(now))

			select {
			case <-timer.C:
				rotateAll()
			case <-done:
				timer.Stop()
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

// stopDailyRotation 通知轮转 goroutine 退出并阻塞等待其退出(幂等)。
func stopDailyRotation() {
	rotationMu.Lock()
	done := rotationDone
	exited := rotationExited
	rotationDone = nil
	rotationExited = nil
	rotationMu.Unlock()
	if done != nil {
		close(done)
		<-exited
	}
}
