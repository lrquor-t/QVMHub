package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"qvmhub/model"
	"qvmhub/router"
	"qvmhub/service"
)

type recordedReq struct {
	method  string
	path    string
	headers http.Header
	body    string
}

func recordingStub(t *testing.T, status int, respBody string) (*httptest.Server, *recordedReq) {
	t.Helper()
	rec := &recordedReq{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.headers = r.Header.Clone()
		b, _ := io.ReadAll(r.Body)
		rec.body = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	return srv, rec
}

func doAuthedWithBody(t *testing.T, ts *httptest.Server, method, path, token, body string) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, ts.URL+path, reader)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// TestProxyPassthrough 是 P2 通用代理的端到端验收:
// JSON 透传、附节点 admin Key、剥离控制器 JWT、状态码原样回传,以及节点 404/禁用 502。
func TestProxyPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initTestStack(t)

	srv, rec := recordingStub(t, http.StatusOK, `{"code":200,"data":{"items":["vm-a","vm-b"]}}`)
	defer srv.Close()

	view, err := service.CreateHostNode(service.HostNodeRequest{
		Name: "proxy-node", APIBaseURL: srv.URL, APIKeyID: "kid", APIKey: "plaintext-admin-key",
	})
	require.NoError(t, err)
	nodeID := view.ID

	r := router.Setup()
	ts := httptest.NewServer(r)
	defer ts.Close()
	token := loginAsAdmin(t, ts)

	// GET 透传
	resp := doAuthedWithBody(t, ts, "GET", "/api/n/"+itoa(nodeID)+"/api/vm/list", token, "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Equal(t, float64(200), got["code"])

	require.Equal(t, "GET", rec.method)
	require.Equal(t, "/api/vm/list", rec.path)
	require.Equal(t, "kid", rec.headers.Get("X-API-Key-Id"))
	require.Equal(t, "plaintext-admin-key", rec.headers.Get("X-Api-Key"))
	require.Empty(t, rec.headers.Get("Authorization"), "控制器 JWT 不得转发给节点")

	// POST 透传(带 body)
	resp2 := doAuthedWithBody(t, ts, "POST", "/api/n/"+itoa(nodeID)+"/api/vm", token, `{"name":"new-vm"}`)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	require.Equal(t, "POST", rec.method)
	require.Equal(t, "/api/vm", rec.path)
	require.Contains(t, rec.body, "new-vm")

	// 节点返回非 2xx → 原样透传
	srvErr, recErr := recordingStub(t, http.StatusNotFound, `{"code":404,"message":"not found"}`)
	defer srvErr.Close()
	// 把节点的 base URL 换成返回 404 的桩。
	_, err = service.UpdateHostNode(nodeID, service.HostNodeRequest{
		Name: "proxy-node", APIBaseURL: srvErr.URL, APIKeyID: "kid", APIKey: "plaintext-admin-key",
	})
	require.NoError(t, err)
	resp3 := doAuthedWithBody(t, ts, "GET", "/api/n/"+itoa(nodeID)+"/api/vm/missing", token, "")
	defer resp3.Body.Close()
	require.Equal(t, http.StatusNotFound, resp3.StatusCode)
	_ = recErr

	// 节点不存在 → 404
	resp4 := doAuthedWithBody(t, ts, "GET", "/api/n/99999/api/vm/list", token, "")
	defer resp4.Body.Close()
	require.Equal(t, http.StatusNotFound, resp4.StatusCode)

	// 禁用节点 → 502
	require.NoError(t, model.DB.Model(&model.HostNode{}).Where("id = ?", nodeID).Update("enabled", false).Error)
	resp5 := doAuthedWithBody(t, ts, "GET", "/api/n/"+itoa(nodeID)+"/api/vm/list", token, "")
	defer resp5.Body.Close()
	require.Equal(t, http.StatusBadGateway, resp5.StatusCode)
}

func itoa(u uint) string { return strconv.FormatUint(uint64(u), 10) }
