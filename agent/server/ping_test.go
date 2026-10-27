package server

import (
	"os"
	"testing"
	"time"
)

// requireOnlineTests 让依赖外网的用例**默认跳过**（2026-09-17，仓库体检）：
// 下面三个用例会 ping 硬编码的外部目标，离线或目标不可达时必然失败，
// 使整仓 `go test ./...` 在全离线机器上永远是红的。要跑它们显式打开：
//
//	KOMARI_AGENT_ONLINE_TESTS=1 go test ./server/...
func requireOnlineTests(t *testing.T) {
	t.Helper()
	if os.Getenv("KOMARI_AGENT_ONLINE_TESTS") != "1" {
		t.Skip("需要外网：设置 KOMARI_AGENT_ONLINE_TESTS=1 才运行（见 docs/MAINTAINING.md §7）")
	}
}

var testTargets = []struct {
	target string
}{
	{"v6-sh-cm.oojj.de"},
	{"2409:8c1e:8f80:2:6a::"},
	{"[2409:8c1e:8f80:2:6a::]"},
	{"[2409:8c1e:8f80:2:6a::]:80"},
	{"v4-sh-cm.oojj.de"},
	{"117.185.125.154"},
	{"117.185.125.154:80"},
}

func TestICMPPing(t *testing.T) {
	requireOnlineTests(t)
	timeout := 3 * time.Second
	for _, tt := range testTargets {
		t.Run(tt.target, func(t *testing.T) {
			latency, err := icmpPing(tt.target, timeout)
			if latency < -1 {
				t.Errorf("ICMP ping %s: invalid latency %d", tt.target, latency)
			}
			if err != nil {
				t.Errorf("ICMP ping %s error: %v", tt.target, err)
			}
		})
	}
}

func TestTCPPing(t *testing.T) {
	requireOnlineTests(t)
	timeout := 3 * time.Second
	for _, tt := range testTargets {
		t.Run(tt.target, func(t *testing.T) {
			latency, err := tcpPing(tt.target, timeout)
			if latency < -1 {
				t.Errorf("TCP ping %s: invalid latency %d", tt.target, latency)
			}
			if err != nil {
				t.Errorf("TCP ping %s error: %v", tt.target, err)
			}
		})
	}
}

func TestHTTPPing(t *testing.T) {
	requireOnlineTests(t)
	timeout := 3 * time.Second
	for _, tt := range testTargets {
		t.Run(tt.target, func(t *testing.T) {
			latency, err := httpPing(tt.target, timeout)
			if latency < -1 {
				t.Errorf("HTTP ping %s: invalid latency %d", tt.target, latency)
			}
			if err != nil {
				t.Errorf("HTTP ping %s error: %v", tt.target, err)
			}
		})
	}
}
