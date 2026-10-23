package clients

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
)

// P2 周期累加用例（design.md §7 第三到第六行）。全部用独立 in-memory SQLite，
// 不碰生产库；时间断言基于"当天"现算出来的边界，因此不依赖真实运行时刻。

// setupPeriodTrafficTest 建独立 in-memory 库（含 clients 与 client_traffic_totals
// 两张表与审计日志表），并按需写入一条节点配置。返回库句柄与节点 UUID。
func setupPeriodTrafficTest(t *testing.T, resetDay int, tzName string) string {
	t.Helper()
	suffix := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	flags.DatabaseType = flags.DatabaseTypeSQLite
	flags.DatabaseFile = "file:period_" + suffix + "?mode=memory&cache=shared"
	db := dbcore.GetDBInstance()
	if err := db.AutoMigrate(&models.ClientTrafficTotal{}, &models.Client{}, &models.Log{}); err != nil {
		t.Fatalf("migrate test tables: %v", err)
	}
	if err := InitTrafficTotals(); err != nil {
		t.Fatalf("init traffic totals: %v", err)
	}

	uuid := "node-" + suffix
	client := models.Client{UUID: uuid, Token: "token-" + suffix, Name: "period-test",
		TrafficResetDay: resetDay, TrafficResetTimezone: tzName}
	if err := db.Create(&client).Error; err != nil {
		t.Fatalf("create client config: %v", err)
	}
	return uuid
}

// seedTrafficRow 直接写入累计行，用来构造"已经处于某个周期中"的现场。
func seedTrafficRow(t *testing.T, uuid string, up, down, periodUp, periodDown int64, periodStart time.Time) {
	t.Helper()
	start := periodStart.UTC()
	row := models.ClientTrafficTotal{
		UUID:            uuid,
		UpTotal:         up,
		DownTotal:       down,
		PeriodUp:        periodUp,
		PeriodDown:      periodDown,
		PeriodStartedAt: &start,
	}
	if err := dbcore.GetDBInstance().Create(&row).Error; err != nil {
		t.Fatalf("seed traffic row: %v", err)
	}
	if err := InitTrafficTotals(); err != nil {
		t.Fatalf("reload traffic totals: %v", err)
	}
}

// mustTrafficRow 读内存里的累计/周期状态，并顺带断言"内存 == 库"。
func mustTrafficRow(t *testing.T, uuid string) TrafficTotal {
	t.Helper()
	got, ok := GetTrafficTotal(uuid)
	if !ok {
		t.Fatalf("traffic total for %s missing from cache", uuid)
	}
	// 内存与库必须一致：RPC 读内存，重启后读库，两边不一致会出现"重启前后数字不同"。
	var row models.ClientTrafficTotal
	if err := dbcore.GetDBInstance().Where("uuid = ?", uuid).First(&row).Error; err != nil {
		t.Fatalf("load persisted row: %v", err)
	}
	if row.UpTotal != got.Up || row.DownTotal != got.Down {
		t.Fatalf("cache/persisted mismatch: cache=(%d,%d) db=(%d,%d)", got.Up, got.Down, row.UpTotal, row.DownTotal)
	}
	if row.PeriodUp != got.PeriodUp || row.PeriodDown != got.PeriodDown {
		t.Fatalf("period cache/persisted mismatch: cache=(%d,%d) db=(%d,%d)",
			got.PeriodUp, got.PeriodDown, row.PeriodUp, row.PeriodDown)
	}
	// 周期起点也必须一致：库里存 NULL ⇔ 内存是零值。
	// （曾经出现过"内存判定为零值 ⇒ 不落库 ⇒ 下次上报读回 NULL 又当成零值"的绕圈，
	// 这条断言能钉住内存与库的起点语义一致。）
	switch {
	case row.PeriodStartedAt == nil && !got.PeriodStart.IsZero():
		t.Fatalf("cache has period start %s but persisted row has NULL", got.PeriodStart)
	case row.PeriodStartedAt != nil && got.PeriodStart.IsZero():
		t.Fatalf("persisted period start %s but cache has zero value", row.PeriodStartedAt)
	case row.PeriodStartedAt != nil && !row.PeriodStartedAt.UTC().Equal(got.PeriodStart):
		t.Fatalf("period start mismatch: cache=%s db=%s", got.PeriodStart, row.PeriodStartedAt.UTC())
	}
	return got
}

// previousBoundary 返回严格早于 ref 的最近一个周期边界：
// 取 ref 之前的某一时刻，取其所在周期的起点，直到该起点严格早于 ref 为止。
// 用与实现同源的 PeriodWindow 反推，避免测试重写一套日期算法而与实现产生分歧。
func previousBoundary(t *testing.T, ref time.Time, resetDay int) time.Time {
	t.Helper()
	candidate := ref.AddDate(0, -1, 0)
	for i := 0; i < 4; i++ {
		start, _, err := PeriodWindow(candidate, resetDay, "UTC")
		if err != nil {
			t.Fatalf("PeriodWindow(previous): %v", err)
		}
		if start.Before(ref) {
			return start
		}
		candidate = candidate.AddDate(0, -1, 0)
	}
	t.Fatalf("could not find a boundary before %s (resetDay=%d)", ref, resetDay)
	return time.Time{}
}

// 跨周期边界：周期归零、起点前移，累计不回退（design.md §7 第三行）。
func TestAccumulateTrafficResetsPeriodOnBoundary(t *testing.T) {
	const resetDay = 5
	uuid := setupPeriodTrafficTest(t, resetDay, "UTC")

	now := time.Now().UTC()
	// 造一个"周期已经走完"的现场：周期起点取 now 所处周期的**上一个**边界。
	// 这样已记录起点 < PeriodWindow(now).start ⇒ 本次上报必然跨边界。
	start := previousBoundary(t, now, resetDay)
	expectedStart, _, err := PeriodWindow(now, resetDay, "UTC")
	if err != nil {
		t.Fatalf("PeriodWindow: %v", err)
	}
	if !start.Before(expectedStart) {
		t.Fatalf("test setup assumption broken: previous boundary %s not before %s", start, expectedStart)
	}
	seedTrafficRow(t, uuid, 1_000_000, 2_000_000, 300_000, 400_000, start)

	if err := AccumulateTraffic(uuid, 1_000_000+100, 2_000_000+200, 100, 200); err != nil {
		t.Fatalf("accumulate across boundary: %v", err)
	}

	got := mustTrafficRow(t, uuid)
	if got.Up != 1_000_100 || got.Down != 2_000_200 {
		t.Fatalf("cumulative = (%d,%d), want (1000100,2000200)（不能回退）", got.Up, got.Down)
	}
	if got.PeriodUp != 100 || got.PeriodDown != 200 {
		t.Fatalf("period = (%d,%d), want (100,200)（跨边界后归零再累本次增量）", got.PeriodUp, got.PeriodDown)
	}
	// 归零后的起点必须是"最近一个边界"（现在所处的周期边界），而不是 now。
	if !got.PeriodStart.Equal(expectedStart.UTC()) {
		t.Fatalf("period start = %s, want boundary %s", got.PeriodStart, expectedStart.UTC())
	}
	if !got.PeriodStart.Before(now) && !got.PeriodStart.Equal(now) {
		t.Fatalf("period start %s should not be in the future", got.PeriodStart)
	}
}

// 未跨边界：周期与累计都继续增长，起点不变（design.md §7 第三行的对照）。
func TestAccumulateTrafficKeepsPeriodWithinWindow(t *testing.T) {
	const resetDay = 5
	uuid := setupPeriodTrafficTest(t, resetDay, "UTC")

	now := time.Now().UTC()
	start, next, err := PeriodWindow(now, resetDay, "UTC")
	if err != nil {
		t.Fatalf("PeriodWindow: %v", err)
	}
	if next.Before(now) {
		t.Fatalf("test setup assumption broken: next=%s now=%s", next, now)
	}
	seedTrafficRow(t, uuid, 1_000, 2_000, 300, 400, start)

	if err := AccumulateTraffic(uuid, 1_100, 2_200, 100, 200); err != nil {
		t.Fatalf("accumulate within window: %v", err)
	}

	got := mustTrafficRow(t, uuid)
	if got.Up != 1_100 || got.Down != 2_200 || got.PeriodUp != 400 || got.PeriodDown != 600 {
		t.Fatalf("got %+v, want up=1100 down=2200 period=(400,600)", got)
	}
	if !got.PeriodStart.Equal(start.UTC()) {
		t.Fatalf("period start = %s, want unchanged %s", got.PeriodStart, start.UTC())
	}
}

// 计数器归零（模拟 agent/机器重启）：周期**不**归零、累计**不**回退
// ——这是"两种归零可区分"的验收项（design.md §7 第四行）。
func TestAccumulateTrafficCounterResetKeepsPeriod(t *testing.T) {
	const resetDay = 5
	uuid := setupPeriodTrafficTest(t, resetDay, "UTC")

	now := time.Now().UTC()
	start, _, err := PeriodWindow(now, resetDay, "UTC")
	if err != nil {
		t.Fatalf("PeriodWindow: %v", err)
	}
	seedTrafficRow(t, uuid, 10_000, 20_000, 1_000, 2_000, start)

	// 机器重启：网卡计数器归零，metricstore 把这次增量算成 0（重置感知）。
	if err := AccumulateTraffic(uuid, 0, 0, 0, 0); err != nil {
		t.Fatalf("accumulate after counter reset: %v", err)
	}
	got := mustTrafficRow(t, uuid)
	if got.Up != 10_000 || got.Down != 20_000 {
		t.Fatalf("cumulative = (%d,%d), want (10000,20000)", got.Up, got.Down)
	}
	if got.PeriodUp != 1_000 || got.PeriodDown != 2_000 {
		t.Fatalf("period = (%d,%d), want unchanged (1000,2000)（重启不是周期边界）", got.PeriodUp, got.PeriodDown)
	}
	if !got.PeriodStart.Equal(start.UTC()) {
		t.Fatalf("period start = %s, want unchanged %s", got.PeriodStart, start.UTC())
	}

	// 重启后继续上报：累计与周期一起增长。
	if err := AccumulateTraffic(uuid, 120, 240, 120, 240); err != nil {
		t.Fatalf("accumulate after reboot: %v", err)
	}
	got = mustTrafficRow(t, uuid)
	if got.Up != 10_120 || got.Down != 20_240 || got.PeriodUp != 1_120 || got.PeriodDown != 2_240 {
		t.Fatalf("got %+v, want up=10120 down=20240 period=(1120,2240)", got)
	}
}

// 再基线：口径切换后首次上报只更新基线，累计不跳变（design.md §7 第五行）。
func TestAccumulateTrafficRebaselineSwallowsOneReport(t *testing.T) {
	const resetDay = 5
	uuid := setupPeriodTrafficTest(t, resetDay, "UTC")

	now := time.Now().UTC()
	start, _, err := PeriodWindow(now, resetDay, "UTC")
	if err != nil {
		t.Fatalf("PeriodWindow: %v", err)
	}
	seedTrafficRow(t, uuid, 1_000_000, 2_000_000, 200_000, 300_000, start)

	if err := MarkTrafficRebaseline(uuid); err != nil {
		t.Fatalf("mark rebaseline: %v", err)
	}
	if !mustTrafficRow(t, uuid).NeedsRebaseline {
		t.Fatal("NeedsRebaseline not set after MarkTrafficRebaseline")
	}

	// 口径切换后的首次上报：上报值从"周期值"跳到"开机计数器"（这里模拟成大幅变大）。
	if err := AccumulateTraffic(uuid, 9_999_999, 8_888_888, 8_999_999, 6_888_888); err != nil {
		t.Fatalf("accumulate after rebaseline: %v", err)
	}

	got := mustTrafficRow(t, uuid)
	if got.Up != 1_000_000 || got.Down != 2_000_000 {
		t.Fatalf("cumulative = (%d,%d), want unchanged (1000000,2000000)（跳变必须被吞掉）", got.Up, got.Down)
	}
	if got.PeriodUp != 200_000 || got.PeriodDown != 300_000 {
		t.Fatalf("period = (%d,%d), want unchanged (200000,300000)", got.PeriodUp, got.PeriodDown)
	}
	if got.NeedsRebaseline {
		t.Fatal("NeedsRebaseline should be cleared after the first report")
	}

	// 再基线只吞掉一次：下一次上报恢复正常累加。
	if err := AccumulateTraffic(uuid, 9_999_999+50, 8_888_888+60, 50, 60); err != nil {
		t.Fatalf("accumulate after rebaseline consumed: %v", err)
	}
	got = mustTrafficRow(t, uuid)
	if got.Up != 1_000_050 || got.Down != 2_000_060 || got.PeriodUp != 200_050 || got.PeriodDown != 300_060 {
		t.Fatalf("got %+v, want up=1000050 down=2000060 period=(200050,300060)", got)
	}
}

// 禁用配置回归：resetDay = 0 时周期字段不动，行为与引入周期概念之前完全一致
// （design.md §7 第六行）。
func TestAccumulateTrafficDisabledTimeoutKeepsLegacyBehavior(t *testing.T) {
	uuid := setupPeriodTrafficTest(t, 0, "")

	if err := AccumulateTraffic(uuid, 1_000, 500, 7, 3); err != nil {
		t.Fatalf("first accumulate: %v", err)
	}
	got := mustTrafficRow(t, uuid)
	if got.Up != 1_000 || got.Down != 500 {
		t.Fatalf("baseline = %+v, want up=1000 down=500", got)
	}
	if got.PeriodUp != 0 || got.PeriodDown != 0 || !got.PeriodStart.IsZero() {
		t.Fatalf("period fields must stay empty when disabled, got %+v", got)
	}

	if err := AccumulateTraffic(uuid, 1_100, 560, 100, 60); err != nil {
		t.Fatalf("second accumulate: %v", err)
	}
	got = mustTrafficRow(t, uuid)
	if got.Up != 1_100 || got.Down != 560 {
		t.Fatalf("after second report = %+v, want up=1100 down=560", got)
	}
	if got.PeriodUp != 0 || got.PeriodDown != 0 || !got.PeriodStart.IsZero() {
		t.Fatalf("period fields must stay empty when disabled, got %+v", got)
	}

	// 禁用状态下即使被标记再基线，也只是清标志，不影响累计。
	if err := MarkTrafficRebaseline(uuid); err != nil {
		t.Fatalf("mark rebaseline: %v", err)
	}
	if err := AccumulateTraffic(uuid, 1_300, 700, 200, 140); err != nil {
		t.Fatalf("accumulate after rebaseline: %v", err)
	}
	got = mustTrafficRow(t, uuid)
	if got.Up != 1_100 || got.Down != 560 || got.NeedsRebaseline {
		t.Fatalf("got %+v, want cumulative unchanged and flag cleared", got)
	}

	// 库里也必须保持"禁用 ⇒ 周期列为空"（内存与库一致，重启用时不会读到旧周期）。
	var row models.ClientTrafficTotal
	if err := dbcore.GetDBInstance().Where("uuid = ?", uuid).First(&row).Error; err != nil {
		t.Fatalf("load persisted row: %v", err)
	}
	if row.PeriodUp != 0 || row.PeriodDown != 0 || row.PeriodStartedAt != nil {
		t.Fatalf("disabled node must keep empty period columns, got %+v", row)
	}
}

// 首次上报（建行）时周期从 max(边界, 现在) 起算，本次增量不计入周期
// ——与累计的基线语义一致（design.md §4）。
func TestAccumulateTrafficFirstReportSetsPeriodBaseline(t *testing.T) {
	const resetDay = 5
	t.Run("boundary in the past", func(t *testing.T) {
		uuid := setupPeriodTrafficTest(t, resetDay, "UTC")
		now := time.Now().UTC()
		boundary, next, err := PeriodWindow(now, resetDay, "UTC")
		if err != nil {
			t.Fatalf("PeriodWindow: %v", err)
		}
		if !boundary.Before(now) || !next.After(now) {
			t.Skipf("requires the current period to have started strictly before now (boundary=%s now=%s)", boundary, now)
		}
		if err := AccumulateTraffic(uuid, 5_000, 6_000, 11, 22); err != nil {
			t.Fatalf("first accumulate: %v", err)
		}
		got := mustTrafficRow(t, uuid)
		if got.Up != 5_000 || got.Down != 6_000 || got.PeriodUp != 0 || got.PeriodDown != 0 {
			t.Fatalf("got %+v, want baseline only (period 0)", got)
		}
		// 规约是 max(边界, 现在)：边界已过去 ⇒ 取"现在"，启用前的流量不算进周期。
		if got.PeriodStart.Before(now.Add(-2*time.Second)) || got.PeriodStart.After(now.Add(2*time.Second)) {
			t.Fatalf("period start = %s, want ~now %s (max(boundary=%s, now))", got.PeriodStart, now, boundary)
		}
	})

	t.Run("boundary in the future", func(t *testing.T) {
		// resetDay = 31 且当前月没有 31 日时，"本月边界"是顺延出来的次月 1 日（在本月里看是未来），
		// 此时周期起点取 max(边界, now) = 边界本身——周期还没开始，先记边界。
		uuid := setupPeriodTrafficTest(t, 31, "UTC")
		now := time.Now().UTC()
		start, _, err := PeriodWindow(now, 31, "UTC")
		if err != nil {
			t.Fatalf("PeriodWindow: %v", err)
		}
		if !start.After(now) {
			t.Skipf("only meaningful when the deferral boundary is in the future (start=%s now=%s)", start, now)
		}
		if err := AccumulateTraffic(uuid, 5_000, 6_000, 11, 22); err != nil {
			t.Fatalf("first accumulate: %v", err)
		}
		got := mustTrafficRow(t, uuid)
		if !got.PeriodStart.Equal(start.UTC()) {
			t.Fatalf("period start = %s, want max(boundary,now) = %s", got.PeriodStart, start.UTC())
		}
		if got.PeriodUp != 0 || got.PeriodDown != 0 {
			t.Fatalf("period = (%d,%d), want 0", got.PeriodUp, got.PeriodDown)
		}
	})
}

// 从"启用"切到"禁用"后，周期列被清空，累计继续增长（换配置不丢累计）。
func TestAccumulateTrafficClearsPeriodWhenDisabled(t *testing.T) {
	const resetDay = 5
	uuid := setupPeriodTrafficTest(t, resetDay, "UTC")

	now := time.Now().UTC()
	start, _, err := PeriodWindow(now, resetDay, "UTC")
	if err != nil {
		t.Fatalf("PeriodWindow: %v", err)
	}
	seedTrafficRow(t, uuid, 1_000, 2_000, 300, 400, start)

	// 把节点改成禁用（模拟管理员保存 traffic_reset_day=0）。
	if err := dbcore.GetDBInstance().Model(&models.Client{}).Where("uuid = ?", uuid).
		Update("traffic_reset_day", 0).Error; err != nil {
		t.Fatalf("disable period: %v", err)
	}

	if err := AccumulateTraffic(uuid, 1_100, 2_200, 100, 200); err != nil {
		t.Fatalf("accumulate after disabling: %v", err)
	}
	got := mustTrafficRow(t, uuid)
	if got.Up != 1_100 || got.Down != 2_200 {
		t.Fatalf("cumulative = (%d,%d), want (1100,2200)（禁用不影响累计）", got.Up, got.Down)
	}
	if got.PeriodUp != 0 || got.PeriodDown != 0 || !got.PeriodStart.IsZero() {
		t.Fatalf("period fields must be cleared after disabling, got %+v", got)
	}

	var row models.ClientTrafficTotal
	if err := dbcore.GetDBInstance().Where("uuid = ?", uuid).First(&row).Error; err != nil {
		t.Fatalf("load persisted row: %v", err)
	}
	if row.PeriodUp != 0 || row.PeriodDown != 0 || row.PeriodStartedAt != nil {
		t.Fatalf("persisted period columns must be cleared, got %+v", row)
	}
}

// currentBoundary 返回 now 所处周期的起点（UTC），供"起点应等于真实边界"的断言使用。
func currentBoundary(t *testing.T, resetDay int, tzName string) time.Time {
	t.Helper()
	start, _, err := PeriodWindow(time.Now().UTC(), resetDay, tzName)
	if err != nil {
		t.Fatalf("PeriodWindow: %v", err)
	}
	return start.UTC()
}

// countPeriodResetEvents 统计该节点写出的周期重置审计事件条数。
func countPeriodResetEvents(t *testing.T, uuid string) int {
	t.Helper()
	var entries []models.Log
	if err := dbcore.GetDBInstance().Find(&entries).Error; err != nil {
		t.Fatalf("load audit log: %v", err)
	}
	prefix := "traffic period reset:" + uuid
	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Message, prefix) {
			count++
		}
	}
	return count
}

// 老节点首次启用周期（打回缺陷回归）。
//
// 现场：累计行在"周期功能上线前"就存在（period_started_at IS NULL），刚保存了周期配置。
// 首次上报必须把起点补成**当前周期的真实边界**；若写成零值时间（0001-01-01），
// 下一次上报会把它判成"很久以前的起点"，写出一条虚假的 `traffic period reset` 事件。
func TestAccumulateTrafficEnablingPeriodOnExistingRowSeedsBoundaryWithoutReset(t *testing.T) {
	const (
		resetDay = 22
		tzName   = "Asia/Shanghai"
	)
	uuid := setupPeriodTrafficTest(t, 0, "") // 先以"禁用"建行

	// 1) 功能上线前就有的累计行：周期列为空。
	if err := AccumulateTraffic(uuid, 5_000, 6_000, 0, 0); err != nil {
		t.Fatalf("pre-feature report: %v", err)
	}
	if got := mustTrafficRow(t, uuid); !got.PeriodStart.IsZero() {
		t.Fatalf("precondition: period start should be empty before enabling, got %s", got.PeriodStart)
	}

	// 2) 保存周期配置（等价于 admin editClient：写配置 + 置再基线）。
	if err := dbcore.GetDBInstance().Model(&models.Client{}).Where("uuid = ?", uuid).
		Updates(map[string]any{
			"traffic_reset_day":      resetDay,
			"traffic_reset_timezone": tzName,
		}).Error; err != nil {
		t.Fatalf("enable period config: %v", err)
	}
	if err := MarkTrafficRebaseline(uuid); err != nil {
		t.Fatalf("mark rebaseline: %v", err)
	}

	startBefore := currentBoundary(t, resetDay, tzName)

	// 3) 启用后第一次上报：只更新基线（再基线），起点补成真实边界，且不得有重置事件。
	if err := AccumulateTraffic(uuid, 9_999, 8_888, 4_999, 2_888); err != nil {
		t.Fatalf("first report after enabling: %v", err)
	}
	startAfter := currentBoundary(t, resetDay, tzName)

	got := mustTrafficRow(t, uuid)
	if got.Up != 5_000 || got.Down != 6_000 {
		t.Fatalf("cumulative = (%d,%d), want unchanged (5000,6000)（再基线吞掉本次增量）", got.Up, got.Down)
	}
	if got.PeriodUp != 0 || got.PeriodDown != 0 {
		t.Fatalf("period = (%d,%d), want (0,0)（首次启用从 0 开始累加）", got.PeriodUp, got.PeriodDown)
	}
	if got.PeriodStart.IsZero() {
		t.Fatal("period start must be seeded to a real boundary, got zero value")
	}
	if got.PeriodStart.Year() == 1 {
		t.Fatalf("period start = %s（零值时间被落库：这正是被打回的缺陷）", got.PeriodStart)
	}
	if !got.PeriodStart.Equal(startBefore) && !got.PeriodStart.Equal(startAfter) {
		t.Fatalf("period start = %s, want current boundary %s", got.PeriodStart, startBefore)
	}
	if events := countPeriodResetEvents(t, uuid); events != 0 {
		t.Fatalf("reset events = %d, want 0（首次启用没有跨边界，不该有重置事件）", events)
	}

	// 4) 再上报一次带增量：周期正常累加、起点不变、依旧没有重置事件。
	seeded := got.PeriodStart
	if err := AccumulateTraffic(uuid, 10_100, 9_000, 101, 112); err != nil {
		t.Fatalf("second report: %v", err)
	}
	got = mustTrafficRow(t, uuid)
	if got.Up != 5_101 || got.Down != 6_112 {
		t.Fatalf("cumulative = (%d,%d), want (5101,6112)", got.Up, got.Down)
	}
	if got.PeriodUp != 101 || got.PeriodDown != 112 {
		t.Fatalf("period = (%d,%d), want (101,112)（必须正常累加，不能被清）", got.PeriodUp, got.PeriodDown)
	}
	if !got.PeriodStart.Equal(seeded) {
		t.Fatalf("period start drifted from %s to %s", seeded, got.PeriodStart)
	}
	if events := countPeriodResetEvents(t, uuid); events != 0 {
		t.Fatalf("reset events = %d after second report, want 0（虚假重置事件）", events)
	}
}

// warnEntry 是一条被捕获的 WARN：消息 + 结构化字段。
type warnEntry struct {
	message string
	attrs   map[string]string
}

// warnRecorder 捕获经 utils/log 写出的 WARN 及以上记录。
//
// utils/log 在未调用 Setup 时走 slog.Default()（utils/log/log.go 的 logger()），
// 所以测试里换掉 slog 默认 handler 就能断言日志——与"替换全局状态 + t.Cleanup 还原"
// 的既有手法一致（见 .trellis/spec/backend/quality-guidelines.md §4.7）。
type warnRecorder struct {
	mu      sync.Mutex
	records []warnEntry
}

func (w *warnRecorder) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelWarn
}

func (w *warnRecorder) Handle(_ context.Context, record slog.Record) error {
	entry := warnEntry{message: record.Message, attrs: make(map[string]string, record.NumAttrs())}
	record.Attrs(func(attr slog.Attr) bool {
		entry.attrs[attr.Key] = attr.Value.String()
		return true
	})
	w.mu.Lock()
	defer w.mu.Unlock()
	w.records = append(w.records, entry)
	return nil
}

func (w *warnRecorder) WithAttrs([]slog.Attr) slog.Handler { return w }
func (w *warnRecorder) WithGroup(string) slog.Handler      { return w }

func (w *warnRecorder) hasMessage(substr string) bool {
	return w.countMessage(substr) > 0
}

// countMessage 统计包含该子串的 WARN 条数（去重断言用）。
func (w *warnRecorder) countMessage(substr string) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	count := 0
	for _, entry := range w.records {
		if strings.Contains(entry.message, substr) {
			count++
		}
	}
	return count
}

// lastMessage 返回最后一条包含该子串的 WARN（断言字段用）；没有则返回 false。
func (w *warnRecorder) lastMessage(substr string) (warnEntry, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for i := len(w.records) - 1; i >= 0; i-- {
		if strings.Contains(w.records[i].message, substr) {
			return w.records[i], true
		}
	}
	return warnEntry{}, false
}

func installWarnRecorder(t *testing.T) *warnRecorder {
	t.Helper()
	previous := slog.Default()
	recorder := &warnRecorder{}
	slog.SetDefault(slog.New(recorder))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return recorder
}

// 库里周期配置非法（外部改库所致）：本次只跳过周期记账，**保留**周期现值，
// 累计照常增长，并留 WARN（Lead 复审批准：数据保全省先于记账）。
func TestAccumulateTrafficInvalidPersistedConfigKeepsPeriodValues(t *testing.T) {
	const (
		resetDay = 22
		tzName   = "Asia/Shanghai"
	)
	uuid := setupPeriodTrafficTest(t, resetDay, tzName)
	logs := installWarnRecorder(t)

	// 1) 正常记账出一组非零周期值。
	if err := AccumulateTraffic(uuid, 1_000, 2_000, 0, 0); err != nil {
		t.Fatalf("first report: %v", err)
	}
	if err := AccumulateTraffic(uuid, 1_300, 2_400, 300, 400); err != nil {
		t.Fatalf("second report: %v", err)
	}
	before := mustTrafficRow(t, uuid)
	if before.PeriodUp != 300 || before.PeriodDown != 400 || before.PeriodStart.IsZero() {
		t.Fatalf("precondition: want non-zero period values, got %+v", before)
	}

	// 2) 外部改库把重置日写成非法值（正常保存路径会拦住，这里模拟"库里被改坏"）。
	if err := dbcore.GetDBInstance().Model(&models.Client{}).Where("uuid = ?", uuid).
		Update("traffic_reset_day", 99).Error; err != nil {
		t.Fatalf("tamper reset day: %v", err)
	}

	// 3) 再上报一次：周期记账被跳过，但**不能**清空周期数据。
	if err := AccumulateTraffic(uuid, 1_500, 2_800, 200, 400); err != nil {
		t.Fatalf("report with invalid config: %v", err)
	}

	got := mustTrafficRow(t, uuid) // 内含"内存 == 库"断言
	if got.Up != 1_500 || got.Down != 2_800 {
		t.Fatalf("cumulative = (%d,%d), want (1500,2800)（累计照常增长）", got.Up, got.Down)
	}
	if got.PeriodUp != before.PeriodUp || got.PeriodDown != before.PeriodDown {
		t.Fatalf("period = (%d,%d), want unchanged (%d,%d)（非法配置不得清空周期值）",
			got.PeriodUp, got.PeriodDown, before.PeriodUp, before.PeriodDown)
	}
	if !got.PeriodStart.Equal(before.PeriodStart) {
		t.Fatalf("period start = %s, want unchanged %s", got.PeriodStart, before.PeriodStart)
	}
	if !logs.hasMessage("invalid traffic period config") {
		t.Fatalf("expected a WARN about the invalid period config, captured=%v", logs.records)
	}

	// 期间不产生重置审计事件（没有跨边界，也没有被"降级成禁用"顺手清空）。
	if events := countPeriodResetEvents(t, uuid); events != 0 {
		t.Fatalf("reset events = %d, want 0", events)
	}

	// 4) 把配置改回合法值后，记账恢复（起点仍是原先的；若已跨边界则正常归零）。
	if err := dbcore.GetDBInstance().Model(&models.Client{}).Where("uuid = ?", uuid).
		Update("traffic_reset_day", resetDay).Error; err != nil {
		t.Fatalf("restore reset day: %v", err)
	}
	if err := AccumulateTraffic(uuid, 1_600, 2_900, 100, 100); err != nil {
		t.Fatalf("report after restoring config: %v", err)
	}
	got = mustTrafficRow(t, uuid)
	if got.PeriodUp < before.PeriodUp || got.PeriodDown < before.PeriodDown {
		t.Fatalf("period should resume accumulating, got (%d,%d), want >= (%d,%d)",
			got.PeriodUp, got.PeriodDown, before.PeriodUp, before.PeriodDown)
	}
}

// 周期归零必须写审计日志（PRD 需求 6：重置事件可观测）。
func TestAccumulateTrafficWritesAuditLogOnReset(t *testing.T) {
	const resetDay = 5
	uuid := setupPeriodTrafficTest(t, resetDay, "UTC")

	now := time.Now().UTC()
	// 同上：起点取 now 所处周期的上一个边界，保证本次上报跨边界并写审计日志。
	seed := previousBoundary(t, now, resetDay)
	seedTrafficRow(t, uuid, 1_000, 2_000, 300, 400, seed)

	if err := AccumulateTraffic(uuid, 1_100, 2_200, 100, 200); err != nil {
		t.Fatalf("accumulate across boundary: %v", err)
	}

	var entries []models.Log
	if err := dbcore.GetDBInstance().Where("msg_type = ?", "info").Find(&entries).Error; err != nil {
		t.Fatalf("load audit log: %v", err)
	}
	want := fmt.Sprintf("traffic period reset:%s", uuid)
	found := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Message, want) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no audit entry with prefix %q in %#v", want, entries)
	}
}

// MarkTrafficRebaseline 不得为"从未上报过"的节点建零值占位行（否则首次上报的基线被清零）。
func TestMarkTrafficRebaselineDoesNotSeedZeroBaseline(t *testing.T) {
	uuid := setupPeriodTrafficTest(t, 0, "")

	if err := MarkTrafficRebaseline(uuid); err != nil {
		t.Fatalf("mark rebaseline: %v", err)
	}
	var count int64
	if err := dbcore.GetDBInstance().Model(&models.ClientTrafficTotal{}).Where("uuid = ?", uuid).Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("rows = %d, want 0（不应插入占位行）", count)
	}

	// 首次上报仍按"只建基线"处理。
	if err := AccumulateTraffic(uuid, 7_000, 8_000, 11, 22); err != nil {
		t.Fatalf("first accumulate: %v", err)
	}
	got := mustTrafficRow(t, uuid)
	if got.Up != 7_000 || got.Down != 8_000 {
		t.Fatalf("baseline = %+v, want up=7000 down=8000", got)
	}
}

func TestValidatePeriodConfigMirrorsSavedClient(t *testing.T) {
	// 保存路径用的校验与 PeriodWindow 必须同源：这里钉住"非法值不会进库"的前置条件。
	if _, err := ValidatePeriodConfig(32, "UTC"); err == nil {
		t.Fatal("resetDay 32 must be rejected")
	}
	if _, err := ValidatePeriodConfig(5, "Not/AZone"); err == nil {
		t.Fatal("invalid timezone must be rejected")
	}
	tz, err := ValidatePeriodConfig(5, "")
	if err != nil || tz != DefaultPeriodTimezone {
		t.Fatalf("ValidatePeriodConfig(5, \"\") = (%q, %v), want (%q, nil)", tz, err, DefaultPeriodTimezone)
	}
}
