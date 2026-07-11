package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsConsoleWSPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/api/vm/myvm/vnc/ws", true},
		{"/api/lxc/ct1/console/ws", true},
		{"/api/vm/myvm/vnc", false},       // 不以 /ws 结尾
		{"/api/vm/myvm/console/ws", false}, // vm 下只认 vnc/ws
		{"/api/host/stats", false},
		{"/api/vm/list", false},
		{"/api/lxc/ct1/vnc/ws", false},      // lxc 下只认 console/ws
		{"/api/lxc/ct1/terminal/ws", false}, // 节点无此路由,非控制台 WS
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, isConsoleWSPath(tc.path), "path=%s", tc.path)
	}
}

func TestToWSURL(t *testing.T) {
	require.Equal(t, "ws://1.2.3.4:9999/api/vm/a/vnc/ws", toWSURL("http://1.2.3.4:9999", "/api/vm/a/vnc/ws"))
	require.Equal(t, "wss://node.example/api/vm/a/vnc/ws", toWSURL("https://node.example/", "/api/vm/a/vnc/ws"))
}
