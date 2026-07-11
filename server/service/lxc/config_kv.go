package lxc

import (
	"os"
	"strings"
)

// 本文件提供 LXC 类 config（`key = value`，也兼容 `key value` 空格分隔）的「原地改写」工具。
// 设计动机：直接追加 key 行会在继承自模板/源的 config 上留下重复键（例如模板的
// lxc.net.0.link = lxcbr0 与追加的 lxc.net.0.link = br-ovs 并存）。LXC 虽 last-wins，
// 但重复键令 config 脏、易误导。原地改写可去重并保证唯一。

// ConfigKV 是一个待改写/追加的 config 键值对。
type ConfigKV struct {
	Key string
	Val string
}

// parseConfigKV 解析一行 `key<sep>value`，sep 为 "=" 或空白。
// 返回 (key, sep)；sep 为 "=" / " "，注释、空行、无分隔符的行 sep 为 ""。
func parseConfigKV(line string) (key, sep string) {
	t := strings.TrimSpace(line)
	if t == "" || strings.HasPrefix(t, "#") {
		return "", ""
	}
	if i := strings.Index(t, "="); i > 0 {
		return strings.TrimSpace(t[:i]), "="
	}
	if i := strings.IndexAny(t, " \t"); i > 0 {
		return strings.TrimSpace(t[:i]), " "
	}
	return "", ""
}

// SetConfigKeysText 在 config 文本上原地改写 pairs：
//   - 已存在的键改写其值（多次出现只保留首次位置并改写、其余丢弃，即去重）；
//   - 不存在的键按 pairs 顺序追加到末尾。
//
// 分隔符沿用原行（"=" 写成 " = "、空白写成 " "），追加的统一用 " = "。纯函数，便于复用。
func SetConfigKeysText(text string, pairs []ConfigKV) string {
	want := map[string]string{}
	for _, p := range pairs {
		want[p.Key] = p.Val
	}
	written := map[string]bool{}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		key, sep := parseConfigKV(line)
		if sep != "" {
			if val, ok := want[key]; ok {
				if !written[key] {
					joiner := " = "
					if sep == " " {
						joiner = " "
					}
					out = append(out, key+joiner+val)
					written[key] = true
				}
				continue // 丢弃源值及后续重复
			}
		}
		out = append(out, line)
	}
	// 缺失的键按 pairs 顺序追加（确定序）
	for _, p := range pairs {
		if !written[p.Key] {
			out = append(out, p.Key+" = "+p.Val)
			written[p.Key] = true
		}
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}

// SetConfigKeys 读 cfgPath、原地改写/追加 pairs 后写回（单一 os.WriteFile，非追加打开）。
func SetConfigKeys(cfgPath string, pairs []ConfigKV) error {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, []byte(SetConfigKeysText(string(data), pairs)), 0644)
}
