package clients

import (
	"testing"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
)

// 让每个用例用独立的 in-memory SQLite（与 database/tasks 的测试同套路）。
func setupTrafficTest(t *testing.T) {
	t.Helper()
	flags.DatabaseType = flags.DatabaseTypeSQLite
	flags.DatabaseFile = "file:traffic_total_" + t.Name() + "?mode=memory&cache=shared"
	if err := dbcore.GetDBInstance().AutoMigrate(&models.ClientTrafficTotal{}); err != nil {
		t.Fatalf("migrate client_traffic_totals: %v", err)
	}
	if err := InitTrafficTotals(); err != nil {
		t.Fatalf("init traffic totals: %v", err)
	}
}

func TestAccumulateTrafficSeedsBaselineOnFirstReport(t *testing.T) {
	setupTrafficTest(t)
	const uuid = "node-baseline"

	// 首次上报：以当前计数器为基线，本次增量不重复累加（否则会把开机以来的流量再加一遍）。
	if err := AccumulateTraffic(uuid, 1000, 500, 7, 3); err != nil {
		t.Fatalf("first accumulate: %v", err)
	}
	got, ok := GetTrafficTotal(uuid)
	if !ok || got.Up != 1000 || got.Down != 500 {
		t.Fatalf("baseline = %+v (ok=%v), want up=1000 down=500", got, ok)
	}

	// 之后每次上报累加增量。
	if err := AccumulateTraffic(uuid, 1100, 560, 100, 60); err != nil {
		t.Fatalf("second accumulate: %v", err)
	}
	if got, _ = GetTrafficTotal(uuid); got.Up != 1100 || got.Down != 560 {
		t.Fatalf("after second report = %+v, want up=1100 down=560", got)
	}
}

func TestAccumulateTrafficSurvivesCounterReset(t *testing.T) {
	setupTrafficTest(t)
	const uuid = "node-reboot"

	if err := AccumulateTraffic(uuid, 10_000, 20_000, 0, 0); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	// 机器重启：网卡计数器归零，metricstore 会把这次增量算成 0（重置感知），累计必须保持。
	if err := AccumulateTraffic(uuid, 0, 0, 0, 0); err != nil {
		t.Fatalf("after reboot: %v", err)
	}
	got, _ := GetTrafficTotal(uuid)
	if got.Up != 10_000 || got.Down != 20_000 {
		t.Fatalf("after reboot = %+v, want up=10000 down=20000（不能回退）", got)
	}
	// 重启后继续上报：累计继续增长。
	if err := AccumulateTraffic(uuid, 120, 240, 120, 240); err != nil {
		t.Fatalf("post reboot accumulate: %v", err)
	}
	got, _ = GetTrafficTotal(uuid)
	if got.Up != 10_120 || got.Down != 20_240 {
		t.Fatalf("after reboot accumulate = %+v, want up=10120 down=20240", got)
	}
}

func TestInitTrafficTotalsLoadsPersistedRows(t *testing.T) {
	setupTrafficTest(t)
	const uuid = "node-persist"

	if err := AccumulateTraffic(uuid, 1, 2, 0, 0); err != nil {
		t.Fatalf("accumulate: %v", err)
	}
	// 模拟服务器重启：清空内存缓存，再从库里加载。
	trafficMu.Lock()
	trafficTotals = map[string]TrafficTotal{}
	trafficMu.Unlock()
	if _, ok := GetTrafficTotal(uuid); ok {
		t.Fatal("cache should be empty before reload")
	}
	if err := InitTrafficTotals(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	got, ok := GetTrafficTotal(uuid)
	if !ok || got.Up != 1 || got.Down != 2 {
		t.Fatalf("after reload = %+v (ok=%v), want up=1 down=2", got, ok)
	}
}

func TestDeleteTrafficTotalRemovesRowAndCache(t *testing.T) {
	setupTrafficTest(t)
	const uuid = "node-delete"

	if err := AccumulateTraffic(uuid, 5, 6, 0, 0); err != nil {
		t.Fatalf("accumulate: %v", err)
	}
	if err := DeleteTrafficTotal(uuid); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := GetTrafficTotal(uuid); ok {
		t.Fatal("cache still has the deleted node")
	}
	var count int64
	if err := dbcore.GetDBInstance().Model(&models.ClientTrafficTotal{}).Where("uuid = ?", uuid).Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("rows = %d, want 0", count)
	}
}
