package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"qvmhub/model"
	"qvmhub/taskqueue"
	"qvmhub/utils"
)

const (
	windowsResetScriptGuestPath = "/ProgramData/kvm-console/reset-password.cmd"
	windowsResetDoneGuestPath   = "/ProgramData/kvm-console/reset-password.done"
	windowsResetErrorGuestPath  = "/ProgramData/kvm-console/reset-password.error"
)

// ResetLinuxPasswordParams 来宾虚拟机重置密码参数
type ResetLinuxPasswordParams struct {
	VMName   string `json:"vm_name"`
	Username string `json:"username"`
	Password string `json:"password"`
	Operator string `json:"operator,omitempty"`
}

// ParseResetLinuxPasswordParams 从 JSON 解析重置密码参数
func ParseResetLinuxPasswordParams(jsonStr string) (*ResetLinuxPasswordParams, error) {
	var params ResetLinuxPasswordParams
	if err := json.Unmarshal([]byte(jsonStr), &params); err != nil {
		return nil, err
	}
	return &params, nil
}

// ValidateResetGuestPasswordParams 按来宾系统类型校验重置密码参数
func ValidateResetGuestPasswordParams(username, password, guestType string) error {
	trimmedUsername := strings.TrimSpace(username)
	if trimmedUsername == "" {
		return fmt.Errorf("请输入要重置的用户名")
	}
	if strings.EqualFold(strings.TrimSpace(guestType), "windows") {
		if len([]rune(trimmedUsername)) > 64 {
			return fmt.Errorf("Windows 用户名长度不能超过 64 个字符")
		}
		if strings.ContainsAny(trimmedUsername, "\"/\\[]:;|=,+*?<>") {
			return fmt.Errorf("Windows 用户名包含不支持的字符")
		}
		return D.ValidateStrongPassword(password)
	}
	if !D.CloneUsernameRegexp.MatchString(trimmedUsername) {
		return fmt.Errorf("用户名只能以小写字母或下划线开头，且只能包含小写字母、数字、下划线和短横线")
	}
	return D.ValidateStrongPassword(password)
}

// ValidateResetLinuxPasswordParams 兼容旧调用，默认按 Linux 规则校验
func ValidateResetLinuxPasswordParams(username, password string) error {
	return ValidateResetGuestPasswordParams(username, password, "linux")
}

// ResetLinuxPassword 离线重置虚拟机密码
func ResetLinuxPassword(ctx context.Context, params *ResetLinuxPasswordParams, progressFn func(int, string)) error {
	vmName := strings.TrimSpace(params.VMName)
	if err := D.HookEnsureVMNotMigrating(vmName, "重置密码"); err != nil {
		return err
	}
	progressFn(5, "正在检查虚拟机状态...")

	vm, err := GetVM(vmName)
	if err != nil {
		return err
	}
	if strings.TrimSpace(vm.Status) != "shut off" {
		return fmt.Errorf("请先将虚拟机关机后再重置密码")
	}
	guestType := strings.ToLower(strings.TrimSpace(vm.OSType))
	if err := ValidateResetGuestPasswordParams(params.Username, params.Password, guestType); err != nil {
		return err
	}
	if guestType != "linux" && guestType != "fnos" && guestType != "windows" {
		return fmt.Errorf("当前仅支持 Linux、Windows 或 fnOS 虚拟机重置密码")
	}
	if vm.DiskPath == "" {
		return fmt.Errorf("未找到虚拟机磁盘，无法重置密码")
	}

	select {
	case <-ctx.Done():
		return taskqueue.ErrTaskCanceled
	default:
	}

	if guestType == "windows" {
		progressFn(25, "正在写入 Windows 重置脚本...")
		if err := stageWindowsPasswordReset(vm.DiskPath, params.Username, params.Password); err != nil {
			return err
		}
	} else {
		progressFn(25, "正在离线修改虚拟机密码...")
		passwordArg := fmt.Sprintf("%s:password:%s", strings.TrimSpace(params.Username), params.Password)
		result := utils.ExecCommandLongRunning("virt-customize", "-a", vm.DiskPath, "--password", passwordArg, "--selinux-relabel")
		if result.Error != nil {
			return fmt.Errorf("离线重置密码失败: %s", strings.TrimSpace(result.Stderr))
		}
	}

	select {
	case <-ctx.Done():
		return taskqueue.ErrTaskCanceled
	default:
	}

	progressFn(80, "正在保存最新凭据...")
	if err := SaveVMCredential(vmName, params.Username, params.Password, "password_reset", params.Operator, true); err != nil {
		return err
	}

	if guestType == "windows" {
		progressFn(100, "Windows 密码重置任务已准备完成，请手动开机一次等待系统自动处理并关机")
		return nil
	}

	progressFn(100, "密码重置完成")
	return nil
}

// SubmitResetLinuxPasswordTask 提交 Linux/FnOS 重置密码任务
func SubmitResetLinuxPasswordTask(params *ResetLinuxPasswordParams, operator string) (*model.Task, error) {
	params.Operator = operator
	return taskqueue.SubmitWithStruct(model.TaskTypeResetVMPassword, params, operator)
}

func stageWindowsPasswordReset(diskPath, username, password string) error {
	scriptContent := buildWindowsPasswordResetScript(username, password)
	writeResult := utils.ExecCommandLongRunning(
		"virt-customize",
		"-a", diskPath,
		"--mkdir", "/ProgramData/kvm-console",
		"--write", windowsResetScriptGuestPath+":"+scriptContent,
		"--selinux-relabel",
	)
	if writeResult.Error != nil {
		return fmt.Errorf("写入 Windows 重置脚本失败: %s", strings.TrimSpace(writeResult.Stderr))
	}

	regFile, err := os.CreateTemp("", "kvm-console-windows-reset-*.reg")
	if err != nil {
		return fmt.Errorf("创建 Windows 注册表文件失败: %w", err)
	}
	regPath := regFile.Name()
	defer os.Remove(regPath)

	if _, err := regFile.WriteString(buildWindowsSetupRegFile()); err != nil {
		_ = regFile.Close()
		return fmt.Errorf("写入 Windows 注册表文件失败: %w", err)
	}
	if err := regFile.Close(); err != nil {
		return fmt.Errorf("关闭 Windows 注册表文件失败: %w", err)
	}

	mergeResult := utils.ExecCommandLongRunning("virt-win-reg", "--merge", diskPath, regPath)
	if mergeResult.Error != nil {
		return fmt.Errorf("写入 Windows 注册表失败: %s", strings.TrimSpace(mergeResult.Stderr))
	}

	return nil
}

func buildWindowsPasswordResetScript(username, password string) string {
	safeUsername := escapeWindowsBatchQuotedValue(strings.TrimSpace(username))
	safePassword := escapeWindowsBatchQuotedValue(password)
	donePath := toWindowsBatchPath(windowsResetDoneGuestPath)
	errorPath := toWindowsBatchPath(windowsResetErrorGuestPath)

	lines := []string{
		"@echo off",
		"setlocal DisableDelayedExpansion",
		fmt.Sprintf("del /f /q \"%s\" >nul 2>&1", donePath),
		fmt.Sprintf("del /f /q \"%s\" >nul 2>&1", errorPath),
		`reg add "HKLM\SYSTEM\Setup" /v CmdLine /t REG_SZ /d "" /f >nul 2>&1`,
		`reg add "HKLM\SYSTEM\Setup" /v SetupType /t REG_DWORD /d 0 /f >nul 2>&1`,
		`reg add "HKLM\SYSTEM\Setup" /v SystemSetupInProgress /t REG_DWORD /d 0 /f >nul 2>&1`,
		`reg add "HKLM\SYSTEM\Setup" /v OOBEInProgress /t REG_DWORD /d 0 /f >nul 2>&1`,
		fmt.Sprintf("net user \"%s\" \"%s\"", safeUsername, safePassword),
		"if errorlevel 1 (",
		fmt.Sprintf("  > \"%s\" echo net_user_failed", errorPath),
		"  shutdown /s /t 5 /f",
		"  exit /b 1",
		")",
		fmt.Sprintf("> \"%s\" echo success", donePath),
		"shutdown /s /t 5 /f",
	}
	return strings.Join(lines, "\r\n")
}

func buildWindowsSetupRegFile() string {
	return strings.Join([]string{
		"Windows Registry Editor Version 5.00",
		"",
		`[HKEY_LOCAL_MACHINE\SYSTEM\Setup]`,
		`"CmdLine"="cmd.exe /c C:\\ProgramData\\kvm-console\\reset-password.cmd"`,
		`"SetupType"=dword:00000002`,
		`"SystemSetupInProgress"=dword:00000001`,
		`"OOBEInProgress"=dword:00000001`,
		"",
	}, "\r\n")
}

func escapeWindowsBatchQuotedValue(value string) string {
	replacer := strings.NewReplacer(
		"^", "^^",
		"%", "%%",
	)
	return replacer.Replace(value)
}

func toWindowsBatchPath(guestPath string) string {
	trimmed := strings.TrimPrefix(filepath.Clean(guestPath), string(filepath.Separator))
	return "C:\\" + strings.ReplaceAll(trimmed, "/", "\\")
}
