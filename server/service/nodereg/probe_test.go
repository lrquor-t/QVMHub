package nodereg

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"qvmhub/config"
	"qvmhub/model"
	"qvmhub/service/host"
)

func init() {
	// BuildNodeSecretKey / EncryptNodeSecret 读 config.GlobalConfig;测试保证非 nil。
	if config.GlobalConfig == nil {
		config.GlobalConfig = &config.Config{}
	}
}

func encryptKey(t *testing.T, plain string) string {
	enc, err := host.EncryptNodeSecret(plain)
	require.NoError(t, err)
	return enc
}

// stubNodeServer 起一个最小 QVM 节点桩:只响应探活/统计两个端点,按 {code,message,data} 信封回传。
// status!=200 时两个端点都返回该状态码,模拟节点错误。
func stubNodeServer(t *testing.T, version string, stats *NodeStats, status int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	respond := func(w http.ResponseWriter, payload interface{}) {
		w.Header().Set("Content-Type", "application/json")
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 200, "data": payload})
	}
	mux.HandleFunc("/api/public/version", func(w http.ResponseWriter, r *http.Request) {
		respond(w, map[string]string{"version": version})
	})
	mux.HandleFunc("/api/host/stats", func(w http.ResponseWriter, r *http.Request) {
		respond(w, stats)
	})
	return httptest.NewServer(mux)
}

func stubNode(server *httptest.Server, keyEnc string) model.HostNode {
	return model.HostNode{
		ID:         7,
		Name:       "stub-node",
		APIBaseURL: server.URL,
		APIKeyID:   "kid",
		APIKeyEnc:  keyEnc,
	}
}

func TestProbeOnlineWithStats(t *testing.T) {
	stats := &NodeStats{CPUPercent: 12.5, VMRunning: 3, VMTotal: 5, Hostname: "h1"}
	srv := stubNodeServer(t, "v9.9.9", stats, http.StatusOK)
	defer srv.Close()

	res := Probe(stubNode(srv, encryptKey(t, "dummy-key")), true)
	require.True(t, res.Online)
	require.Equal(t, "v9.9.9", res.Version)
	require.Empty(t, res.Error)
	require.NotNil(t, res.Stats)
	require.Equal(t, 3, res.Stats.VMRunning)
	require.Equal(t, "h1", res.Stats.Hostname)
}

func TestProbeOnlineWithoutStats(t *testing.T) {
	srv := stubNodeServer(t, "v1.0.0", nil, http.StatusOK)
	defer srv.Close()

	res := Probe(stubNode(srv, encryptKey(t, "dummy-key")), false)
	require.True(t, res.Online)
	require.Equal(t, "v1.0.0", res.Version)
	require.Nil(t, res.Stats)
}

func TestProbeOfflineServerDown(t *testing.T) {
	srv := stubNodeServer(t, "v1.0.0", nil, http.StatusOK)
	srv.Close() // 立即关闭 → 连接被拒,模拟节点离线

	res := Probe(stubNode(srv, encryptKey(t, "dummy-key")), true)
	require.False(t, res.Online)
	require.NotEmpty(t, res.Error)
	require.Empty(t, res.Version)
}

func TestProbeOfflineNodeError(t *testing.T) {
	// 节点本身返回 500(如内部错误)。
	srv := stubNodeServer(t, "v1.0.0", nil, http.StatusInternalServerError)
	defer srv.Close()

	res := Probe(stubNode(srv, encryptKey(t, "dummy-key")), false)
	require.False(t, res.Online)
	require.NotEmpty(t, res.Error)
}
