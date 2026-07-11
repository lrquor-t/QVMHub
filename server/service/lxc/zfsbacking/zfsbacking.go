// Package zfsbacking 提供模板/容器的 zfs dataset 操作。
//
// 单独成包（不在 package lxc 内）以打破 import 环：
// service/lxc（create.go）依赖 service/lxc/template，而 template 又要调 zfs 命令。
// 把 zfs 命令下沉到这个不依赖 lxc/template 的叶子包后，service/lxc 与
// service/lxc/template 都可安全 import 它。service/lxc/zfs.go 仅做同名再导出，
// 供 lxc 包内部（create/destroy 等小写调用）与既有 lxc.Zfs* 形态的调用方继续使用。
package zfsbacking

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"qvmhub/config"
	"qvmhub/utils"
)

// zfs backing：lxc-create -t none 不设 rootfs，lxc-copy -B zfs 是全量拷贝，故走手动 zfs clone（CoW）。
// 布局（每个容器一个 dataset，rootfs 是子目录；parent = 挂载在 LXCLxcPath 的 zfs dataset）：
//   <parent>/<base>            模板容器 dataset（含 config + rootfs/）
//   <parent>/<base>@base       模板快照（导入末尾打一次，所有容器克隆源）
//   <parent>/<name>            容器 dataset（clone of @base），mountpoint <lxcpath>/<name>

const BaseSnap = "@base"

// —— 纯函数：dataset 名构造（便于单测）——

func BaseDataset(parent, base string) string          { return parent + "/" + base }
func BaseSnapshot(parent, base string) string         { return BaseDataset(parent, base) + BaseSnap }
func ContainerDataset(parent, name string) string     { return parent + "/" + name }
func ContainerMountpoint(lxcpath, name string) string { return filepath.Join(lxcpath, name) }

// CommentProperty 是记录快照备注的 zfs user property（pm03 实测 set/get 正常，中文值保留）。
const CommentProperty = "kvm_console:comment"

// ContainerSnapshot 构造容器用户快照名 <parent>/<name>@<snap>（纯函数，便于单测）。
func ContainerSnapshot(parent, name, snap string) string {
	return ContainerDataset(parent, name) + "@" + snap
}

// RewriteRootfsPathForClone 把克隆 config 里继承自基底的 rootfs 路径替换为容器自己的（纯函数）。
func RewriteRootfsPathForClone(cfg, oldRootfsPath, newRootfsPath string) string {
	return strings.ReplaceAll(cfg, oldRootfsPath, newRootfsPath)
}

// —— zfs 命令封装（实测命令；非单测，靠真机手测 Task 7）——

// ResolveParent 返回挂载在 lxcpath 的 zfs dataset 名（如 /zp01/lxc → zp01/lxc）。
func ResolveParent(lxcpath string) (string, error) {
	res := utils.ExecCommand("zfs", "list", "-Ho", "name", lxcpath)
	if res.Error != nil {
		return "", fmt.Errorf("解析 lxcpath 的 zfs dataset 失败（%s 不是 zfs 挂载点？backing=zfs 仅支持 lxc 目录在 zfs 上）: %w", lxcpath, res.Error)
	}
	parent := strings.TrimSpace(res.Stdout)
	if parent == "" {
		return "", fmt.Errorf("lxcpath %s 未对应任何 zfs dataset", lxcpath)
	}
	return parent, nil
}

// CreateBase 创建模板容器 dataset <parent>/<base>（rootfs 是其子目录，不再单独建 dataset）。
func CreateBase(parent, base string) error {
	ds := BaseDataset(parent, base)
	// 若 dataset 已存在（上次失败残留；existsContainer 对无 config 的残留查不到）→ 先清理再建，保证导入可重试。
	if res := utils.ExecCommand("zfs", "list", "-Ho", "name", ds); res.Error == nil {
		if err := renameAndDestroy(ds); err != nil {
			return fmt.Errorf("清理残留基底 dataset 失败: %w", err)
		}
	}
	if res := utils.ExecCommand("zfs", "create", "-p", ds); res.Error != nil {
		return fmt.Errorf("zfs create 模板 dataset 失败: %w", res.Error)
	}
	return nil
}

// SnapshotBase 给模板 dataset 打 @base 快照（导入末尾一次；克隆源）。
func SnapshotBase(parent, base string) error {
	if res := utils.ExecCommand("zfs", "snapshot", BaseSnapshot(parent, base)); res.Error != nil {
		return fmt.Errorf("zfs snapshot @base 失败: %w", res.Error)
	}
	return nil
}

// DestroyBase 销毁模板 dataset（rename 到回收名后 destroy -r，连带 @base 快照；有克隆时 zfs 会拒绝）。
func DestroyBase(parent, base string) error {
	return renameAndDestroy(BaseDataset(parent, base))
}

// CloneContainer 从 <parent>/<base>@base 克隆出 <parent>/<name>，mountpoint 设到 <lxcpath>/<name>。
// 克隆继承基底 config + rootfs（CoW）；调用方随后改写 config 的 rootfs.path。
func CloneContainer(parent, base, name string) error {
	ds := ContainerDataset(parent, name)
	if res := utils.ExecCommand("zfs", "clone", BaseSnapshot(parent, base), ds); res.Error != nil {
		return fmt.Errorf("zfs clone 失败: %w", res.Error)
	}
	if res := utils.ExecCommand("zfs", "set", "mountpoint="+ContainerMountpoint(config.GlobalConfig.LXCLxcPath, name), ds); res.Error != nil {
		return fmt.Errorf("zfs set mountpoint 失败: %w", res.Error)
	}
	return nil
}

// CloneContainerFromSnapshot 从任意快照 <parent>/<src>@<snap> CoW 克隆出 <parent>/<dst>，
// mountpoint 设到 <lxcpath>/<dst>。预留用于后续「容器克隆」功能（容器→容器 CoW 克隆，origin 依赖可接受）。
// 注意：克隆出的 dataset 会依赖源快照（origin），源快照在该 dataset 存在期间不可删除。
// 故「从容器制作模板」改走 CopyContainerBySendRecv（全量拷贝、基底独立），不走本函数。
// 与 CloneContainer 的区别：源是任意容器快照而非基底 @base。
func CloneContainerFromSnapshot(parent, src, snap, dst string) error {
	srcSnap := ContainerSnapshot(parent, src, snap)
	ds := ContainerDataset(parent, dst)
	if res := utils.ExecCommand("zfs", "clone", srcSnap, ds); res.Error != nil {
		return fmt.Errorf("zfs clone 失败: %w", res.Error)
	}
	if res := utils.ExecCommand("zfs", "set", "mountpoint="+ContainerMountpoint(config.GlobalConfig.LXCLxcPath, dst), ds); res.Error != nil {
		// 回滚已 clone 的 dataset：否则其 origin 依赖会锁住源快照（has_clones=true → 源快照不可删）。
		_ = renameAndDestroy(ds)
		return fmt.Errorf("zfs set mountpoint 失败: %w", res.Error)
	}
	return nil
}

// CopyContainerBySendRecv 用 zfs send|zfs receive 把 <parent>/<src>@<snap> 全量拷贝成
// 独立 dataset <parent>/<dst>（receive 出的不带 origin 依赖），mountpoint 设到 <lxcpath>/<dst>。
// 用于「从容器制作模板」：源容器快照 → 全量拷贝成基底 dataset。
//
// 为什么不沿用 CloneContainerFromSnapshot（zfs clone，CoW）：clone 出的 dataset 会依赖源快照
// （origin），导致源快照在基底存在期间不可删除——源容器既清不掉 _tmplmake、自身也删不掉。
// send|receive 是全量拷贝、基底完全独立，源快照可正常清理。
// 代价：无 CoW（一次性全量拷贝），但制作模板是低频操作，后续建容器仍走 @base 的 CoW 克隆。
func CopyContainerBySendRecv(parent, src, snap, dst string) error {
	srcSnap := ContainerSnapshot(parent, src, snap)
	ds := ContainerDataset(parent, dst)
	// -u：receive 后不挂载（流的 mountpoint 是源的 <lxcpath>/<src>，已被源容器占用），随后显式改 mountpoint 再挂载。
	// pipefail：send 失败时让管道整体非零退出，避免被 receive 的退出码掩盖。
	cmd := fmt.Sprintf("set -o pipefail; zfs send %s | zfs receive -u %s",
		utils.ShellSingleQuote(srcSnap), utils.ShellSingleQuote(ds))
	res := utils.ExecShellWithTimeout(cmd, 30*time.Minute)
	if res.Error != nil {
		return fmt.Errorf("zfs send|receive 拷贝失败: %w (stderr: %q)", res.Error, res.Stderr)
	}
	// 改 mountpoint 到 <lxcpath>/<dst> 并挂载；后续步骤失败则回滚已 receive 的 dst dataset，避免残留。
	mountpoint := ContainerMountpoint(config.GlobalConfig.LXCLxcPath, dst)
	if res := utils.ExecCommand("zfs", "set", "mountpoint="+mountpoint, ds); res.Error != nil {
		_ = renameAndDestroy(ds)
		return fmt.Errorf("zfs set mountpoint 失败: %w", res.Error)
	}
	if res := utils.ExecCommand("zfs", "mount", ds); res.Error != nil {
		// set mountpoint 时若 canmount=on 可能已自动挂载，already mounted 视为成功
		if !strings.Contains(res.Stderr, "already mounted") {
			_ = renameAndDestroy(ds)
			return fmt.Errorf("zfs mount 失败: %w", res.Error)
		}
	}
	// receive 会顺带在 dst 上留 @<snap> 快照；dst 已独立、无克隆依赖它，清掉保持干净（best-effort）。
	_ = utils.ExecCommandQuiet("zfs", "destroy", ContainerSnapshot(parent, dst, snap))
	return nil
}

// DestroyContainer 销毁容器 dataset <parent>/<name>（rename 到回收名后 destroy -r，连带其快照）。
// 调用方再 os.RemoveAll 清理残留空目录。
func DestroyContainer(parent, name string) error {
	return renameAndDestroy(ContainerDataset(parent, name))
}

// ZfsSnapshot 是 ListContainerSnapshots 的返回元素。
type ZfsSnapshot struct {
	Name      string
	Comment   string
	CreatedAt string
	Clones    string // zfs clones 属性：依赖此快照的克隆 dataset（逗号分隔），"-" 或空=无克隆
}

// SnapshotContainer 给容器 dataset 打用户快照。
func SnapshotContainer(parent, name, snap string) error {
	if res := utils.ExecCommand("zfs", "snapshot", ContainerSnapshot(parent, name, snap)); res.Error != nil {
		return fmt.Errorf("zfs snapshot 容器快照失败: %w", res.Error)
	}
	return nil
}

// SetSnapshotComment 把备注写入快照的 zfs user property（comment 为空则跳过）。
func SetSnapshotComment(parent, name, snap, comment string) error {
	if comment == "" {
		return nil
	}
	ds := ContainerSnapshot(parent, name, snap)
	if res := utils.ExecCommand("zfs", "set", CommentProperty+"="+comment, ds); res.Error != nil {
		return fmt.Errorf("zfs set 快照备注失败: %w", res.Error)
	}
	return nil
}

// RollbackContainer 回滚容器 dataset 到指定快照。
// pm03 实测：挂载态 dataset 可直接回滚（无需 umount/-R）；-r 在有更新快照时销毁它们、无则无害。
// 调用方负责先 lxc-stop（语义安全：不回滚运行中容器的 live rootfs）。
func RollbackContainer(parent, name, snap string) error {
	if res := utils.ExecCommand("zfs", "rollback", "-r", ContainerSnapshot(parent, name, snap)); res.Error != nil {
		return fmt.Errorf("zfs rollback 失败: %w", res.Error)
	}
	return nil
}

// DestroyContainerSnapshot 销毁单个容器快照（不加 -r，仅删该快照本身）。
func DestroyContainerSnapshot(parent, name, snap string) error {
	if res := utils.ExecCommand("zfs", "destroy", ContainerSnapshot(parent, name, snap)); res.Error != nil {
		return fmt.Errorf("zfs destroy 容器快照失败: %w", res.Error)
	}
	return nil
}

// ListContainerSnapshots 列出容器 dataset 的用户快照（zfs 默认旧→新）。
// 用 -H 制表符分隔，按 \t 切列（creation 含空格、comment 可含空格都安全），剥 ds@ 前缀得快照名。
// normalizeZfsCreation 把 zfs creation 列在 LANG=C 下的英文格式（如 "Sun Jul  5 13:26 2026"）
// 规整为 "2006-01-02 15:04:05"，与 dir 路径（lxc-snapshot -L）的输出一致，便于前端统一展示。
// 解析失败则原样返回（去首尾空白）。
func normalizeZfsCreation(s string) string {
	collapsed := strings.Join(strings.Fields(s), " ") // 折叠多空格（zfs 单日会留双空格）
	t, err := time.Parse("Mon Jan 2 15:04 2006", collapsed)
	if err != nil {
		return strings.TrimSpace(s)
	}
	return t.Format("2006-01-02 15:04:05")
}

func ListContainerSnapshots(parent, name string) ([]ZfsSnapshot, error) {
	ds := ContainerDataset(parent, name)
	res := utils.ExecCommand("zfs", "list", "-H", "-t", "snapshot",
		"-o", "name,creation,"+CommentProperty+",clones", "-r", "-d1", ds)
	if res.Error != nil {
		return nil, fmt.Errorf("zfs list 容器快照失败: %w", res.Error)
	}
	prefix := ds + "@"
	var out []ZfsSnapshot
	for _, line := range strings.Split(res.Stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 3 || !strings.HasPrefix(f[0], prefix) {
			continue
		}
		comment := f[2]
		if comment == "-" {
			comment = ""
		}
		clones := ""
		if len(f) >= 4 {
			clones = f[3]
		}
		out = append(out, ZfsSnapshot{
			Name:      strings.TrimPrefix(f[0], prefix),
			CreatedAt: normalizeZfsCreation(f[1]),
			Comment:   comment,
			Clones:    clones,
		})
	}
	return out, nil
}

// SnapshotHasClones 报告快照 <parent>/<name>@<snap> 是否有依赖它的 origin 克隆 dataset。
// 有克隆时该快照不可销毁（zfs 会拒绝）。查 clones 属性，非 "-" 且非空即视为有克隆。
func SnapshotHasClones(parent, name, snap string) (bool, error) {
	res := utils.ExecCommand("zfs", "get", "-H", "-o", "value", "clones", ContainerSnapshot(parent, name, snap))
	if res.Error != nil {
		return false, fmt.Errorf("zfs get clones 失败: %w", res.Error)
	}
	v := strings.TrimSpace(res.Stdout)
	return v != "" && v != "-", nil
}

// CreateContainerDataset 创建容器 dataset <parent>/<name>（download 模式 zfs backing 用：
// 先建 dataset，再 lxc-create -t download 把 rootfs 填进去）。
func CreateContainerDataset(parent, name string) error {
	ds := ContainerDataset(parent, name)
	if res := utils.ExecCommand("zfs", "create", "-p", ds); res.Error != nil {
		return fmt.Errorf("zfs create 容器 dataset 失败: %w", res.Error)
	}
	// 显式设 mountpoint（不依赖父 dataset 的 mountpoint 继承），确保 dataset 挂在 <lxcpath>/<name>
	if res := utils.ExecCommand("zfs", "set", "mountpoint="+ContainerMountpoint(config.GlobalConfig.LXCLxcPath, name), ds); res.Error != nil {
		return fmt.Errorf("zfs set mountpoint 失败: %w", res.Error)
	}
	return nil
}

// renameAndDestroy 先把 dataset rename 到 .del-<ts> 回收名（释放原名、隔离失败），
// 再 zfs destroy -r（连带快照/子 dataset）。直接 destroy 在有快照（lxc-snapshot）时会失败。
// rename 失败（dataset 已不存在等）则兜底直接 destroy -r 原名。
func renameAndDestroy(ds string) error {
	trash := ds + ".del-" + time.Now().UTC().Format("20060102-150405")
	if res := utils.ExecCommand("zfs", "rename", ds, trash); res.Error == nil {
		if res := utils.ExecCommand("zfs", "destroy", "-r", trash); res.Error != nil {
			return fmt.Errorf("zfs destroy -r %s 失败: %w", trash, res.Error)
		}
		return nil
	}
	// rename 失败 → 兜底直接 destroy -r 原名（dataset 可能已不存在，错误由调用方记录）
	if res := utils.ExecCommand("zfs", "destroy", "-r", ds); res.Error != nil {
		return fmt.Errorf("zfs destroy -r %s 失败: %w", ds, res.Error)
	}
	return nil
}

// IsLxcpathZfs 报告 lxcpath 是否挂载在一个 zfs dataset 上（用于前端给"dir on zfs"提示）。
func IsLxcpathZfs(lxcpath string) bool {
	res := utils.ExecCommand("zfs", "list", "-Ho", "name", lxcpath)
	return res.Error == nil && strings.TrimSpace(res.Stdout) != ""
}

// IsZfsContainer 判断 <lxcpath>/<name> 是否本身就是 zfs dataset 挂载点（zfs 容器），
// 还是父 dataset 上的普通子目录（dir/overlay 容器）。查 zfs 该路径的 mountpoint：
// zfs 容器 → mountpoint == 该路径；dir 容器 → mountpoint 是父（如 /zp01/lxc）≠ 该路径。
// 比 DB Backing 更稳：不受孤儿/手工篡改影响（看真实文件系统状态）。
func IsZfsContainer(name string) bool {
	p := filepath.Join(config.GlobalConfig.LXCLxcPath, name)
	res := utils.ExecCommand("zfs", "list", "-Ho", "mountpoint", p)
	if res.Error != nil {
		return false
	}
	return strings.TrimSpace(res.Stdout) == p
}

// SetContainerRefquota 设/取消容器 dataset 的 refquota。bytes>0 设上限；<=0 取消(refquota=none)。
func SetContainerRefquota(parent, name string, bytes int64) error {
	val := "none"
	if bytes > 0 {
		val = strconv.FormatInt(bytes, 10)
	}
	if res := utils.ExecCommand("zfs", "set", "refquota="+val, ContainerDataset(parent, name)); res.Error != nil {
		return fmt.Errorf("zfs set refquota 失败: %w (%s)", res.Error, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// GetContainerRefquota 读容器 refquota（字节，用 -p 取精确值）；none/0/异常→0。
func GetContainerRefquota(parent, name string) (int64, error) {
	res := utils.ExecCommand("zfs", "get", "-Hp", "-o", "value", "refquota", ContainerDataset(parent, name))
	if res.Error != nil {
		return 0, fmt.Errorf("zfs get refquota 失败: %w", res.Error)
	}
	v := strings.TrimSpace(res.Stdout)
	if v == "" || v == "none" || v == "-" {
		return 0, nil
	}
	bytes, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, nil // 兜底：非常规值视作 0
	}
	return bytes, nil
}
