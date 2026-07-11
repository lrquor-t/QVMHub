//go:build windows

package utils

import "os/exec"

func prepareProcessGroup(cmd *exec.Cmd) {
}

func killProcessTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
