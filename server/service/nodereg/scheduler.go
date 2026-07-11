package nodereg

import (
	"context"
	"errors"
	"sync"
	"time"

	"qvmhub/model"
)

// errNoDB 表示尚未初始化数据库(调度器在 InitDB 之前被调用)。
var errNoDB = errors.New("database not initialized")

// 默认探活节奏(设计 §2 / §5.2):15s 探 version、60s 拉 stats。
const (
	defaultVersionEvery = 15 * time.Second
	defaultStatsEvery   = 60 * time.Second
	probeConcurrency    = 8
)

// Scheduler 周期性探活所有 enabled 节点,把结果写进 HealthCache 并回写 host_nodes。
type Scheduler struct {
	cache        *HealthCache
	versionEvery time.Duration
	statsEvery   time.Duration

	closeCh chan struct{}
	once    sync.Once
	wg      sync.WaitGroup
	startMu sync.Mutex
	started bool
}

// NewScheduler 用默认节奏构造调度器。
func NewScheduler(cache *HealthCache) *Scheduler {
	return &Scheduler{
		cache:        cache,
		versionEvery: defaultVersionEvery,
		statsEvery:   defaultStatsEvery,
		closeCh:      make(chan struct{}),
	}
}

// GlobalHealthCache / GlobalScheduler 是进程级单例,供 handler 与 main 复用。
var (
	GlobalHealthCache = NewHealthCache()
	GlobalScheduler   = NewScheduler(GlobalHealthCache)
)

// Start 启动两个后台 loop(version 15s / stats 60s);幂等。
func (s *Scheduler) Start(ctx context.Context) {
	s.startMu.Lock()
	if s.started {
		s.startMu.Unlock()
		return
	}
	s.started = true
	s.startMu.Unlock()

	s.wg.Add(2)
	go s.tickLoop(ctx, s.versionEvery, false)
	go s.tickLoop(ctx, s.statsEvery, true)
}

// Stop 停止调度并等待在途探活完成;幂等。
func (s *Scheduler) Stop() {
	s.once.Do(func() { close(s.closeCh) })
	s.wg.Wait()
}

func (s *Scheduler) tickLoop(ctx context.Context, interval time.Duration, withStats bool) {
	defer s.wg.Done()
	s.probeAll(ctx, withStats) // 启动后立即探一轮,不用等首个 tick
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.closeCh:
			return
		case <-t.C:
			s.probeAll(ctx, withStats)
		}
	}
}

// probeAll 读取所有节点:enabled 的并发探活;disabled 的只刷新缓存条目(不发包)。
func (s *Scheduler) probeAll(_ context.Context, withStats bool) {
	if model.DB == nil {
		return
	}
	var nodes []model.HostNode
	if err := model.DB.Find(&nodes).Error; err != nil {
		return
	}
	sem := make(chan struct{}, probeConcurrency)
	var wg sync.WaitGroup
	for i := range nodes {
		n := nodes[i]
		if !n.Enabled {
			s.cache.Set(disabledHealth(n))
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			s.RefreshNode(n, withStats)
		}()
	}
	wg.Wait()
}

// RefreshNode 立即探活单个节点(不论 enabled),写缓存 + 回写 host_nodes,返回最新健康。
func (s *Scheduler) RefreshNode(node model.HostNode, withStats bool) NodeHealth {
	res := Probe(node, withStats)
	h := resultToHealth(node, res)
	s.cache.Set(h)
	persistProbeResult(node.ID, res)
	return h
}

// RefreshNodeByID 按 ID 加载节点后探活;节点不存在返回错误。
func (s *Scheduler) RefreshNodeByID(id uint, withStats bool) (NodeHealth, error) {
	if model.DB == nil {
		return NodeHealth{}, errNoDB
	}
	var node model.HostNode
	if err := model.DB.First(&node, id).Error; err != nil {
		return NodeHealth{}, err
	}
	return s.RefreshNode(node, withStats), nil
}

func resultToHealth(node model.HostNode, res *ProbeResult) NodeHealth {
	h := NodeHealth{
		NodeID:     node.ID,
		Name:       node.Name,
		APIBaseURL: node.APIBaseURL,
		Enabled:    node.Enabled,
		Version:    res.Version,
		Stats:      res.Stats,
		LastError:  res.Error,
		UpdatedAt:  res.ProbedAt,
	}
	if res.Online {
		h.Online = true
		h.Status = StatusOnline
		seen := res.ProbedAt
		h.LastSeen = &seen
	} else {
		h.Status = StatusOffline
	}
	return h
}

func disabledHealth(node model.HostNode) NodeHealth {
	return NodeHealth{
		NodeID:     node.ID,
		Name:       node.Name,
		APIBaseURL: node.APIBaseURL,
		Enabled:    false,
		Status:     StatusDisabled,
	}
}

// persistProbeResult 把探活结论回写 host_nodes(状态/消息/时间)。探活失败不自动禁用(§7)。
func persistProbeResult(nodeID uint, res *ProbeResult) {
	if model.DB == nil {
		return
	}
	status := StatusOnline
	msg := "节点在线"
	if !res.Online {
		status = StatusOffline
		msg = res.Error
		if msg == "" {
			msg = "节点离线"
		}
	}
	model.DB.Model(&model.HostNode{}).Where("id = ?", nodeID).Updates(map[string]interface{}{
		"status":             status,
		"last_probe_message": msg,
		"last_probed_at":     &res.ProbedAt,
	})
}
