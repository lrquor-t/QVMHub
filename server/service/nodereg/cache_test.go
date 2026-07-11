package nodereg

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHealthCacheSetGetRemove(t *testing.T) {
	c := NewHealthCache()

	// 空缓存
	_, ok := c.Get(1)
	require.False(t, ok)
	require.Equal(t, 0, c.Len())
	require.Empty(t, c.Snapshot())

	// 写入
	c.Set(NodeHealth{NodeID: 1, Name: "n1", Online: true, Status: StatusOnline})
	h, ok := c.Get(1)
	require.True(t, ok)
	require.Equal(t, "n1", h.Name)
	require.True(t, h.Online)
	require.Equal(t, 1, c.Len())

	// 覆盖
	c.Set(NodeHealth{NodeID: 1, Name: "n1", Online: false, Status: StatusOffline})
	h, ok = c.Get(1)
	require.True(t, ok)
	require.False(t, h.Online)
	require.Equal(t, StatusOffline, h.Status)

	// 删除
	c.Remove(1)
	_, ok = c.Get(1)
	require.False(t, ok)
}

func TestHealthCacheSnapshotOrderedAndCopy(t *testing.T) {
	c := NewHealthCache()
	c.Set(NodeHealth{NodeID: 3, Name: "c"})
	c.Set(NodeHealth{NodeID: 1, Name: "a"})
	c.Set(NodeHealth{NodeID: 2, Name: "b"})

	snap := c.Snapshot()
	require.Len(t, snap, 3)
	require.Equal(t, []uint{1, 2, 3}, []uint{snap[0].NodeID, snap[1].NodeID, snap[2].NodeID})

	// Snapshot 是拷贝:改它不影响缓存。
	snap[0].Name = "mutated"
	h, _ := c.Get(1)
	require.Equal(t, "a", h.Name, "缓存不受 Snapshot 拷贝修改影响")
}

// TestHealthCacheConcurrent 并发读写不应 panic(-race 下验证)。
func TestHealthCacheConcurrent(t *testing.T) {
	c := NewHealthCache()
	const writers, iters = 4, 200
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id uint) {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				c.Set(NodeHealth{NodeID: id, Status: StatusOnline})
				_ = c.Snapshot()
				c.Get(id)
			}
		}(uint(w + 1))
	}
	wg.Wait()
	require.Equal(t, writers, c.Len())
}
