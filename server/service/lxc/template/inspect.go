package template

import (
	"os"
	"path/filepath"
	"strings"
)

// backingFromRootfsPath 由 lxc.rootfs.path 的 scheme 推断 backing。
// "zfs:..." → zfs；"overlayfs:..." → overlay；其余（dir:/btrfs:/lvm: 或裸路径）→ dir。
// 与 validateImportParams 支持的 dir/overlay/zfs 集合一致。
func backingFromRootfsPath(rootfsPath string) string {
	s := strings.TrimSpace(rootfsPath)
	if i := strings.IndexByte(s, ':'); i > 0 {
		switch s[:i] {
		case "zfs":
			return "zfs"
		case "overlayfs":
			return "overlay"
		}
	}
	return "dir"
}

// extractRootfsPathFromConfig 从 lxc config 文本取 lxc.rootfs.path 的裸值（含 scheme）。
func extractRootfsPathFromConfig(cfg string) string {
	for _, line := range strings.Split(cfg, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		eq := strings.IndexByte(trim, '=')
		if eq <= 0 {
			continue
		}
		if strings.TrimSpace(trim[:eq]) == "lxc.rootfs.path" {
			return strings.TrimSpace(trim[eq+1:])
		}
	}
	return ""
}

// parseLxcInfoState 从 lxc-info -n <name> 的 "Key: value" 输出取 State（大写，如 RUNNING/STOPPED）。
// 模板包不能 import service/lxc（环引），故独立实现一份最小解析。
func parseLxcInfoState(stdout string) string {
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(line[:idx]), "State") {
			return strings.TrimSpace(line[idx+1:])
		}
	}
	return ""
}

// readOSReleaseFromDir 读 <rootfsDir>/etc/os-release（回退 usr/lib/os-release），
// 复用 parseOSRelease 取 distro/release。缺失或读失败返回空串（best-effort）。
func readOSReleaseFromDir(rootfsDir string) (distro, release string) {
	for _, rel := range []string{"etc/os-release", "usr/lib/os-release"} {
		b, err := os.ReadFile(filepath.Join(rootfsDir, rel))
		if err != nil || len(strings.TrimSpace(string(b))) == 0 {
			continue
		}
		if d, r := parseOSRelease(string(b)); d != "" || r != "" {
			return d, r
		}
	}
	return "", ""
}
