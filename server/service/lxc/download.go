package lxc

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"qvmhub/utils"
)

// DownloadImageEntry 是 lxc-create -t download -- --list 里的一条镜像（distro/release/arch）。
type DownloadImageEntry struct {
	Distro  string `json:"distro"`
	Release string `json:"release"`
	Arch    string `json:"arch"`
}

// downloadListCache 缓存 --list 结果（拉镜像索引较慢）。进程内缓存，10 分钟 TTL。
var (
	downloadListCache    []DownloadImageEntry
	downloadListCachedAt time.Time
	downloadListMu       sync.Mutex
	downloadListCacheTTL = 10 * time.Minute
)

// DownloadImageList 拉取官方镜像清单（带缓存）。
// 注意：lxc-create -t download -n <dummy> -- --list 非交互、会退出，但 exit code 非零
// （--list 只列不建，lxc-create 视为 create 失败）——故忽略 exit code，按 stdout 解析；
// 解析为空（无外网/格式异常）则返回错误。
func DownloadImageList() ([]DownloadImageEntry, error) {
	downloadListMu.Lock()
	defer downloadListMu.Unlock()
	if time.Since(downloadListCachedAt) < downloadListCacheTTL && len(downloadListCache) > 0 {
		return downloadListCache, nil
	}
	res := utils.ExecCommandWithTimeout("lxc-create", 2*time.Minute, "-t", "download", "-n", "__qvm_imglist__", "--", "--list")
	entries := parseDownloadList(res.Stdout)
	if len(entries) == 0 {
		tail := res.Stdout
		if len(tail) > 200 {
			tail = tail[:200] + "..."
		}
		return nil, fmt.Errorf("拉取官方镜像清单失败（stdout 无镜像行，可能宿主机无外网）: %s", tail)
	}
	downloadListCache = entries
	downloadListCachedAt = time.Now()
	return entries, nil
}

// parseDownloadList 解析 lxc-create -t download -- --list 的 stdout（纯函数，便于单测）。
// 找到 "DIST RELEASE ARCH VARIANT BUILD" 表头行后，逐行取 5 字段（空白分隔），
// 只保留 VARIANT=default，去重。无表头/无有效行 → 空 slice。
func parseDownloadList(stdout string) []DownloadImageEntry {
	seen := map[string]bool{}
	var out []DownloadImageEntry
	headerSeen := false
	for _, line := range strings.Split(stdout, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if !headerSeen {
			if strings.HasPrefix(trim, "DIST") && strings.Contains(trim, "RELEASE") {
				headerSeen = true
			}
			continue
		}
		fields := strings.Fields(trim)
		if len(fields) < 5 {
			continue
		}
		distro, release, arch, variant := fields[0], fields[1], fields[2], fields[3]
		if variant != "default" {
			continue
		}
		key := distro + "|" + release + "|" + arch
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, DownloadImageEntry{Distro: distro, Release: release, Arch: arch})
	}
	return out
}
