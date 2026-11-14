package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRetryMetricStoreConnectionStopsAfterRecovery(t *testing.T) {
	wantErr := errors.New("temporary connection failure")
	attempts := 0
	err := retryMetricStoreConnection(3, 0, func() error {
		attempts++
		if attempts < 3 {
			return wantErr
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retry returned error after recovery: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestGuideNoRouteRejectsUnknownAPIAndRedirectsPages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(noStoreAPIResponses())
	router.NoRoute(guideNoRoute("/install", "Not found in install mode", nil))

	apiResponse := httptest.NewRecorder()
	router.ServeHTTP(apiResponse, httptest.NewRequest(http.MethodGet, "/api/unknown", nil))
	if apiResponse.Code != http.StatusNotFound {
		t.Fatalf("API status = %d, want %d", apiResponse.Code, http.StatusNotFound)
	}
	if got := apiResponse.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("API cache control = %q, want no-store", got)
	}

	pageResponse := httptest.NewRecorder()
	router.ServeHTTP(pageResponse, httptest.NewRequest(http.MethodGet, "/stale-guide", nil))
	if pageResponse.Code != http.StatusTemporaryRedirect {
		t.Fatalf("page status = %d, want %d", pageResponse.Code, http.StatusTemporaryRedirect)
	}
	if got := pageResponse.Header().Get("Location"); got != "/install" {
		t.Fatalf("redirect location = %q, want /install", got)
	}
}

func TestRetryMetricStoreConnectionReturnsLastError(t *testing.T) {
	wantErr := errors.New("connection unavailable")
	attempts := 0
	err := retryMetricStoreConnection(3, 0, func() error {
		attempts++
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestShutdownReturnsCleanupError(t *testing.T) {
	wantErr := errors.New("queued reports were not flushed")
	app := New(Options{})
	app.addCleanup("metric-report-batcher", func(_ context.Context) error {
		return wantErr
	})

	err := app.Shutdown()
	if !errors.Is(err, wantErr) {
		t.Fatalf("shutdown error = %v, want %v", err, wantErr)
	}
}

// 全新实例会在安装前激活内置主题；安装向导必须只发内置前端，否则 /install
// 会被主题 SPA 接管（没有该路由），操作者看到 404 且 RPC 全失败。
func TestInstallGuideConfigRestrictsStaticFrontend(t *testing.T) {
	cfg := installGuideConfig()
	if !cfg.restrictedStatic {
		t.Fatal("install guide must restrict static assets to the embedded frontend")
	}
	if cfg.requireIdentity {
		t.Fatal("install guide must stay reachable without authentication")
	}
	if cfg.pagePath != "/install" {
		t.Fatalf("install guide page path = %q, want /install", cfg.pagePath)
	}
	if cfg.missingAPI != "Not found in install mode" {
		t.Fatalf("install guide API error = %q, want the install-mode message", cfg.missingAPI)
	}
}
