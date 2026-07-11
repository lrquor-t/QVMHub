package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"qvmhub/router"
	"qvmhub/service"
)

// sseStub 起一个 SSE 端点:分三次、间隔 60ms 推送 event 行,用以验证代理的分块刷新透传。
func sseStub(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		if fl != nil {
			fl.Flush()
		}
		for i := 1; i <= 3; i++ {
			fmt.Fprintf(w, "data: chunk-%d\n\n", i)
			if fl != nil {
				fl.Flush()
			}
			time.Sleep(60 * time.Millisecond)
		}
	}))
	return srv
}

// blobStub 起一个二进制端点(固定字节),用以验证 Blob 原样回传。
func blobStub(t *testing.T, payload []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(payload)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	return srv
}

// TestProxySSEStreamAndBlob 验证 P3:SSE 分块流式透传 + Blob 二进制原样回传。
func TestProxySSEStreamAndBlob(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initTestStack(t)

	// --- SSE ---
	sseSrv := sseStub(t)
	defer sseSrv.Close()
	view, err := service.CreateHostNode(service.HostNodeRequest{
		Name: "sse-node", APIBaseURL: sseSrv.URL, APIKeyID: "kid", APIKey: "k",
	})
	require.NoError(t, err)
	nodeID := view.ID

	r := router.Setup()
	ts := httptest.NewServer(r)
	defer ts.Close()
	token := loginAsAdmin(t, ts)

	resp := doAuthedWithBody(t, ts, "GET", "/api/n/"+itoa(nodeID)+"/api/host/stats/sse", token, "")
	defer resp.Body.Close()
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	// 逐行读取,应能收到全部 3 个 chunk(SSE 经代理透传,不被 45s 超时切断)。
	scanner := bufio.NewScanner(resp.Body)
	got := []string{}
	for scanner.Scan() {
		got = append(got, scanner.Text())
	}
	require.Len(t, got, 6, "3 个 data 行 + 3 个空行") // 每条事件两行:data: + 空行
	require.Contains(t, got[0], "chunk-1")
	require.Contains(t, got[2], "chunk-2")

	// --- Blob ---
	blob := []byte{0x1f, 0x8b, 0x00, 0xff, 0xab, 0xcd, 0xef, 0x01, 0x02, 0x03}
	blobSrv := blobStub(t, blob)
	defer blobSrv.Close()
	_, err = service.UpdateHostNode(nodeID, service.HostNodeRequest{
		Name: "sse-node", APIBaseURL: blobSrv.URL, APIKeyID: "kid", APIKey: "k",
	})
	require.NoError(t, err)

	resp2 := doAuthedWithBody(t, ts, "GET", "/api/n/"+itoa(nodeID)+"/api/logs/export", token, "")
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	body, err := io.ReadAll(resp2.Body)
	require.NoError(t, err)
	require.Equal(t, blob, body, "Blob 字节必须原样回传")
}
