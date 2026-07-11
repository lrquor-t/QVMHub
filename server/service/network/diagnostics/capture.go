package diagnostics

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"qvmhub/utils"
)

// InitNetworkCaptureSession 初始化抓包会话
func InitNetworkCaptureSession(taskID uint, vmName string, req NetworkCaptureRequest, createdBy string) {
	now := time.Now()
	pruneOldCaptureSessions(now.Add(-24 * time.Hour))
	deletePreviousCaptureFile(taskID)
	session := &NetworkCaptureSession{
		TaskID:          taskID,
		VMName:          strings.TrimSpace(vmName),
		InterfaceName:   strings.TrimSpace(req.InterfaceName),
		Filter:          req.Filter,
		Status:          "pending",
		Message:         "抓包任务已提交",
		DurationSeconds: req.DurationSeconds,
		MaxMB:           req.MaxMB,
		MaxPackets:      req.MaxPackets,
		StartedAt:       now,
		UpdatedAt:       now,
	}
	captureSessionsMu.Lock()
	captureSessions[taskID] = session
	captureSessionsMu.Unlock()
}

// GetNetworkCaptureSession 获取抓包会话信息
func GetNetworkCaptureSession(taskID uint) (*NetworkCaptureSession, bool) {
	captureSessionsMu.RLock()
	defer captureSessionsMu.RUnlock()
	session, ok := captureSessions[taskID]
	if !ok {
		return nil, false
	}
	cp := *session
	cp.SummaryLines = append([]string{}, session.SummaryLines...)
	if cp.FileName != "" {
		cp.FileSize = captureFileSize(filepath.Join(networkCaptureDir(), cp.FileName))
	}
	return &cp, true
}

// DeleteNetworkCaptureFile 删除抓包文件
func DeleteNetworkCaptureFile(taskID uint) error {
	session, ok := GetNetworkCaptureSession(taskID)
	if !ok {
		return fmt.Errorf("抓包任务不存在或已过期")
	}
	if session.Status == "running" {
		return fmt.Errorf("抓包仍在运行，请先取消或等待完成后再删除")
	}
	if strings.TrimSpace(session.FileName) == "" {
		return nil
	}
	filePath, _, err := CaptureFilePathAbs(taskID)
	if err != nil {
		updateNetworkCaptureSession(taskID, func(s *NetworkCaptureSession) {
			s.FileName = ""
			s.DownloadPath = ""
			s.FileSize = 0
			s.Message = "pcap 文件已不存在"
		})
		return nil
	}
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除 pcap 文件失败: %w", err)
	}
	updateNetworkCaptureSession(taskID, func(s *NetworkCaptureSession) {
		s.FileName = ""
		s.DownloadPath = ""
		s.FileSize = 0
		s.Message = "pcap 文件已删除"
	})
	return nil
}

// ExecuteNetworkCapture 执行抓包任务
func ExecuteNetworkCapture(ctx context.Context, taskID uint, params NetworkCaptureParams, progress func(int, string)) (string, error) {
	if _, err := exec.LookPath("tcpdump"); err != nil {
		captureErr := fmt.Errorf("未检测到 tcpdump，请先安装 tcpdump 后再执行抓包")
		failNetworkCaptureSession(taskID, captureErr)
		return "", captureErr
	}
	req, iface, bpf, err := NormalizeNetworkCaptureRequest(params.VMName, params.NetworkCaptureRequest)
	if err != nil {
		failNetworkCaptureSession(taskID, err)
		return "", err
	}
	updateNetworkCaptureSession(taskID, func(session *NetworkCaptureSession) {
		session.VMName = params.VMName
		session.InterfaceName = iface
		session.Filter = req.Filter
		session.BPF = bpf
		session.DurationSeconds = req.DurationSeconds
		session.MaxMB = req.MaxMB
		session.MaxPackets = req.MaxPackets
		session.Status = "running"
		session.Message = "正在抓包..."
	})
	if progress != nil {
		progress(10, "正在准备抓包环境...")
	}
	if err := os.MkdirAll(networkCaptureDir(), 0o750); err != nil {
		captureErr := fmt.Errorf("创建抓包目录失败: %w", err)
		failNetworkCaptureSession(taskID, captureErr)
		return "", captureErr
	}
	fileName := fmt.Sprintf("capture-%d-%s-%s.pcap", taskID, sanitizeFilePart(params.VMName), time.Now().Format("20060102-150405"))
	filePath := filepath.Join(networkCaptureDir(), fileName)
	updateNetworkCaptureSession(taskID, func(session *NetworkCaptureSession) {
		session.FileName = fileName
		session.DownloadPath = fmt.Sprintf("/api/network/captures/%d/download", taskID)
	})

	captureCtx, cancel := context.WithTimeout(ctx, time.Duration(req.DurationSeconds)*time.Second)
	defer cancel()
	errCh := make(chan error, 2)
	go func() {
		defer utils.RecoverAndLog("capture-tcpdump-file")
		errCh <- runTcpdumpToFile(captureCtx, iface, filePath, req.MaxPackets, bpf)
	}()
	go func() {
		defer utils.RecoverAndLog("capture-tcpdump-summary")
		errCh <- runTcpdumpSummary(captureCtx, taskID, iface, req.MaxPackets, bpf)
	}()
	go monitorCaptureFileSize(captureCtx, cancel, taskID, filePath, int64(req.MaxMB)*1024*1024)

	if progress != nil {
		progress(30, "抓包进行中...")
	}
	var firstErr error
	for i := 0; i < 2; i++ {
		err := <-errCh
		if err != nil && firstErr == nil && captureCtx.Err() == nil {
			firstErr = err
			cancel()
		}
	}
	if progress != nil {
		progress(90, "正在整理抓包结果...")
	}
	now := time.Now()
	fileSize := captureFileSize(filePath)
	status := "success"
	message := "抓包完成"
	if ctx.Err() == context.Canceled {
		status = "canceled"
		message = "抓包任务已取消"
		firstErr = ctx.Err()
	} else if firstErr != nil {
		status = "failed"
		message = firstErr.Error()
	}
	updateNetworkCaptureSession(taskID, func(session *NetworkCaptureSession) {
		session.Status = status
		session.Message = message
		session.FileSize = fileSize
		session.FinishedAt = &now
	})
	result := map[string]interface{}{
		"task_id":       taskID,
		"vm_name":       params.VMName,
		"interface":     iface,
		"bpf":           bpf,
		"file_name":     fileName,
		"download_path": fmt.Sprintf("/api/network/captures/%d/download", taskID),
		"file_size":     fileSize,
	}
	data, _ := json.Marshal(result)
	if firstErr != nil {
		return string(data), firstErr
	}
	if progress != nil {
		progress(100, "抓包完成")
	}
	return string(data), nil
}

// ── 内部函数 ──

func runTcpdumpToFile(ctx context.Context, iface, filePath string, packets int, bpf string) error {
	args := []string{"-i", iface, "-nn", "-s", "0", "-U", "-w", filePath, "-c", strconv.Itoa(packets)}
	if bpf != "" {
		args = append(args, bpf)
	}
	cmd := exec.CommandContext(ctx, "tcpdump", args...)
	output, err := cmd.CombinedOutput()
	if err != nil && ctx.Err() == nil {
		return fmt.Errorf("tcpdump 写入 pcap 失败: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func runTcpdumpSummary(ctx context.Context, taskID uint, iface string, packets int, bpf string) error {
	args := []string{"-i", iface, "-nn", "-l", "-tttt", "-s", "160", "-c", strconv.Itoa(packets)}
	if bpf != "" {
		args = append(args, bpf)
	}
	cmd := exec.CommandContext(ctx, "tcpdump", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 tcpdump 摘要失败: %w", err)
	}
	var wg sync.WaitGroup
	readPipe := func(scanner *bufio.Scanner) {
		defer wg.Done()
		scanner.Buffer(make([]byte, 1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				appendCaptureSummaryLine(taskID, line)
			}
		}
	}
	wg.Add(2)
	go readPipe(bufio.NewScanner(stdout))
	go readPipe(bufio.NewScanner(stderr))
	err = cmd.Wait()
	wg.Wait()
	if err != nil && ctx.Err() == nil {
		return fmt.Errorf("tcpdump 摘要输出失败: %w", err)
	}
	return nil
}

func monitorCaptureFileSize(ctx context.Context, cancel context.CancelFunc, taskID uint, filePath string, maxBytes int64) {
	if maxBytes <= 0 {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			size := captureFileSize(filePath)
			updateNetworkCaptureSession(taskID, func(session *NetworkCaptureSession) {
				session.FileSize = size
			})
			if size >= maxBytes {
				appendCaptureSummaryLine(taskID, "抓包文件达到大小上限，已停止抓包")
				cancel()
				return
			}
		}
	}
}
