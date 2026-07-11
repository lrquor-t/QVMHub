package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"qvmhub/config"
	"qvmhub/model"
	"qvmhub/router"
	"qvmhub/service"
	"qvmhub/service/nodereg"
)

// initTestStack 初始化一个独立临时 DB。全局 logger/config 已由 TestMain 一次性初始化,
// 这里只覆盖 DB 路径 + 跑迁移(避免重置全局 logger/config 与在途请求 goroutine 竞争)。
func initTestStack(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	config.GlobalConfig.DBPath = filepath.Join(tmp, "qvmhub.db")
	model.InitDB()
	nodereg.GlobalHealthCache.Reset() // 隔离:清掉上个测试残留的缓存
}

// loginAsAdmin 重置默认 admin 密码为已知值并登录,返回 "Bearer <token>" 与用户名。
func loginAsAdmin(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	var admin model.User
	require.NoError(t, model.DB.Where("role = ?", "admin").First(&admin).Error)
	hash, err := bcrypt.GenerateFromPassword([]byte("TestPass-123"), bcrypt.DefaultCost)
	require.NoError(t, err)
	// 同时清除强制改密标志,否则 ForcePasswordChangeMiddleware 会拦截 /api/overview。
	require.NoError(t, model.DB.Model(&admin).Updates(map[string]interface{}{
		"password_hash":         string(hash),
		"force_password_change": false,
	}).Error)

	body := `{"username":"` + admin.Username + `","password":"TestPass-123"}`
	resp, err := http.Post(ts.URL+"/api/auth/login", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.NotEmpty(t, got.Data.Token)
	return got.Data.Token
}

// stubNodeServer 最小 QVM 节点桩:按 {code,message,data} 信封回 version/stats。
func stubNodeServer(t *testing.T, online bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/public/version", func(w http.ResponseWriter, r *http.Request) {
		if !online {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200, "data": map[string]string{"version": "v3.1"},
		})
	})
	mux.HandleFunc("/api/host/stats", func(w http.ResponseWriter, r *http.Request) {
		if !online {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"data": map[string]interface{}{"vm_running": 7, "vm_total": 9, "hostname": "stub-host"},
		})
	})
	return httptest.NewServer(mux)
}

// registerStubNode 用真实 HTTP-only CRUD 注册一个指向 stub 的节点,返回其 ID。
func registerStubNode(t *testing.T, srv *httptest.Server) uint {
	t.Helper()
	view, err := service.CreateHostNode(service.HostNodeRequest{
		Name: "stub-node", APIBaseURL: srv.URL, APIKeyID: "kid", APIKey: "plaintext-admin-key",
	})
	require.NoError(t, err)
	require.NotEmpty(t, view.APIKeyPrefix, "列表应返回脱敏 Key 前缀而非明文")
	require.NotContains(t, view.APIKeyPrefix, "plaintext")
	return view.ID
}

func doAuthed(t *testing.T, ts *httptest.Server, method, path, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// TestOverviewReflectsNodeHealth 是 P1 的端到端验收:
// 注册 stub 节点 → 探活在线 → GET /api/overview 看到在线+stats;
// 节点变离线 → 重新探活 → 总览显示离线。
func TestOverviewReflectsNodeHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initTestStack(t)

	stub := stubNodeServer(t, true)
	defer stub.Close()

	nodeID := registerStubNode(t, stub)

	// 探活:在线(带 stats)。
	h, err := nodereg.GlobalScheduler.RefreshNodeByID(nodeID, true)
	require.NoError(t, err)
	require.True(t, h.Online)
	require.Equal(t, "v3.1", h.Version)
	require.NotNil(t, h.Stats)
	require.Equal(t, 7, h.Stats.VMRunning)

	// DB 侧状态被回写为 online。
	var dbNode model.HostNode
	require.NoError(t, model.DB.First(&dbNode, nodeID).Error)
	require.Equal(t, "online", dbNode.Status)

	r := router.Setup()
	ts := httptest.NewServer(r)
	defer ts.Close()
	token := loginAsAdmin(t, ts)

	// GET /api/overview:节点在线,统计命中。
	resp := doAuthed(t, ts, "GET", "/api/overview", token)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var ov struct {
		Data struct {
			Total  int `json:"total"`
			Online int `json:"online"`
			Nodes  []struct {
				NodeID  uint               `json:"node_id"`
				Online  bool               `json:"online"`
				Status  string             `json:"status"`
				Version string             `json:"version"`
				Stats   *nodereg.NodeStats `json:"stats"`
			} `json:"nodes"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&ov))
	require.Equal(t, 1, ov.Data.Total)
	require.Equal(t, 1, ov.Data.Online)
	require.Len(t, ov.Data.Nodes, 1)
	require.Equal(t, nodeID, ov.Data.Nodes[0].NodeID)
	require.True(t, ov.Data.Nodes[0].Online)
	require.Equal(t, "online", ov.Data.Nodes[0].Status)
	require.Equal(t, "v3.1", ov.Data.Nodes[0].Version)
	require.NotNil(t, ov.Data.Nodes[0].Stats)

	// 节点变离线:重起一个 offline stub 在同 URL 不可行(端口会变),改为直接探一个坏地址。
	stub.Close()
	time.Sleep(50 * time.Millisecond) // 确保端口释放/连接被拒
	_, err = nodereg.GlobalScheduler.RefreshNodeByID(nodeID, true)
	require.NoError(t, err) // 离线不是错误:返回 NodeHealth,Online=false

	resp2 := doAuthed(t, ts, "GET", "/api/overview", token)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	var ov2 struct {
		Data struct {
			Online int `json:"online"`
			Nodes  []struct {
				Online bool   `json:"online"`
				Status string `json:"status"`
			} `json:"nodes"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&ov2))
	require.Equal(t, 0, ov2.Data.Online, "节点离线后总览 online 计数应为 0")
	require.Len(t, ov2.Data.Nodes, 1)
	require.False(t, ov2.Data.Nodes[0].Online)
	require.Equal(t, "offline", ov2.Data.Nodes[0].Status)
}
