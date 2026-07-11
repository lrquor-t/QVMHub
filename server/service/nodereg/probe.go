package nodereg

import (
	"fmt"
	"time"

	"qvmhub/model"
	"qvmhub/service"
)

// ProbeResult 是对单节点一次探活的结果。
type ProbeResult struct {
	Online   bool
	Version  string
	Stats    *NodeStats
	Error    string // 非空表示有探活/拉取失败;Online 仍可能为 true(stats 拉取失败时)
	ProbedAt time.Time
}

// callNodeAPI 指向 service.CallNodeAPI;抽成变量便于单测注入桩。
var callNodeAPI = service.CallNodeAPI

// probeVersion 命中 /api/public/version(15s 节奏);CallNodeAPI 自动解信封并附 admin Key 头。
func probeVersion(node model.HostNode) (string, error) {
	var v struct {
		Version string `json:"version"`
	}
	if _, err := callNodeAPI(node, "GET", "/api/public/version", nil, &v); err != nil {
		return "", err
	}
	return v.Version, nil
}

// probeStats 命中 /api/host/stats(60s 节奏)。
func probeStats(node model.HostNode) (*NodeStats, error) {
	var s NodeStats
	if _, err := callNodeAPI(node, "GET", "/api/host/stats", nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Probe 对单个节点执行 version 探活,可选附带 stats 拉取。
// 版本探活失败 ⇒ Online=false;版本 OK 但 stats 失败 ⇒ Online=true 且 Error 记 warning。
func Probe(node model.HostNode, withStats bool) *ProbeResult {
	res := &ProbeResult{ProbedAt: time.Now()}
	ver, err := probeVersion(node)
	if err != nil {
		res.Error = fmt.Sprintf("version 探活失败: %v", err)
		return res
	}
	res.Online = true
	res.Version = ver
	if withStats {
		if stats, err := probeStats(node); err == nil {
			res.Stats = stats
		} else {
			res.Error = fmt.Sprintf("stats 拉取失败: %v", err)
		}
	}
	return res
}
