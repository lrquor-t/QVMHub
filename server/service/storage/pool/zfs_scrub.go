package pool

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"qvmhub/utils"
)

// ── 类型 ──

// ZFSScrubStatus scrub 状态 + 健康度（GET /zfs-scrub/status 返回）。
type ZFSScrubStatus struct {
	Pool          string  `json:"pool"`
	Health        string  `json:"health"`         // ONLINE/DEGRADED/FAULTED/...
	ScrubState    string  `json:"scrub_state"`    // none/running/finished/canceled
	ProgressPct   float64 `json:"progress_pct"`   // 直接取自输出 <pct>% done
	ScannedBytes  int64   `json:"scanned"`
	IssuedBytes   int64   `json:"issued"`
	TotalBytes    int64   `json:"total"`
	RepairedBytes int64   `json:"repaired"`
	RateBps       int64   `json:"rate_bps"`       // 字节/秒（运行中）
	TimeRemaining string  `json:"time_remaining"` // "HH:MM:SS to go" 或空
	ErrorsFound   int64   `json:"errors_found"`   // 完成行 with N errors
	StartedAt     string  `json:"started_at"`     // 运行中 since <dt>
	FinishedAt    string  `json:"finished_at"`    // 完成行 on <dt>
	Duration      string  `json:"duration"`       // 完成行 in <dur>
	ReadErr       int64   `json:"read_err"`       // config 池行
	WriteErr      int64   `json:"write_err"`
	ChecksumErr   int64   `json:"checksum_err"`
}

// ZFSErrorList 永久错误文件清单（GET /zfs-errors 返回）。
type ZFSErrorList struct {
	Pool      string   `json:"pool"`
	Total     int      `json:"total"`
	Truncated bool     `json:"truncated"`
	TimedOut  bool     `json:"timed_out"`
	Files     []string `json:"files"`
}

// 错误清单展示上限。
const zfsErrorsMaxFiles = 200

// ── 纯函数：字节单位换算 ──

// parseZFSBytes 把 ZFS 人类可读字节量（"642M"/"0B"/"1.32M"）解析为字节数，base-1024。
// 输入含速率后缀 "/s" 时先剥离。无法解析返回 0。
func parseZFSBytes(s string) int64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "/s") // 容忍速率 "642M/s"
	if s == "" || s == "-" {
		return 0
	}
	var numStr, unit string
	for i, r := range s {
		if (r >= '0' && r <= '9') || r == '.' {
			continue
		}
		numStr = s[:i]
		unit = strings.TrimSpace(s[i:])
		break
	}
	if numStr == "" {
		numStr = s // 纯数字无单位
	}
	f, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}
	mul := map[string]float64{
		"": 1, "B": 1,
		"K": 1024, "M": 1024 * 1024, "G": 1024 * 1024 * 1024,
		"T": 1024 * 1024 * 1024 * 1024, "P": 1024 * 1024 * 1024 * 1024 * 1024,
		"E": 1024 * 1024 * 1024 * 1024 * 1024 * 1024,
	}
	return int64(f * mul[strings.ToUpper(unit)])
}

// ── 纯函数：解析 zpool status 输出 ──

var (
	zfsScrubRunningRe = regexp.MustCompile(`scrub in progress since\s+(.+)$`)
	zfsScrubDoneRe    = regexp.MustCompile(`scrub repaired\s+(\S+)\s+in\s+(\S+)\s+with\s+(\d+)\s+errors\s+on\s+(.+)$`)
	zfsScanLineRe     = regexp.MustCompile(`(\S+)\s+scanned at\s+(\S+),\s+(\S+)\s+issued at\s+(\S+),\s+(\S+)\s+total`)
	zfsPctRe          = regexp.MustCompile(`([\d.]+)%\s+done`)
	zfsEtaRe          = regexp.MustCompile(`(\d+:\d+:\d+)\s+to\s+go`)
)

// parseZpoolStatus 解析 `zpool status <pool>` 的英文输出（C locale）。
func parseZpoolStatus(raw string, pool string) (ZFSScrubStatus, error) {
	st := ZFSScrubStatus{Pool: pool, ScrubState: "none"}
	lines := strings.Split(raw, "\n")

	for _, line := range lines {
		trim := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trim, "state:"):
			st.Health = strings.TrimSpace(strings.TrimPrefix(trim, "state:"))
		case strings.HasPrefix(trim, "scan:"):
			rest := strings.TrimSpace(strings.TrimPrefix(trim, "scan:"))
			parseScanLine(rest, &st)
		}
	}

	// 运行中的 scan 块第 2/3 行（紧跟 scan: 的缩进行）——单独扫一遍
	if st.ScrubState == "running" {
		for _, line := range lines {
			c := strings.TrimSpace(line)
			if zfsScanLineRe.MatchString(c) && st.TotalBytes == 0 {
				m := zfsScanLineRe.FindStringSubmatch(c)
				st.ScannedBytes = parseZFSBytes(m[1])
				st.RateBps = parseZFSBytes(m[2])
				st.IssuedBytes = parseZFSBytes(m[3])
				st.TotalBytes = parseZFSBytes(m[5])
			}
			if strings.Contains(c, "% done") && st.ProgressPct == 0 {
				if m := zfsPctRe.FindStringSubmatch(c); len(m) > 1 {
					if v, err := strconv.ParseFloat(m[1], 64); err == nil {
						st.ProgressPct = v
					}
				}
				if strings.Contains(c, "no estimated completion time") {
					st.TimeRemaining = ""
				} else if m := zfsEtaRe.FindStringSubmatch(c); len(m) > 1 {
					st.TimeRemaining = m[1]
				}
				if strings.Contains(c, "repaired") {
					st.RepairedBytes = parseZFSBytes(scanFieldBefore(c, "repaired"))
				}
			}
		}
	}

	parseConfigErrors(lines, &st)
	return st, nil
}

// parseScanLine 解析 scan: 行本身（判定 state + 提取 since/on/duration/errors）。
func parseScanLine(rest string, st *ZFSScrubStatus) {
	switch {
	case strings.HasPrefix(rest, "scrub in progress"):
		st.ScrubState = "running"
		if m := zfsScrubRunningRe.FindStringSubmatch(rest); len(m) > 1 {
			st.StartedAt = strings.TrimSpace(m[1])
		}
	case strings.HasPrefix(rest, "scrub repaired"):
		st.ScrubState = "finished"
		if m := zfsScrubDoneRe.FindStringSubmatch(rest); len(m) >= 5 {
			st.RepairedBytes = parseZFSBytes(m[1])
			st.Duration = m[2]
			if n, err := strconv.ParseInt(m[3], 10, 64); err == nil {
				st.ErrorsFound = n
			}
			st.FinishedAt = strings.TrimSpace(m[4])
		}
	case strings.HasPrefix(rest, "scrub canceled"):
		st.ScrubState = "canceled"
	}
}

// scanFieldBefore 取 token 前一个空白分隔字段（如 "0B repaired" 的 "0B"）。
func scanFieldBefore(s, token string) string {
	idx := strings.Index(s, token)
	if idx < 0 {
		return ""
	}
	prefix := strings.TrimRight(s[:idx], " ")
	fs := strings.Fields(prefix)
	if len(fs) == 0 {
		return ""
	}
	return fs[len(fs)-1]
}

// parseConfigErrors 从 config: 段取池行（表头后第一数据行）的 READ/WRITE/CKSUM。
// 用「见过表头后的下一非空行即池行」定位，避免池名含 '-' 时被误判为子 vdev。
func parseConfigErrors(lines []string, st *ZFSScrubStatus) {
	inConfig := false
	seenHeader := false
	for _, line := range lines {
		c := strings.TrimSpace(line)
		if strings.HasPrefix(c, "config:") {
			inConfig = true
			seenHeader = false
			continue
		}
		if !inConfig || c == "" {
			continue
		}
		if !seenHeader {
			seenHeader = true // 表头行（NAME STATE READ WRITE CKSUM）
			continue
		}
		fs := strings.Fields(c) // 表头后第一数据行 = 池行
		if len(fs) >= 5 {
			st.ReadErr, _ = strconv.ParseInt(fs[2], 10, 64)
			st.WriteErr, _ = strconv.ParseInt(fs[3], 10, 64)
			st.ChecksumErr, _ = strconv.ParseInt(fs[4], 10, 64)
		}
		return
	}
}

// ── 纯函数：解析 zpool status -v 的永久错误文件清单 ──

var zfsPermErrHeaderRe = regexp.MustCompile(`(?i)Permanent errors have been detected in the following files`)

// parseZpoolErrors 从 `zpool status -v` 输出中提取永久错误文件路径列表。
// 定位 "Permanent errors ... following files:" 之后、到文件末尾的非空非缩进行；
// 跳过可能的 "errors:" 行。
func parseZpoolErrors(raw string) []string {
	lines := strings.Split(raw, "\n")
	var files []string
	collecting := false
	for _, line := range lines {
		c := strings.TrimSpace(line)
		if zfsPermErrHeaderRe.MatchString(c) {
			collecting = true
			continue
		}
		if !collecting {
			continue
		}
		if c == "" || strings.HasPrefix(c, "errors:") {
			continue
		}
		files = append(files, c)
	}
	return files
}

// ── 命令封装（exec wrapper；不单测，靠真机手测）──

// GetZFSScrubStatus 读取某 zpool 的 scrub 状态与健康度。
func GetZFSScrubStatus(pool string) (ZFSScrubStatus, error) {
	res := utils.ExecCommand("zpool", "status", pool)
	if res.Error != nil {
		return ZFSScrubStatus{}, fmt.Errorf("读取 zpool %s 状态失败: %w", pool, res.Error)
	}
	return parseZpoolStatus(res.Stdout, pool)
}

// StartZFSScrub 启动 scrub（已在 scrub 时 zpool 会报错，由前端按 state 禁用按钮兜底）。
func StartZFSScrub(pool string) error {
	if res := utils.ExecCommand("zpool", "scrub", pool); res.Error != nil {
		return fmt.Errorf("启动 scrub 失败: %w (%s)", res.Error, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// StopZFSScrub 停止正在进行的 scrub。
func StopZFSScrub(pool string) error {
	if res := utils.ExecCommand("zpool", "scrub", "-s", pool); res.Error != nil {
		return fmt.Errorf("停止 scrub 失败: %w (%s)", res.Error, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// ClearZFSErrors 清除瞬时错误计数（zpool clear；无错误时无害空操作）。
func ClearZFSErrors(pool string) error {
	if res := utils.ExecCommand("zpool", "clear", pool); res.Error != nil {
		return fmt.Errorf("清除错误失败: %w (%s)", res.Error, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// GetZFSErrors 读取永久错误文件清单（zpool status -v，截断前 200 条）。
// -v 在错误文件极多时可能卡顿，被 ExecCommand 的 30s 超时兜底；超时返回空 + TimedOut。
func GetZFSErrors(pool string) (ZFSErrorList, error) {
	res := utils.ExecCommand("zpool", "status", "-v", pool)
	out := ZFSErrorList{Pool: pool, Files: []string{}}
	if res.Error != nil {
		// 超时/异常：返回空清单并标注，不当硬错误（主状态端点不受影响）
		out.TimedOut = true
		return out, nil
	}
	files := parseZpoolErrors(res.Stdout)
	out.Total = len(files)
	if len(files) > zfsErrorsMaxFiles {
		out.Truncated = true
		out.Files = files[:zfsErrorsMaxFiles]
	} else {
		out.Files = files
	}
	return out, nil
}
