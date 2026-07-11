package lxc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/utils"
)

// moveStep 表示单个容器目录的迁移源/目标。
type moveStep struct {
	From string
	To   string
}

// CascadeImportDir 计算迁移后模板导入临时目录的级联值。
// 规则：若当前 import dir 等于 <oldLxcPath>/_imports（默认跟随），则切到 <newLxcPath>/_imports；否则保持现值。
func CascadeImportDir(oldLxcPath, newLxcPath, curImportDir string) string {
	if curImportDir == filepath.Join(oldLxcPath, "_imports") {
		return filepath.Join(newLxcPath, "_imports")
	}
	return curImportDir
}

// rewriteLxcConf 把 lxc.conf 文本中 lxc.lxcpath 行替换为新值（无则追加），保留其它行。
// 仅匹配紧邻空白或 '=' 的 lxc.lxcpath 键，避免误伤 lxc.lxcpath.xxx 之类。
func rewriteLxcConf(content, newLxcPath string) string {
	const key = "lxc.lxcpath"
	lines := strings.Split(content, "\n")
	found := false
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, key) {
			continue
		}
		rest := strings.TrimPrefix(trim, key)
		if rest == "" || rest[0] == ' ' || rest[0] == '\t' || rest[0] == '=' {
			lines[i] = "lxc.lxcpath = " + newLxcPath
			found = true
		}
	}
	if !found {
		lines = append(lines, "lxc.lxcpath = "+newLxcPath)
	}
	return strings.Join(lines, "\n")
}

// rewriteContainerConfig 把容器 config 文本中 <oldLxcPath>/ 前缀替换为 <newLxcPath>/。
// 用「带尾斜线匹配」避免 /var/lib/lxc 与 /var/lib/lxc2 前缀碰撞叠加。
func rewriteContainerConfig(content, oldLxcPath, newLxcPath string) string {
	if oldLxcPath == "" {
		return content
	}
	oldDir := strings.TrimRight(oldLxcPath, "/") + "/"
	newDir := strings.TrimRight(newLxcPath, "/") + "/"
	return strings.ReplaceAll(content, oldDir, newDir)
}

// planRelocateMoves 列出每个容器目录的 {from,to} 迁移对（跳过空名）。
// 幂等判定（目标已存在则跳过实际搬移）在执行处 moveDir 处理。
func planRelocateMoves(oldLxcPath, newLxcPath string, names []string) []moveStep {
	steps := make([]moveStep, 0, len(names))
	for _, n := range names {
		if n == "" {
			continue
		}
		steps = append(steps, moveStep{
			From: filepath.Join(oldLxcPath, n),
			To:   filepath.Join(newLxcPath, n),
		})
	}
	return steps
}

// lxcConfPath 是中央 lxc 配置文件路径（写 lxc.lxcpath 让 lxc-tools 读新路径）。
// 用变量便于测试替换；生产固定 /etc/lxc/lxc.conf。
var lxcConfPath = "/etc/lxc/lxc.conf"

// RelocateParams 是 LXC 存储迁移任务的参数。
type RelocateParams struct {
	NewLxcPath   string
	NewImportDir string // 级联后的目标 import dir
	OldLxcPath   string // 日志/回退参考
	OldImportDir string
}

// enumerateContainerDirs 列出 lxcpath 下所有容器目录（含金基底 lxc__tmpl__*），排除 _imports 与非目录。
func enumerateContainerDirs(lxcpath string) ([]string, error) {
	entries, err := os.ReadDir(lxcpath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "_imports" {
			continue
		}
		names = append(names, e.Name())
	}
	return names, nil
}

// crossFsCopyTimeout 是跨文件系统 cp -a 一个容器/模板 rootfs 的超时。
// LXC rootfs 可达数 GB，且目标常为较慢的存储（如带校验/压缩的 ZFS），
// 默认 30s 必然不够；30 分钟覆盖大 rootfs 在慢盘上的拷贝。
const crossFsCopyTimeout = 30 * time.Minute

// moveDir 把 from 目录搬到 to。同文件系统优先 os.Rename；跨文件系统回退 cp -a + rm -rf。
// 幂等（安全）：仅当「源已消失 且 目标已就位」时跳过（上次 cp+rm 都完成）。
// 若 to 是上次被超时/失败 cp 留下的半成品（from 仍在），先清掉 to 再重拷，避免把半成品当完成。
func moveDir(from, to string) error {
	if from == to {
		return nil
	}
	// 幂等：源已不在。若目标也在 → 上次完全迁完，跳过。
	if _, err := os.Stat(from); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("stat 源目录失败: %w", err)
	}
	// 目标父目录要先在（rename/cp 都需要）。
	if err := os.MkdirAll(filepath.Dir(to), 0755); err != nil {
		return fmt.Errorf("创建目标父目录失败: %w", err)
	}
	// 清掉残留的半成品 to（失败重试场景），避免 cp 合并进半棵树。
	if _, err := os.Stat(to); err == nil {
		if rm := utils.ExecCommandLongRunning("rm", "-rf", to); rm.Error != nil {
			return fmt.Errorf("清理残留目标目录失败: %w", rm.Error)
		}
	}
	// 同文件系统：rename 瞬时完成。
	if err := os.Rename(from, to); err == nil {
		return nil
	}
	// 跨文件系统：cp -a <from> <to父目录> → 产生 <to父>/<basename(from)>。
	// 直接传参（不走 shell），无注入面；rootfs 大，用长超时。
	name := filepath.Base(from)
	cp := utils.ExecCommandWithTimeout("cp", crossFsCopyTimeout, "-a", from, filepath.Dir(to))
	if cp.Error != nil {
		return fmt.Errorf("cp -a 失败: %w", cp.Error)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(to), name)); err != nil {
		return fmt.Errorf("cp -a 后目标不存在: %w", err)
	}
	rm := utils.ExecCommandLongRunning("rm", "-rf", from)
	if rm.Error != nil {
		// 目标已就绪；源未删。重试时幂等判定会因 from 仍在而重拷（安全，仅浪费一次拷贝）。
		return fmt.Errorf("删除源目录失败（目标已就绪）: %w", rm.Error)
	}
	return nil
}

// rewriteContainerConfigFile 改写 <containerDir>/config 内的旧 lxcpath 前缀为新值（原子替换；无 config 跳过）。
func rewriteContainerConfigFile(containerDir, oldLxcPath, newLxcPath string) error {
	cfg := filepath.Join(containerDir, "config")
	data, err := os.ReadFile(cfg)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	rewritten := rewriteContainerConfig(string(data), oldLxcPath, newLxcPath)
	if rewritten == string(data) {
		return nil
	}
	tmp := cfg + ".tmp"
	if err := os.WriteFile(tmp, []byte(rewritten), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, cfg)
}

// writeLxcConf 原子写 /etc/lxc/lxc.conf：替换/追加 lxc.lxcpath 行，保留其它行。
func writeLxcConf(newLxcPath string) error {
	var existing string
	if data, err := os.ReadFile(lxcConfPath); err == nil {
		existing = string(data)
	}
	rewritten := rewriteLxcConf(existing, newLxcPath)
	if err := os.MkdirAll(filepath.Dir(lxcConfPath), 0755); err != nil {
		return err
	}
	tmp := lxcConfPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(rewritten), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, lxcConfPath)
}

// persistLxcPaths 把新 lxcpath/import_dir 写入运行态 config + DB settings + .env。
func persistLxcPaths(newLxcPath, newImportDir string) {
	config.GlobalConfig.LXCLxcPath = newLxcPath
	config.GlobalConfig.LXCTemplateImportDir = newImportDir
	if err := model.SetSetting("lxc_lxc_path", newLxcPath); err != nil {
		logger.App.Warn("持久化 lxc_lxc_path 到 DB 失败", "value", newLxcPath, "error", err)
	}
	if err := model.SetSetting("lxc_template_import_dir", newImportDir); err != nil {
		logger.App.Warn("持久化 lxc_template_import_dir 到 DB 失败", "value", newImportDir, "error", err)
	}
	config.SyncEnvFile()
}

// finalizeRelocate 执行「写 lxc.conf → 更新应用配置 → 同步缓存」（Relocate 与 SwitchLxcPath 共享）。
func finalizeRelocate(newLxcPath, newImportDir string, progress func(int, string)) error {
	if progress != nil {
		progress(80, "切换 lxc.lxcpath")
	}
	if err := writeLxcConf(newLxcPath); err != nil {
		return fmt.Errorf("写 lxc.conf 失败: %w", err)
	}
	if progress != nil {
		progress(85, "更新应用配置")
	}
	persistLxcPaths(newLxcPath, newImportDir)
	if progress != nil {
		progress(90, "同步容器缓存")
	}
	if err := SyncContainerCache(); err != nil {
		logger.App.Warn("同步容器缓存失败", "error", err)
	}
	return nil
}

// detectRunningContainers 用 lxc-ls --fancy 取当前运行中的容器（含金基底，确保搬移前全部停止）。
func detectRunningContainers() []string {
	res := LxcLsFancy()
	if res.Error != nil {
		return nil
	}
	items, err := ParseLxcLsFancy(res.Stdout)
	if err != nil {
		return nil
	}
	var running []string
	for _, it := range items {
		if it.Running {
			running = append(running, it.Name)
		}
	}
	return running
}

// Relocate 执行完整迁移（后台任务）：停→搬→改 config→写 lxc.conf→更新 config→重启→同步缓存。
// 任一搬移失败立即返回错误，且不写 lxc.conf、不更新 config（数据完好但 lxc-tools 仍读旧路径，可重试）。
func Relocate(p RelocateParams, progress func(int, string)) error {
	progress(5, "记录运行中容器")
	// 这里重新枚举旧路径目录（handler 的 EstimateRelocateTargets 仅用于弹窗展示与决策），
	// 以任务执行时刻的文件系统为准，确保实际搬迁覆盖当前所有容器/模板目录。
	names, err := enumerateContainerDirs(p.OldLxcPath)
	if err != nil {
		return fmt.Errorf("枚举旧路径容器失败: %w", err)
	}
	wasRunning := detectRunningContainers()

	progress(15, "停止运行中容器")
	for _, name := range wasRunning {
		if res := utils.ExecCommandLongRunning("lxc-stop", "-n", name); res.Error != nil {
			logger.App.Warn("lxc-stop 失败，继续迁移", "name", name, "error", res.Error)
		}
	}

	steps := planRelocateMoves(p.OldLxcPath, p.NewLxcPath, names)
	for i, st := range steps {
		pct := 20 + (50*(i+1))/(len(steps)+1)
		progress(pct, fmt.Sprintf("迁移 %d/%d: %s", i+1, len(steps), filepath.Base(st.From)))
		if err := moveDir(st.From, st.To); err != nil {
			return fmt.Errorf("迁移失败 %s: %w（已迁 %d/%d）", filepath.Base(st.From), err, i, len(steps))
		}
		if err := rewriteContainerConfigFile(st.To, p.OldLxcPath, p.NewLxcPath); err != nil {
			logger.App.Warn("改写容器 config 路径失败", "name", filepath.Base(st.To), "error", err)
		}
	}

	if err := finalizeRelocate(p.NewLxcPath, p.NewImportDir, progress); err != nil {
		return err
	}

	progress(95, "重启运行中容器")
	for _, name := range wasRunning {
		if res := utils.ExecCommandLongRunning("lxc-start", "-n", name); res.Error != nil {
			logger.App.Warn("lxc-start 失败，请手动处理", "name", name, "error", res.Error)
		}
	}
	progress(100, "迁移完成")
	return nil
}

// SwitchLxcPath 无容器时的轻量切换（同步）：仅写 lxc.conf + 更新 config + 同步缓存。
func SwitchLxcPath(newLxcPath, newImportDir string) error {
	return finalizeRelocate(newLxcPath, newImportDir, nil)
}

// EstimateRelocateTargets 探测迁移规模：用户容器数（lxc-ls 非基底）、模板数（DB 行，authoritative）、
// 待搬目录总数（旧 lxcpath 下排除 _imports 的目录数；>0 即需迁移）。
func EstimateRelocateTargets() (containers, templates, totalDirs int, err error) {
	if res := LxcLsFancy(); res.Error == nil {
		if items, perr := ParseLxcLsFancy(res.Stdout); perr == nil {
			for _, it := range items {
				if !isHiddenContainer(it.Name) {
					containers++
				}
			}
		}
	}
	var tc int64
	if merr := model.DB.Model(&model.LXCTemplate{}).Count(&tc).Error; merr == nil {
		templates = int(tc)
	}
	names, derr := enumerateContainerDirs(config.GlobalConfig.LXCLxcPath)
	if derr != nil {
		err = derr
		return
	}
	totalDirs = len(names)
	return
}
