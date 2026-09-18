package install

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/internal/metricstore"
	"github.com/komari-monitor/komari/database/models"
	appconfig "github.com/komari-monitor/komari/internal/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupInstallRouter(t *testing.T) (*gin.Engine, *gorm.DB, *Controller) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+filepath.ToSlash(filepath.Join(t.TempDir(), "install.db"))+"?mode=rwc"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open install database: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &appconfig.ConfigItem{}); err != nil {
		t.Fatalf("migrate install database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get install sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	appconfig.SetDb(db)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	controller := NewController(db)
	controller.Activate()
	controller.Register(r)
	return r, db, controller
}

func performJSON(r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	encoded, _ := json.Marshal(body)
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	return response
}

// 拒绝的依据是**缺少监控库 DSN**（不是口令）：2026-09-18 起口令不再做任何强度/长度校验，
// 所以这条测试不再承担"弱口令被拒"的职责，只验证"坏输入不落库"。
func TestInstallRejectsMissingDSNWithoutCreatingAccount(t *testing.T) {
	r, db, _ := setupInstallRouter(t)
	response := performJSON(r, http.MethodPost, APIPath+"/complete", completeRequest{
		Username: "admin", Password: "short", Sitename: "Komari",
	})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid install status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("invalid install created users: count=%d err=%v", count, err)
	}
}

// 2026-09-18 契约变更之二（用户要求连长度也不校验）：3 个字符的口令现在**合法**。
func TestInstallAcceptsShortPassword(t *testing.T) {
	r, db, _ := setupInstallRouter(t)
	metricDSN := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "metrics.db")) + "?mode=rwc"
	response := performJSON(r, http.MethodPost, APIPath+"/complete", completeRequest{
		Username: "admin", Password: "abc", Sitename: "Komari", MetricDSN: metricDSN,
	})
	if response.Code != http.StatusOK {
		t.Fatalf("short password status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("install with short password should create exactly 1 admin: count=%d err=%v", count, err)
	}
}

// 2026-09-18 契约变更（用户明确要求去掉密码复杂度限制）：
// "≥8 位、只有小写与数字"的密码现在是**合法**的，安装应当成功并创建管理员。
// 变更前的断言断的是"这种口令必须被拒"，随规则删除一并替换。
func TestInstallAcceptsPasswordWithoutUppercase(t *testing.T) {
	r, db, _ := setupInstallRouter(t)
	metricDSN := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "metrics.db")) + "?mode=rwc"
	response := performJSON(r, http.MethodPost, APIPath+"/complete", completeRequest{
		Username: "admin", Password: "lowercaseonly1", Sitename: "Komari", MetricDSN: metricDSN,
	})
	if response.Code != http.StatusOK {
		t.Fatalf("password without uppercase status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("install with simple password should create exactly 1 admin: count=%d err=%v", count, err)
	}
}

func TestInstallCompletesAndPersistsSettings(t *testing.T) {
	r, db, _ := setupInstallRouter(t)
	metricDSN := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "metrics.db")) + "?mode=rwc"
	response := performJSON(r, http.MethodPost, APIPath+"/complete", completeRequest{
		Username:    "owner",
		Password:    "Correct-horse-battery-staple1",
		Sitename:    "My Komari",
		Description: "Private monitoring",
		MetricDSN:   metricDSN,
	})
	if response.Code != http.StatusOK {
		t.Fatalf("complete install status = %d: %s", response.Code, response.Body.String())
	}
	var user models.User
	if err := db.First(&user, "username = ?", "owner").Error; err != nil {
		t.Fatalf("find installed admin: %v", err)
	}
	want := map[string]any{
		appconfig.SitenameKey:         "My Komari",
		appconfig.DescriptionKey:      "Private monitoring",
		metricstore.MetricDBDriverKey: "sqlite",
		metricstore.MetricDBDSNKey:    metricDSN,
	}
	got, err := appconfig.GetAll()
	if err != nil {
		t.Fatalf("read all install settings: %v", err)
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("setting %s = %#v, want %#v", key, got[key], value)
		}
	}

	repeat := performJSON(r, http.MethodPost, APIPath+"/complete", completeRequest{
		Username: "other", Password: "Another-password1", Sitename: "Other", MetricDSN: "./data/metrics.db",
	})
	if repeat.Code != http.StatusConflict {
		t.Fatalf("repeat install status = %d, want %d", repeat.Code, http.StatusConflict)
	}
}

func TestInstallRejectsUnknownDSN(t *testing.T) {
	r, db, _ := setupInstallRouter(t)
	response := performJSON(r, http.MethodPost, APIPath+"/complete", completeRequest{
		Username: "admin", Password: "Strong-password1", Sitename: "Komari",
		MetricDSN: "not-a-recognized-dsn",
	})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown DSN status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("failed DSN created users: count=%d err=%v", count, err)
	}
}
