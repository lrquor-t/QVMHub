package nodereg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"qvmhub/config"
	"qvmhub/model"
	"qvmhub/service/host"
)

func init() {
	if config.GlobalConfig == nil {
		config.GlobalConfig = &config.Config{}
	}
}

func encKey(t *testing.T, plain string) string {
	t.Helper()
	enc, err := host.EncryptNodeSecret(plain)
	require.NoError(t, err)
	return enc
}

func nodePointingAt(t *testing.T, server *httptest.Server) model.HostNode {
	t.Helper()
	return model.HostNode{
		ID: 42, Name: "sched-node", APIBaseURL: server.URL,
		APIKeyID: "kid", APIKeyEnc: encKey(t, "x"), Enabled: true,
	}
}

// TestSchedulerRefreshNodeOnlineThenOffline 验证 RefreshNode 把探活结果正确写入缓存,
// 且节点从在线变为离线时缓存随之更新。model.DB 为 nil,persistProbeResult 安全跳过。
func TestSchedulerRefreshNodeOnlineThenOffline(t *testing.T) {
	cache := NewHealthCache()
	s := NewScheduler(cache)

	stats := &NodeStats{VMRunning: 2, VMTotal: 4, Hostname: "h"}
	srv := stubNodeServer(t, "v2.0", stats, http.StatusOK)

	// 在线
	h := s.RefreshNode(nodePointingAt(t, srv), true)
	require.True(t, h.Online)
	require.Equal(t, "v2.0", h.Version)
	require.NotNil(t, h.Stats)
	require.Equal(t, 2, h.Stats.VMRunning)

	cached, ok := cache.Get(42)
	require.True(t, ok)
	require.True(t, cached.Online)
	require.Equal(t, StatusOnline, cached.Status)

	// 离线:关掉 stub
	srv.Close()
	h = s.RefreshNode(nodePointingAt(t, srv), true)
	require.False(t, h.Online)
	require.Equal(t, StatusOffline, h.Status)

	cached, _ = cache.Get(42)
	require.False(t, cached.Online)
}

// TestSchedulerStartStopIdempotent 验证 Start 幂等、Stop 幂等且不阻塞。
func TestSchedulerStartStopIdempotent(t *testing.T) {
	s := NewScheduler(NewHealthCache())
	s.Start(context.Background())
	s.Start(context.Background()) // 二次启动无副作用
	s.Stop()
	s.Stop() // 二次停止无副作用、不 panic
}
