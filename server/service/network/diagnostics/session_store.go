package diagnostics

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const maxCaptureSummaryLines = 300

var (
	captureSessions   = make(map[uint]*NetworkCaptureSession)
	captureSessionsMu sync.RWMutex
)

// UpdateNetworkCaptureSession 更新抓包会话（导出版供 capture.go 使用）
func UpdateNetworkCaptureSession(taskID uint, fn func(*NetworkCaptureSession)) {
	now := time.Now()
	captureSessionsMu.Lock()
	defer captureSessionsMu.Unlock()
	session, ok := captureSessions[taskID]
	if !ok {
		session = &NetworkCaptureSession{TaskID: taskID, StartedAt: now}
		captureSessions[taskID] = session
	}
	fn(session)
	session.UpdatedAt = now
}

// updateNetworkCaptureSession 内部版
func updateNetworkCaptureSession(taskID uint, fn func(*NetworkCaptureSession)) {
	UpdateNetworkCaptureSession(taskID, fn)
}

func deletePreviousCaptureFile(currentTaskID uint) {
	var previousTaskID uint
	var previousFileName string
	var previousUpdatedAt time.Time
	captureSessionsMu.RLock()
	for id, session := range captureSessions {
		if id == currentTaskID || session.Status == "running" || strings.TrimSpace(session.FileName) == "" {
			continue
		}
		if previousTaskID == 0 || session.UpdatedAt.After(previousUpdatedAt) {
			previousTaskID = id
			previousFileName = session.FileName
			previousUpdatedAt = session.UpdatedAt
		}
	}
	captureSessionsMu.RUnlock()
	if previousTaskID == 0 {
		return
	}
	fileName := filepath.Base(previousFileName)
	if fileName == "" || fileName == "." {
		return
	}
	_ = os.Remove(filepath.Join(networkCaptureDir(), fileName))
	updateNetworkCaptureSession(previousTaskID, func(session *NetworkCaptureSession) {
		session.FileName = ""
		session.DownloadPath = ""
		session.FileSize = 0
		session.Message = "pcap 文件已在新抓包开始前自动删除"
	})
}

func failNetworkCaptureSession(taskID uint, err error) {
	now := time.Now()
	updateNetworkCaptureSession(taskID, func(session *NetworkCaptureSession) {
		session.Status = "failed"
		if err != nil {
			session.Message = err.Error()
		} else {
			session.Message = "抓包失败"
		}
		session.FinishedAt = &now
	})
}

func appendCaptureSummaryLine(taskID uint, line string) {
	updateNetworkCaptureSession(taskID, func(session *NetworkCaptureSession) {
		session.SummaryLines = append(session.SummaryLines, line)
		if len(session.SummaryLines) > maxCaptureSummaryLines {
			session.SummaryLines = session.SummaryLines[len(session.SummaryLines)-maxCaptureSummaryLines:]
		}
	})
}

// PruneOldCaptureSessions 清理过期的抓包会话（导出版供外部调用）
func PruneOldCaptureSessions(cutoff time.Time) {
	captureSessionsMu.Lock()
	defer captureSessionsMu.Unlock()
	for id, session := range captureSessions {
		if !session.UpdatedAt.IsZero() && session.UpdatedAt.Before(cutoff) && session.Status != "running" {
			delete(captureSessions, id)
		}
	}
}

// pruneOldCaptureSessions 内部版
func pruneOldCaptureSessions(cutoff time.Time) {
	PruneOldCaptureSessions(cutoff)
}
