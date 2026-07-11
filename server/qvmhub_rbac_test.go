package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"qvmhub/model"
	"qvmhub/router"
	"qvmhub/service"
)

// createNonAdmin 建一个已知密码的非 admin 用户(viewer / user),用于 RBAC 验证。
func createNonAdmin(t *testing.T, username, role string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("TestPass-123"), bcrypt.DefaultCost)
	require.NoError(t, err)
	u := model.User{
		Username:     username,
		Role:         role,
		Status:       "active",
		CloudType:    "elastic",
		PasswordHash: string(hash),
	}
	require.NoError(t, model.DB.Create(&u).Error)
}

// loginAs 按用户名重置密码(并清除强制改密标志)后登录,返回 token。
func loginAs(t *testing.T, ts *httptest.Server, username string) string {
	t.Helper()
	var u model.User
	require.NoError(t, model.DB.Where("username = ?", username).First(&u).Error)
	hash, err := bcrypt.GenerateFromPassword([]byte("TestPass-123"), bcrypt.DefaultCost)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&u).Updates(map[string]interface{}{
		"password_hash":         string(hash),
		"force_password_change": false,
	}).Error)
	body := `{"username":"` + username + `","password":"TestPass-123"}`
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

func adminUsername(t *testing.T) string {
	t.Helper()
	var admin model.User
	require.NoError(t, model.DB.Where("role = ?", "admin").First(&admin).Error)
	return admin.Username
}

// TestProxyRBACPolicy 验证 §5.5:admin 全权;非 admin 跨节点只读(GET 放行、POST 403、
// 控制台 WS 即便 GET 也 403)。
func TestProxyRBACPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initTestStack(t)

	srv, _ := recordingStub(t, http.StatusOK, `{"code":200,"data":[]}`)
	defer srv.Close()
	view, err := service.CreateHostNode(service.HostNodeRequest{
		Name: "rbac-node", APIBaseURL: srv.URL, APIKeyID: "kid", APIKey: "k",
	})
	require.NoError(t, err)
	nodeID := view.ID

	createNonAdmin(t, "viewer1", "viewer")

	r := router.Setup()
	ts := httptest.NewServer(r)
	defer ts.Close()
	adminToken := loginAs(t, ts, adminUsername(t))
	viewerToken := loginAs(t, ts, "viewer1")

	// 非 admin:GET 放行(透传到节点)。
	resp := doAuthedWithBody(t, ts, "GET", "/api/n/"+itoa(nodeID)+"/api/vm/list", viewerToken, "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// 非 admin:POST 被拦截。
	resp2 := doAuthedWithBody(t, ts, "POST", "/api/n/"+itoa(nodeID)+"/api/vm", viewerToken, `{"name":"x"}`)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusForbidden, resp2.StatusCode)

	// 非 admin:控制台 WS 路径即便 GET 也被拦截。
	resp3 := doAuthedWithBody(t, ts, "GET", "/api/n/"+itoa(nodeID)+"/api/vm/x/vnc/ws", viewerToken, "")
	defer resp3.Body.Close()
	require.Equal(t, http.StatusForbidden, resp3.StatusCode)

	// admin:POST 放行(策略不影响 admin)。
	resp4 := doAuthedWithBody(t, ts, "POST", "/api/n/"+itoa(nodeID)+"/api/vm", adminToken, `{"name":"y"}`)
	defer resp4.Body.Close()
	require.Equal(t, http.StatusOK, resp4.StatusCode)
}
