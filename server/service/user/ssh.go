package user

import (
	"fmt"
	"os"
	"strings"

	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/utils"
)

const (
	// sshShellBash 允许 SSH 时使用的 shell
	sshShellBash = "/bin/bash"
	// sshShellNologin 禁止 SSH 时使用的 shell
	sshShellNologin = "/usr/sbin/nologin"
)

// SetUserSSH 设置用户 SSH 访问开关
func SetUserSSH(username string, enabled bool) error {
	// 更新数据库记录
	result := model.DB.Model(&model.User{}).Where("username = ?", username).Update("ssh_enabled", enabled)
	if result.Error != nil {
		return fmt.Errorf("更新 SSH 状态失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("用户 %s 不存在", username)
	}

	// 切换系统用户的 shell
	setUserShell(username, enabled)

	// 如果是关闭 SSH，杀死该用户的所有现有 SSH 会话
	if !enabled {
		killUserSSHSessions(username)
	}

	// 重新生成 sshd 限制配置（双重保险）
	return regenerateSSHDenyConfig()
}

// GetUserSSHStatus 获取用户 SSH 状态
func GetUserSSHStatus(username string) (bool, error) {
	var user model.User
	if err := model.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return false, fmt.Errorf("查询用户失败: %w", err)
	}
	return user.SSHEnabled, nil
}

// setUserShell 设置系统用户的登录 shell
// enabled=true 时设为 /bin/bash，enabled=false 时设为 /usr/sbin/nologin
func setUserShell(username string, enabled bool) {
	shell := sshShellNologin
	if enabled {
		shell = sshShellBash
	}
	utils.ExecCommand("usermod", "-s", shell, username)
}

// killUserSSHSessions 杀死指定用户的所有 SSH 会话
func killUserSSHSessions(username string) {
	// 杀死该用户拥有的所有 sshd 进程（即该用户的 SSH 会话）
	utils.ExecShell(fmt.Sprintf("pkill -KILL -u %s sshd 2>/dev/null || true", utils.ShellSingleQuote(username)))
	// 同时杀死该用户的所有 shell 进程（清理残留会话）
	utils.ExecShell(fmt.Sprintf("pkill -KILL -u %s -t pts 2>/dev/null || true", utils.ShellSingleQuote(username)))
}

// regenerateSSHDenyConfig 重新生成 sshd DenyUsers 配置
// 使用 /etc/ssh/sshd_config.d/ 目录下的 drop-in 文件来管理
func regenerateSSHDenyConfig() error {
	// 获取所有 SSH 被禁止的普通用户（ssh_enabled = false 且 role = user）
	var users []model.User
	if err := model.DB.Where("role = ? AND ssh_enabled = ?", "user", false).Find(&users).Error; err != nil {
		return fmt.Errorf("查询用户列表失败: %w", err)
	}

	configPath := "/etc/ssh/sshd_config.d/kvm-console-deny.conf"

	if len(users) == 0 {
		// 没有需要禁止的用户，删除配置文件
		_ = os.Remove(configPath)
	} else {
		// 生成 DenyUsers 列表
		var denyList []string
		for _, u := range users {
			denyList = append(denyList, u.Username)
		}

		config := fmt.Sprintf("# 由 QVMConsole 自动生成 - 请勿手动编辑\n# 禁止面板用户通过 SSH 登录\nDenyUsers %s\n",
			strings.Join(denyList, " "))

		// 确保 sshd_config.d 目录存在
		utils.ExecCommand("mkdir", "-p", "/etc/ssh/sshd_config.d")

		// 确保主配置文件包含 Include 指令
		ensureSSHDInclude()

		// 写入配置文件
		if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
			return fmt.Errorf("写入 SSH 拒绝配置失败: %v", err)
		}
	}

	// 同步所有面板用户的 shell（确保与数据库状态一致）
	syncAllUserShells()

	// 重新加载 sshd 配置（不中断现有连接）
	reloadResult := utils.ExecCommand("systemctl", "reload", "sshd")
	if reloadResult.Error != nil {
		// 某些系统服务名为 ssh 而非 sshd
		utils.ExecCommand("systemctl", "reload", "ssh")
	}

	return nil
}

// syncAllUserShells 同步所有面板普通用户的 shell
func syncAllUserShells() {
	var users []model.User
	if err := model.DB.Where("role = ?", "user").Find(&users).Error; err != nil {
		logger.App.Warn("查询用户列表失败", "error", err)
		return
	}

	for _, u := range users {
		// 检查系统用户是否存在
		checkResult := utils.ExecShell(fmt.Sprintf("id %s 2>/dev/null", utils.ShellSingleQuote(u.Username)))
		if checkResult.Error != nil {
			continue // 系统用户不存在，跳过
		}
		setUserShell(u.Username, u.SSHEnabled)
	}
}

// ensureSSHDInclude 确保 sshd_config 中包含 Include 指令
func ensureSSHDInclude() {
	sshdConfigPath := "/etc/ssh/sshd_config"
	// 检查是否已包含 Include 指令
	content, err := os.ReadFile(sshdConfigPath)
	if err == nil && strings.Contains(string(content), "Include /etc/ssh/sshd_config.d/") {
		return
	}
	// 没有找到，在文件开头添加 Include 指令
	newContent := "Include /etc/ssh/sshd_config.d/*.conf\n" + string(content)
	if err := utils.AtomicWriteFile(sshdConfigPath, []byte(newContent), 0644); err != nil {
		logger.App.Warn("写入 sshd_config Include 指令失败", "error", err)
	}
}

// SyncSSHDenyConfig 启动时同步 SSH 拒绝配置（供初始化时调用）
func SyncSSHDenyConfig() {
	if err := regenerateSSHDenyConfig(); err != nil {
		logger.App.Warn("同步 SSH 拒绝配置失败", "error", err)
	}
}
