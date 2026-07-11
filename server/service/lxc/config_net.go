package lxc

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// nicLineRe 匹配 `lxc.net.<order>.<key> = <val>`。
var nicLineRe = regexp.MustCompile(`^\s*lxc\.net\.(\d+)\.([A-Za-z0-9_.]+)\s*=\s*(.*?)\s*$`)

// SplitNICBlocks 把 config 文本拆为 (other, blocks)。
// other：所有非 lxc.net.<n>.* 行原样保留（含空行/注释），末尾补一个换行。
// blocks：blocks[order][key] = val，重复键后者覆盖（与 LXC 后值覆盖语义一致）。
func SplitNICBlocks(text string) (string, map[int]map[string]string) {
	blocks := map[int]map[string]string{}
	var other []string
	for _, line := range strings.Split(text, "\n") {
		m := nicLineRe.FindStringSubmatch(line)
		if m == nil {
			other = append(other, line)
			continue
		}
		order, err := strconv.Atoi(m[1])
		if err != nil {
			other = append(other, line)
			continue
		}
		if blocks[order] == nil {
			blocks[order] = map[string]string{}
		}
		blocks[order][m[2]] = m[3]
	}
	// 去掉末尾空串（Split 产生），再保证单个换行结尾
	o := strings.Join(other, "\n")
	o = strings.TrimRight(o, "\n")
	if len(blocks) == 0 {
		return o, blocks
	}
	if o != "" {
		o += "\n"
	}
	return o, blocks
}

// RenderNICBlocks 按 order 升序、块内 key 升序渲染，末尾换行。
func RenderNICBlocks(blocks map[int]map[string]string) string {
	orders := make([]int, 0, len(blocks))
	for o := range blocks {
		orders = append(orders, o)
	}
	sort.Ints(orders)
	var b strings.Builder
	for _, o := range orders {
		keys := make([]string, 0, len(blocks[o]))
		for k := range blocks[o] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString("lxc.net.")
			b.WriteString(strconv.Itoa(o))
			b.WriteString(".")
			b.WriteString(k)
			b.WriteString(" = ")
			b.WriteString(blocks[o][k])
			b.WriteString("\n")
		}
	}
	return b.String()
}

// CompactNICBlocks 把 blocks 的 order 重排为从 0 起的连续整数（按原 order 升序映射）。
func CompactNICBlocks(blocks map[int]map[string]string) map[int]map[string]string {
	orders := make([]int, 0, len(blocks))
	for o := range blocks {
		orders = append(orders, o)
	}
	sort.Ints(orders)
	out := map[int]map[string]string{}
	for newIdx, old := range orders {
		out[newIdx] = blocks[old]
	}
	return out
}
