package nodereg

import (
	"sort"
	"sync"
)

// HealthCache 是线程安全的内存健康缓存:nodeID → NodeHealth。
// 总览页只读这个缓存,重启后下周期探活自动填满(设计 §5.2/§5.3)。
type HealthCache struct {
	mu sync.RWMutex
	m  map[uint]*NodeHealth
}

// NewHealthCache 创建空缓存。
func NewHealthCache() *HealthCache {
	return &HealthCache{m: make(map[uint]*NodeHealth)}
}

// Set 写入/覆盖某节点的健康快照(拷贝入参,调用方后续修改不影响缓存)。
func (c *HealthCache) Set(h NodeHealth) {
	c.mu.Lock()
	cp := h
	c.m[h.NodeID] = &cp
	c.mu.Unlock()
}

// Get 读取某节点快照;不存在返回 ok=false。
func (c *HealthCache) Get(id uint) (NodeHealth, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	h, ok := c.m[id]
	if !ok {
		return NodeHealth{}, false
	}
	return *h, true
}

// Remove 删除某节点缓存(节点被删除时调用)。
func (c *HealthCache) Remove(id uint) {
	c.mu.Lock()
	delete(c.m, id)
	c.mu.Unlock()
}

// Snapshot 返回所有节点快照的拷贝,按 NodeID 升序。
func (c *HealthCache) Snapshot() []NodeHealth {
	c.mu.RLock()
	out := make([]NodeHealth, 0, len(c.m))
	for _, h := range c.m {
		out = append(out, *h)
	}
	c.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].NodeID < out[j].NodeID })
	return out
}

// Len 返回缓存节点数。
func (c *HealthCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.m)
}
