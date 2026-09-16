package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDownloadMarketURLPrivateEndpointPolicy 固化我们 fork 的私网地址策略：
// 默认**允许**访问私网/内网地址（自托管场景需要，且默认主题市场源在部分网络下会被
// DNS 解析成私网地址或被污染，旧行为会 fail-closed 直接拦掉导致市场不可用）；
// 只有显式设置 KOMARI_BLOCK_PRIVATE_ENDPOINTS=1 时才恢复 SSRF 拦截。
func TestDownloadMarketURLPrivateEndpointPolicy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"version":1,"themes":[]}`))
	}))
	defer srv.Close()

	t.Run("默认允许私网地址", func(t *testing.T) {
		t.Setenv("KOMARI_BLOCK_PRIVATE_ENDPOINTS", "")
		body, err := downloadMarketURL(srv.URL, marketCatalogMaxSize)
		if err != nil {
			t.Fatalf("默认应允许 127.0.0.1，实际报错: %v", err)
		}
		if !strings.Contains(string(body), "version") {
			t.Fatalf("未取到内容: %q", body)
		}
	})

	t.Run("KOMARI_BLOCK_PRIVATE_ENDPOINTS=1 时拦截", func(t *testing.T) {
		t.Setenv("KOMARI_BLOCK_PRIVATE_ENDPOINTS", "1")
		_, err := downloadMarketURL(srv.URL, marketCatalogMaxSize)
		if err == nil || !strings.Contains(err.Error(), "private or internal") {
			t.Fatalf("开启开关后应拦截私网地址，实际: %v", err)
		}
	})
}

// TestBlockPrivateEndpointsDefault 明确默认值，避免以后被无意改回 fail-closed。
func TestBlockPrivateEndpointsDefault(t *testing.T) {
	t.Setenv("KOMARI_BLOCK_PRIVATE_ENDPOINTS", "")
	if blockPrivateEndpoints() {
		t.Fatal("默认不应开启私网地址拦截")
	}
	t.Setenv("KOMARI_BLOCK_PRIVATE_ENDPOINTS", "1")
	if !blockPrivateEndpoints() {
		t.Fatal("设置 KOMARI_BLOCK_PRIVATE_ENDPOINTS=1 后应开启拦截")
	}
}
