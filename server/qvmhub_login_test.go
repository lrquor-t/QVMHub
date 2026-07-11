package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

// TestMain 为整个 qvmhub 测试二进制做一次性初始化。
//
// 关键:全局 logger.App / config.GlobalConfig 只在此处赋值一次。各测试若反复 Init 会与
// 「WS 升级劫持连接后仍在途的请求 goroutine」(其在 RequestLoggerMiddleware 延迟日志里读
// logger.App)发生数据竞争。一次性初始化使运行期只剩读,无并发写 → 无竞争。
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	logDir, _ := os.MkdirTemp("", "qvmhub-test-logs-")
	logger.InitWithConsoleConfig(logDir, "info", 7, true, false, "", "info", 10, 3)

	os.Setenv("KVM_JWT_SECRET", "shared-test-secret")
	config.Init()

	code := m.Run()
	logger.Close()
	_ = os.RemoveAll(logDir)
	os.Exit(code)
}

func TestLoginDefaultAdmin(t *testing.T) {
	tmp := t.TempDir()
	// 直接覆盖 DB 路径(不重跑 config.Init,避免重置全局 logger/config)。
	config.GlobalConfig.DBPath = filepath.Join(tmp, "qvmhub-test.db")
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
