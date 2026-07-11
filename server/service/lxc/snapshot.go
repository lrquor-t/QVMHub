package lxc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"qvmhub/config"
	"qvmhub/utils"
)

// LXCSnapshot 是前端展示的快照（list 返回）。
type LXCSnapshot struct {
	Name      string `json:"name"`
	Comment   string `json:"comment"`
	CreatedAt string `json:"created_at"`
	HasClones bool   `json:"has_clones"` // 是否有 zfs origin 克隆依赖（有则不可删除）
}

// SnapshotParams 是异步创建快照任务的参数。
type SnapshotParams struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
}

// ParseSnapshotParams 反序列化创建快照任务参数。
func ParseSnapshotParams(s string) (*SnapshotParams, error) {
	var p SnapshotParams
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ListSnapshots 列出容器快照（按 backing 分支；返回新→旧）。
func ListSnapshots(name string) ([]LXCSnapshot, error) {
	if isZfsContainer(name) {
		return listZfsSnapshots(name)
	}
	return listDirSnapshots(name)
}

func listZfsSnapshots(name string) ([]LXCSnapshot, error) {
	parent, err := ZfsResolveParent(config.GlobalConfig.LXCLxcPath)
	if err != nil {
		return nil, err
	}
	zs, err := zfsListContainerSnapshots(parent, name)
	if err != nil {
		return nil, err
	}
	out := make([]LXCSnapshot, 0, len(zs))
	for i := len(zs) - 1; i >= 0; i-- { // zfs 默认旧→新，反转为新→旧
		out = append(out, LXCSnapshot{
			Name:      zs[i].Name,
			Comment:   zs[i].Comment,
			CreatedAt: zs[i].CreatedAt,
			HasClones: zs[i].Clones != "" && zs[i].Clones != "-",
		})
	}
	return out, nil
}

func listDirSnapshots(name string) ([]LXCSnapshot, error) {
	res := utils.ExecCommand("lxc-snapshot", "-n", name, "-L")
	if res.ExitCode != 0 {
		if strings.Contains(res.Stderr, "no snapshot") || strings.Contains(res.Stderr, "not supported") {
			return []LXCSnapshot{}, nil
		}
		return nil, errors.New("列出快照失败: " + res.Stderr)
	}
	snaps := parseSnapshotList(res.Stdout)
	for i, j := 0, len(snaps)-1; i < j; i, j = i+1, j-1 { // lxc-snapshot -L 旧→新(snap0..)，反转为新→旧
		snaps[i], snaps[j] = snaps[j], snaps[i]
	}
	return snaps, nil
}

// sanitizeSnapshotComment 清洗快照备注：把连续的 \n/\r/\t 折叠成单个空格，并截到 200 runes（与前端 maxlength 一致）。
// 纯函数；CreateSnapshot 在两条分支前的单一 chokepoint 调用，防止控制符进入 zfs user property（导致 strings.Split("\n") 错位）
// 或 dir 备注（导致 parseSnapshotList 的 strings.Fields 错切），从而引发列表静默丢/截。
func sanitizeSnapshotComment(s string) string {
	// 1) 把每个 \n/\r/\t 视为分隔符，相邻连续运行折叠成单个空格（保留单词间不粘连）。
	var b strings.Builder
	b.Grow(len(s))
	inRun := false
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			inRun = true
			continue
		}
		if inRun {
			b.WriteByte(' ')
			inRun = false
		}
		b.WriteRune(r)
	}
	if inRun { // 末尾的控制符 run 也写成一个空格（保持"剥离 + 折叠"语义）
		b.WriteByte(' ')
	}
	cleaned := b.String()
	// 2) 截到 200 runes（rune-safe：不会切断多字节字符）。前端 maxlength=200 同此上限。
	const max = 200
	if utf8.RuneCountInString(cleaned) <= max {
		return cleaned
	}
	out := make([]rune, 0, max)
	for _, r := range cleaned {
		if len(out) >= max {
			break
		}
		out = append(out, r)
	}
	return string(out)
}

// parseSnapshotList 解析 lxc-snapshot -L 的 stdout（Name/Comment/Creation time 三列，空格对齐）。
// name=首段；creation=末两段(日期 时间)；comment=中间段(可含空格)；Comment 为 "-" 视为空。
func parseSnapshotList(stdout string) []LXCSnapshot {
	var out []LXCSnapshot
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Name") {
			continue
		}
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		snap := LXCSnapshot{Name: f[0]}
		if len(f) >= 4 {
			snap.CreatedAt = f[len(f)-2] + " " + f[len(f)-1]
			snap.Comment = strings.Join(f[1:len(f)-2], " ")
		}
		if snap.Comment == "-" {
			snap.Comment = ""
		}
		out = append(out, snap)
	}
	return out
}

// CreateSnapshot 对容器创建新快照（异步任务调用；可能耗时）。
func CreateSnapshot(name, comment string) error {
	comment = sanitizeSnapshotComment(comment) // 服务端清洗：避免 \n/\r/\t 进入 zfs user property 或 dir 备注，破坏后续列表解析
	if isZfsContainer(name) {
		return createZfsSnapshot(name, comment)
	}
	return createDirSnapshot(name, comment)
}

func createZfsSnapshot(name, comment string) error {
	parent, err := ZfsResolveParent(config.GlobalConfig.LXCLxcPath)
	if err != nil {
		return err
	}
	snap := "snap-" + time.Now().Format("20060102150405")
	if err := zfsSnapshotContainer(parent, name, snap); err != nil {
		return err
	}
	return zfsSetSnapshotComment(parent, name, snap, comment)
}

func createDirSnapshot(name, comment string) error {
	args := []string{"-n", name}
	if comment != "" {
		tmp, err := os.CreateTemp("", "lxc-snap-comment-*")
		if err != nil {
			return fmt.Errorf("创建备注临时文件失败: %w", err)
		}
		if _, err := tmp.WriteString(comment); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return fmt.Errorf("写入备注临时文件失败: %w", err)
		}
		tmp.Close()
		defer os.Remove(tmp.Name())
		args = append(args, "-c", tmp.Name())
	}
	res := utils.ExecCommandLongRunning("lxc-snapshot", args...)
	return res.Error
}

// RestoreSnapshot 从指定快照恢复（先防御性关机；zfs 回滚会销毁更新快照——前端已二次确认）。
func RestoreSnapshot(name, snap string) error {
	// 防御性关机：zfs 回滚不应作用于运行中容器的 live rootfs；dir 路径也更安全。
	_ = utils.ExecCommandQuiet("lxc-stop", "-n", name).Error
	if isZfsContainer(name) {
		parent, err := ZfsResolveParent(config.GlobalConfig.LXCLxcPath)
		if err != nil {
			return err
		}
		return zfsRollbackContainer(parent, name, snap)
	}
	res := utils.ExecCommandLongRunning("lxc-snapshot", "-n", name, "-r", snap)
	return res.Error
}

// DeleteSnapshot 删除指定快照。zfs 快照若有 origin 克隆依赖则不可销毁（前端已禁，此处兜底给中文提示）。
func DeleteSnapshot(name, snap string) error {
	if isZfsContainer(name) {
		parent, err := ZfsResolveParent(config.GlobalConfig.LXCLxcPath)
		if err != nil {
			return err
		}
		if has, _ := zfsSnapshotHasClones(parent, name, snap); has {
			return errors.New("该快照已被克隆为容器，无法删除（请先删除依赖它的克隆容器）")
		}
		return zfsDestroyContainerSnapshot(parent, name, snap)
	}
	res := utils.ExecCommandLongRunning("lxc-snapshot", "-n", name, "-d", snap)
	return res.Error
}
