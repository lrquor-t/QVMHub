package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"qvmhub/model"
	"qvmhub/utils"
)

// sshWarningPatterns SSH 客户端输出的非错误性警告信息，这些不应被视为错误原因
var sshWarningPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^Warning: Permanently added .+ to the list of known hosts\.$`),
	regexp.MustCompile(`^Warning: the .+ host key .+$`),
}

// stripSSHWarnings 从 stderr 输出中移除 SSH 的警告/信息性消息，保留真正的错误信息
func stripSSHWarnings(stderr string) string {
	lines := strings.Split(stderr, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		isWarning := false
		for _, pattern := range sshWarningPatterns {
			if pattern.MatchString(trimmed) {
				isWarning = true
				break
			}
		}
		if !isWarning {
			filtered = append(filtered, line)
		}
	}
	return strings.TrimSpace(strings.Join(filtered, "\n"))
}

func RemoteSSHCommand(ctx context.Context, node model.HostNode, command string, timeout time.Duration) (string, error) {
	return remoteSSHExec(ctx, node, command, timeout, false)
}

// remoteSSHExec 执行远程 SSH 命令
// tolerateRemoteExit 为 true 时，远程命令的非零退出码不视为 SSH 层面的错误
// 因为 SSH 本身连接正常，只是远程命令执行失败（如 command -v 找不到命令）
func remoteSSHExec(ctx context.Context, node model.HostNode, command string, timeout time.Duration, tolerateRemoteExit bool) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	passwordFile, cleanup, err := writeSSHPasswordFile(node)
	if err != nil {
		return "", err
	}
	defer cleanup()
	target := fmt.Sprintf("%s@%s", strings.TrimSpace(node.SSHUser), strings.TrimSpace(node.SSHHost))
	sshPort := node.SSHPort
	if sshPort <= 0 {
		sshPort = 22
	}
	cmd := fmt.Sprintf("sshpass -f %s ssh -p %d -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o GlobalKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=10 %s %s",
		utils.ShellSingleQuote(passwordFile), sshPort, utils.ShellSingleQuote(target), utils.ShellSingleQuote(command))
	result := utils.ExecShellContextWithTimeout(ctx, cmd, timeout)
	if result.Error != nil {
		if result.ExitCode == 255 {
			cleanStderr := stripSSHWarnings(result.Stderr)
			errMsg := FirstNonEmpty(cleanStderr, result.Error.Error())
			return "", fmt.Errorf("SSH 连接失败: %s", errMsg)
		}
		if tolerateRemoteExit {
			return result.Stdout, nil
		}
		cleanStderr := stripSSHWarnings(result.Stderr)
		errMsg := FirstNonEmpty(cleanStderr, result.Error.Error())
		return "", fmt.Errorf("%s", errMsg)
	}
	return result.Stdout, nil
}

func RemoteRsyncFile(ctx context.Context, node model.HostNode, sourcePath, targetPath string, timeout time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	passwordFile, cleanup, err := writeSSHPasswordFile(node)
	if err != nil {
		return err
	}
	defer cleanup()
	target := fmt.Sprintf("%s@%s:%s", strings.TrimSpace(node.SSHUser), strings.TrimSpace(node.SSHHost), targetPath)
	sshPort := node.SSHPort
	if sshPort <= 0 {
		sshPort = 22
	}
	cmd := fmt.Sprintf("sshpass -f %s rsync -aS --numeric-ids -e %s %s %s",
		utils.ShellSingleQuote(passwordFile),
		utils.ShellSingleQuote(fmt.Sprintf("ssh -p %d -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o GlobalKnownHostsFile=/dev/null -o LogLevel=ERROR", sshPort)),
		utils.ShellSingleQuote(sourcePath),
		utils.ShellSingleQuote(target))
	result := utils.ExecShellContextWithTimeout(ctx, cmd, timeout)
	if result.Error != nil {
		cleanStderr := stripSSHWarnings(result.Stderr)
		errMsg := FirstNonEmpty(cleanStderr, result.Error.Error())
		return fmt.Errorf("%s", errMsg)
	}
	return nil
}

func WriteRemoteFile(ctx context.Context, node model.HostNode, content, targetPath string, timeout time.Duration) error {
	tmp, err := os.CreateTemp("", "kvm-migration-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	_ = tmp.Close()
	defer os.Remove(tmpPath)
	return RemoteRsyncFile(ctx, node, tmpPath, targetPath, timeout)
}

func EnsureDefaultSSHKeyTrusted(ctx context.Context, node model.HostNode) error {
	if ctx == nil {
		ctx = context.Background()
	}
	keyPath := "/root/.ssh/id_ed25519"
	if result := utils.ExecShell("test -f " + utils.ShellSingleQuote(keyPath) + " || ssh-keygen -t ed25519 -N '' -f " + utils.ShellSingleQuote(keyPath) + " >/dev/null"); result.Error != nil {
		return fmt.Errorf("生成本机迁移 SSH Key 失败: %s", result.Stderr)
	}
	pub := utils.ExecShell("cat " + utils.ShellSingleQuote(keyPath+".pub"))
	if pub.Error != nil || strings.TrimSpace(pub.Stdout) == "" {
		return fmt.Errorf("读取本机迁移 SSH 公钥失败: %s", pub.Stderr)
	}
	install := fmt.Sprintf("mkdir -p /root/.ssh && chmod 700 /root/.ssh && grep -qxF %s /root/.ssh/authorized_keys 2>/dev/null || echo %s >> /root/.ssh/authorized_keys; chmod 600 /root/.ssh/authorized_keys",
		utils.ShellSingleQuote(strings.TrimSpace(pub.Stdout)), utils.ShellSingleQuote(strings.TrimSpace(pub.Stdout)))
	if _, err := RemoteSSHCommand(ctx, node, install, 30*time.Second); err != nil {
		return err
	}
	return nil
}

func writeSSHPasswordFile(node model.HostNode) (string, func(), error) {
	password, err := decryptHostNodeSSHPassword(node)
	if err != nil {
		return "", func() {}, fmt.Errorf("解密 SSH 密码失败: %w", err)
	}
	tmpDir := os.TempDir()
	path := filepath.Join(tmpDir, fmt.Sprintf("kvm-node-%d-%d.pass", node.ID, time.Now().UnixNano()))
	if err := os.WriteFile(path, []byte(password), 0600); err != nil {
		return "", func() {}, err
	}
	return path, func() { _ = os.Remove(path) }, nil
}

func CallNodeAPI(node model.HostNode, method, path string, body interface{}, out interface{}) ([]byte, error) {
	apiKey, err := decryptHostNodeAPIKey(node)
	if err != nil {
		return nil, fmt.Errorf("解密目标面板 API Key 失败: %w", err)
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}
	url := strings.TrimRight(node.APIBaseURL, "/") + path
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key-ID", node.APIKeyID)
	req.Header.Set("X-API-Key", apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return raw, fmt.Errorf("目标面板接口 %s 返回 %d: %s", path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out != nil {
		var wrapper struct {
			Code    int             `json:"code"`
			Message string          `json:"message"`
			Data    json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(raw, &wrapper); err == nil && len(wrapper.Data) > 0 {
			if wrapper.Code >= 400 {
				return raw, fmt.Errorf("%s", wrapper.Message)
			}
			if err := json.Unmarshal(wrapper.Data, out); err != nil {
				return raw, err
			}
			return raw, nil
		}
		if err := json.Unmarshal(raw, out); err != nil {
			return raw, err
		}
	}
	return raw, nil
}
