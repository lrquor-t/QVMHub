package host

import (
	"testing"

	"github.com/stretchr/testify/require"

	"qvmhub/config"
	"qvmhub/model"
)

// ensureGlobalConfig 保证 BuildNodeSecretKey(读 config.GlobalConfig)不因 nil 指针 panic。
func ensureGlobalConfig() {
	if config.GlobalConfig == nil {
		config.GlobalConfig = &config.Config{}
	}
}

func TestMaskAPIKey(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"short fully masked", "abc", "***"},
		{"boundary 8 fully masked", "12345678", "********"},
		{"long masked prefix+suffix", "abcd-efgh-ijkl-wxyz", "abcd****wxyz"},
		{"trim spaces", "  abcd1234efgh5678  ", "abcd****5678"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, maskAPIKey(tc.in))
		})
	}
}

// TestBuildHostNodeFromRequestHTTPOnly 验证 QVMHub 的 HTTP-only 加入流程:
// 不提供任何 SSH 字段也应能构造节点(SSH 仅模型保留,§5.1)。
func TestBuildHostNodeFromRequestHTTPOnly(t *testing.T) {
	ensureGlobalConfig()
	enabled := true
	node, err := buildHostNodeFromRequest(model.HostNode{}, HostNodeRequest{
		Name:       "node-1",
		APIBaseURL: "http://10.0.0.1:9999",
		APIKeyID:   "kid-1",
		APIKey:     "super-secret-key-1234",
		Enabled:    &enabled,
	}, true)
	require.NoError(t, err)
	require.Equal(t, "node-1", node.Name)
	require.Equal(t, "http://10.0.0.1:9999", node.APIBaseURL)
	require.Equal(t, "kid-1", node.APIKeyID)
	require.NotEmpty(t, node.APIKeyEnc, "API Key 必须加密落库")
	require.Empty(t, node.SSHHost, "SSH 字段应为空(可选)")
	require.Empty(t, node.SSHPasswordEnc)
	require.True(t, node.Enabled)
}

// TestBuildHostNodeFromRequestMissingAPIFields 验证 HTTP 必填项缺失时报错。
func TestBuildHostNodeFromRequestMissingAPIFields(t *testing.T) {
	ensureGlobalConfig()
	_, err := buildHostNodeFromRequest(model.HostNode{}, HostNodeRequest{
		Name: "node-1", APIBaseURL: "http://10.0.0.1:9999",
		// 缺 APIKeyID
	}, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "API Key ID")

	// 创建时缺 API Key 本体
	_, err = buildHostNodeFromRequest(model.HostNode{}, HostNodeRequest{
		Name: "node-1", APIBaseURL: "http://10.0.0.1:9999", APIKeyID: "kid-1",
	}, true)
	require.Error(t, err)
}
