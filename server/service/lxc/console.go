package lxc

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// AttachSession 封装一次 lxc-attach 的 PTY 会话。
type AttachSession struct {
	Ptmx *os.File
	Cmd  *exec.Cmd
}

// StartAttach 用 PTY 启动 lxc-attach 进入容器交互 shell。
// 用 `su -` 起登录 shell（而非 /bin/sh）：加载 /etc/profile、得到正常提示符 [root@xxx ~]#，
// 否则进的是 bare sh，提示符是 sh-4.4#、环境也没初始化。su 默认切到 root。
func StartAttach(name string) (*AttachSession, error) {
	cmd := exec.Command("lxc-attach", "-n", name, "--", "su", "-")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("启动 lxc-attach PTY 失败: %w", err)
	}
	return &AttachSession{Ptmx: ptmx, Cmd: cmd}, nil
}

// Resize 调整 PTY 窗口大小。
func (s *AttachSession) Resize(cols, rows int) error {
	if s == nil || s.Ptmx == nil || cols <= 0 || rows <= 0 {
		return nil
	}
	return pty.Setsize(s.Ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

// Close 关闭 PTY 主端并等待进程退出（忽略“已退出”错误）。
func (s *AttachSession) Close() error {
	if s == nil || s.Ptmx == nil {
		return nil
	}
	_ = s.Ptmx.Close()
	if s.Cmd != nil && s.Cmd.Process != nil {
		_ = s.Cmd.Process.Signal(syscall.SIGKILL)
		_, _ = s.Cmd.Process.Wait()
	}
	return nil
}
