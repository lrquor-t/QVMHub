package vm

import (
	"context"
	"regexp"
	"strings"
	"time"

	"qvmhub/utils"
)

const (
	VMFirstBootRebootNormal = "normal"
	VMFirstBootRebootCold   = "cold"

	windowsFirstBootColdRebootTimeout = 15 * time.Minute
	windowsFirstBootColdRebootPoll    = 5 * time.Second
)

var onRebootRegexp = regexp.MustCompile(`<on_reboot>[^<]*</on_reboot>`)

func NormalizeVMFirstBootRebootMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case VMFirstBootRebootCold, "host_cold", "power_cycle":
		return VMFirstBootRebootCold
	default:
		return VMFirstBootRebootNormal
	}
}

func ShouldUseWindowsFirstBootColdReboot(mode, osType string) bool {
	return strings.EqualFold(strings.TrimSpace(osType), "windows") &&
		NormalizeVMFirstBootRebootMode(mode) == VMFirstBootRebootCold
}

func ApplyFirstBootRebootModeToDomainXML(xmlStr, mode string) string {
	action := "restart"
	if NormalizeVMFirstBootRebootMode(mode) == VMFirstBootRebootCold {
		action = "destroy"
	}
	replacement := "<on_reboot>" + action + "</on_reboot>"
	if onRebootRegexp.MatchString(xmlStr) {
		return onRebootRegexp.ReplaceAllString(xmlStr, replacement)
	}
	if strings.Contains(xmlStr, "<on_poweroff>") {
		return strings.Replace(xmlStr, "</on_poweroff>", "</on_poweroff>"+replacement, 1)
	}
	return strings.Replace(xmlStr, "</features>", "</features>\n  "+replacement, 1)
}

func CompleteWindowsFirstBootColdReboot(ctx context.Context, name string, progressFn func(int, string)) error {
	if progressFn == nil {
		progressFn = func(int, string) {}
	}
	progressFn(48, "等待 Windows 首次重启转换为宿主冷启动...")
	coldRebooted, err := WaitForVMShutOff(ctx, name, windowsFirstBootColdRebootTimeout)
	if err != nil {
		return err
	}

	FixOnReboot(name)
	if !coldRebooted {
		progressFn(49, "未检测到首次重启关机，已恢复后续正常重启策略")
		return nil
	}

	progressFn(49, "首次重启已关机，正在从宿主机重新开机...")
	return StartVM(name)
}

func WaitForVMShutOff(ctx context.Context, name string, timeout time.Duration) (bool, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	ticker := time.NewTicker(windowsFirstBootColdRebootPoll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-deadline.C:
			return false, nil
		case <-ticker.C:
			stateResult := utils.ExecCommand("virsh", "domstate", name)
			if stateResult.Error != nil {
				continue
			}
			state := strings.ToLower(strings.TrimSpace(stateResult.Stdout))
			if state == "shut off" || state == "shutoff" {
				return true, nil
			}
		}
	}
}
