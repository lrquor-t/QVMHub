package pool

import (
	"fmt"
	"strconv"
	"strings"

	"qvmhub/utils"
)

// ZFSPropValue 单个属性当前值 + 来源（local / inherited from X / default）。
type ZFSPropValue struct {
	Value  string `json:"value"`
	Source string `json:"source"`
}

// ZFSPropertyInfo 编辑器读取结果。
type ZFSPropertyInfo struct {
	Dataset     string       `json:"dataset"`
	Compression ZFSPropValue `json:"compression"`
	Atime       ZFSPropValue `json:"atime"`
	Quota       ZFSPropValue `json:"quota"`
	Refquota    ZFSPropValue `json:"refquota"`
	PoolAvail   int64        `json:"pool_avail"`
}

// zfsEditableProps 可编辑属性白名单（防误设 mountpoint/canmount 等危险属性）。
var zfsEditableProps = map[string]bool{
	"compression": true, "atime": true, "quota": true, "refquota": true,
}

// parseZFSSize 把 ZFS 大小字符串（"50G"/"1024M"/"none"）解析为字节数。
// 合法→(bytes,true)；none→(0,true)；空/非法→(0,false)。base-1024。
// 与 ① 的 parseZFSBytes 不合并（那用于机器输出、须剥 /s；此用于用户表单值）。
func parseZFSSize(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if s == "none" {
		return 0, true
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
		return 0, false
	}
	f, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, false
	}
	mul := map[string]float64{
		"": 1, "B": 1,
		"K": 1024, "M": 1024 * 1024, "G": 1024 * 1024 * 1024,
		"T": 1024 * 1024 * 1024 * 1024, "P": 1024 * 1024 * 1024 * 1024 * 1024,
		"E": 1024 * 1024 * 1024 * 1024 * 1024 * 1024,
	}
	m, ok := mul[strings.ToUpper(unit)]
	if !ok {
		return 0, false
	}
	return int64(f * m), true
}

// validateZFSPropValue 校验属性名白名单 + 值合法（未导出，供 Set 内部 + Validate 复用）。
func validateZFSPropValue(prop, value string) error {
	if !zfsEditableProps[prop] {
		return fmt.Errorf("不允许修改属性: %s（仅支持 compression/atime/quota/refquota）", prop)
	}
	v := strings.TrimSpace(value)
	switch prop {
	case "compression":
		if v == "off" || v == "lz4" || v == "zstd" || v == "gzip" || v == "lzjb" || strings.HasPrefix(v, "gzip-") {
			return nil
		}
		return fmt.Errorf("压缩值非法: %s（支持 off/lz4/zstd/gzip/lzjb/gzip-N）", v)
	case "atime":
		if v == "on" || v == "off" {
			return nil
		}
		return fmt.Errorf("atime 值非法: %s（仅 on/off）", v)
	case "quota", "refquota":
		if _, ok := parseZFSSize(v); ok {
			return nil
		}
		return fmt.Errorf("%s 值非法: %s（如 50G / 1024M / none）", prop, v)
	}
	return nil
}

// ValidateZFSProperty 导出版（handler 用于区分 400 校验错 vs 500 命令错）。
func ValidateZFSProperty(prop, value string) error { return validateZFSPropValue(prop, value) }

// GetZFSProperties 读 dataset 的 4 属性 + 来源 + 池可用量。
func GetZFSProperties(dataset string) (ZFSPropertyInfo, error) {
	info := ZFSPropertyInfo{Dataset: dataset}
	res := utils.ExecCommand("zfs", "get", "-H", "-o", "property,value,source",
		"compression,atime,quota,refquota", dataset)
	if res.Error != nil {
		return info, fmt.Errorf("读取 zfs 属性失败: %w", res.Error)
	}
	for _, line := range strings.Split(res.Stdout, "\n") {
		f := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(f) < 3 {
			continue
		}
		pv := ZFSPropValue{Value: f[1], Source: f[2]}
		switch f[0] {
		case "compression":
			info.Compression = pv
		case "atime":
			info.Atime = pv
		case "quota":
			info.Quota = pv
		case "refquota":
			info.Refquota = pv
		}
	}
	// 池可用量：dataset 第一段即 pool 名
	poolName := dataset
	if idx := strings.Index(dataset, "/"); idx > 0 {
		poolName = dataset[:idx]
	}
	if r := utils.ExecCommand("zfs", "list", "-H", "-o", "avail", poolName); r.Error == nil {
		if bytes, ok := parseZFSSize(strings.TrimSpace(r.Stdout)); ok {
			info.PoolAvail = bytes
		}
	}
	return info, nil
}

// SetZFSProperty 设 dataset 单个属性（先白名单+值校验，再 zfs set）。
func SetZFSProperty(dataset, prop, value string) error {
	if err := validateZFSPropValue(prop, value); err != nil {
		return err
	}
	res := utils.ExecCommand("zfs", "set", prop+"="+strings.TrimSpace(value), dataset)
	if res.Error != nil {
		return fmt.Errorf("设置 %s 失败: %w (%s)", prop, res.Error, strings.TrimSpace(res.Stderr))
	}
	return nil
}
