package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"qvmhub/config"
	"qvmhub/logger"
	"qvmhub/model"
	"qvmhub/router"
)

// TestMain 为所有 qvmhub 测试做一次性初始化(独立临时 DB,不污染开发库)。
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

func TestLoginDefaultAdmin(t *testing.T) {
	// 独立临时目录(SQLite + 日志),保证可复现且不污染开发库;
	// 用 t.TempDir() 而非硬编码 /tmp 路径,确保任意执行用户可写。
	tmp := t.TempDir()
	t.Setenv("KVM_DB_PATH", filepath.Join(tmp, "qvmhub-test.db"))
	t.Setenv("KVM_JWT_SECRET", "test-secret-do-not-use-in-prod")
	config.Init()
	logger.InitWithConsoleConfig(
		filepath.Join(tmp, "logs"), "info", 7, true, false, "", "info", 10, 3,
	)
	t.Cleanup(func() { logger.Close() })

	model.InitDB()

	// 默认 admin 密码来自 config.DefaultAdminPass(未知),重置为已知密码以便断言
	var admin model.User
	require.NoError(t, model.DB.Where("role = ?", "admin").First(&admin).Error)
	hash, err := bcrypt.GenerateFromPassword([]byte("TestPass-123"), bcrypt.DefaultCost)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&admin).
		Update("password_hash", string(hash)).Error)

	r := router.Setup()
	ts := httptest.NewServer(r)
	defer ts.Close()

	body := `{"username":"` + admin.Username + `","password":"TestPass-123"}`
	resp, err := http.Post(ts.URL+"/api/auth/login", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	// 解析 QVM 信封,断言拿到 token(任何登录分支都发 token)
	var got struct {
		Code int `json:"code"`
		Data struct {
			Token    string `json:"token"`
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Equal(t, 200, got.Code)
	require.NotEmpty(t, got.Data.Token, "登录必须返回 JWT")
	require.Equal(t, admin.Username, got.Data.Username)
	require.Equal(t, "admin", got.Data.Role)
}
