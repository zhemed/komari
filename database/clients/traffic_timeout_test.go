package clients

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
)

// 缺陷 2：主库调用必须带显式超时，且超时是**可见**的（不能静默挂起），
// 同时不得破坏累计单调不变式（宁可少计一次，也不能写坏/回退）。

func shrinkMainDBTimeout(t *testing.T, timeout time.Duration) {
	t.Helper()
	previous := mainDBOperationTimeout
	mainDBOperationTimeout = timeout
	t.Cleanup(func() { mainDBOperationTimeout = previous })
}

func setSlowMainDBThreshold(t *testing.T, threshold time.Duration) {
	t.Helper()
	previous := slowMainDBOperationThreshold
	slowMainDBOperationThreshold = threshold
	t.Cleanup(func() { slowMainDBOperationThreshold = previous })
}

// 主库唯一连接被占死时：累计必须在显式超时内返回错误（而不是无限期静默挂起），
// 且不写坏任何状态；连接释放后继续单调增长。
func TestAccumulateTrafficTimesOutInsteadOfHangingOnBusyMainDatabase(t *testing.T) {
	const (
		resetDay = 22
		tzName   = "UTC"
	)
	uuid := setupPeriodTrafficTest(t, resetDay, tzName)
	logs := installWarnRecorder(t)
	shrinkMainDBTimeout(t, 200*time.Millisecond)

	now := time.Now().UTC()
	periodStart, _, err := PeriodWindow(now, resetDay, tzName)
	if err != nil {
		t.Fatalf("PeriodWindow: %v", err)
	}
	seedTrafficRow(t, uuid, 1_000, 2_000, 300, 400, periodStart)
	before := mustTrafficRow(t, uuid)

	// 占住主库唯一的连接（SQLite 分支 SetMaxOpenConns(1)）：开一个事务不提交。
	holder := dbcore.GetDBInstance().Begin()
	if holder.Error != nil {
		t.Fatalf("begin holder transaction: %v", holder.Error)
	}

	started := time.Now()
	err = AccumulateTraffic(uuid, 1_100, 2_100, 100, 100)
	elapsed := time.Since(started)

	if err == nil {
		_ = holder.Rollback()
		t.Fatal("连接被占死时累计应返回错误（不能无限期静默等待）")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		_ = holder.Rollback()
		t.Fatalf("err = %v, want context.DeadlineExceeded（池等待必须受显式超时约束）", err)
	}
	if elapsed > 2*time.Second {
		_ = holder.Rollback()
		t.Fatalf("耗时 %s，显式超时未生效（期望 ~200ms 返回）", elapsed)
	}
	if !logs.hasMessage("main database operation timed out") {
		_ = holder.Rollback()
		t.Fatalf("超时必须留下 WARN（否则仍是静默挂起），captured=%v", logs.records)
	}

	// 不变式（内存侧，不碰库）：超时不动任何状态——既不回退也不清空周期列。
	if cached, ok := GetTrafficTotal(uuid); !ok {
		_ = holder.Rollback()
		t.Fatal("超时不应把节点从内存快照里删掉")
	} else if cached.Up != before.Up || cached.Down != before.Down ||
		cached.PeriodUp != before.PeriodUp || cached.PeriodDown != before.PeriodDown ||
		!cached.PeriodStart.Equal(before.PeriodStart) {
		_ = holder.Rollback()
		t.Fatalf("超时改动了内存状态：before=%+v after=%+v", before, cached)
	}

	if err := holder.Rollback().Error; err != nil {
		t.Fatalf("rollback holder transaction: %v", err)
	}

	// 库侧同样未被改动（此处的断言必须在释放连接之后做，否则它会自己撞上同一个超时）。
	persisted := mustTrafficRow(t, uuid)
	if persisted.Up != before.Up || persisted.Down != before.Down ||
		persisted.PeriodUp != before.PeriodUp || persisted.PeriodDown != before.PeriodDown ||
		!persisted.PeriodStart.Equal(before.PeriodStart) {
		t.Fatalf("超时改动了库中状态：before=%+v after=%+v", before, persisted)
	}

	// 恢复后照常累加：单调、不回退。
	if err := AccumulateTraffic(uuid, 1_200, 2_200, 100, 100); err != nil {
		t.Fatalf("恢复后上报失败：%v", err)
	}
	recovered := mustTrafficRow(t, uuid)
	if recovered.Up != before.Up+100 || recovered.Down != before.Down+100 {
		t.Fatalf("恢复后 = (%d,%d), want (%d,%d)", recovered.Up, recovered.Down, before.Up+100, before.Down+100)
	}
}

// 慢操作（未超时但要记 WARN）：日志含 operation / uuid / elapsed_ms。
func TestWithMainDBTimeoutLogsSlowOperation(t *testing.T) {
	uuid := setupPeriodTrafficTest(t, 0, "")
	logs := installWarnRecorder(t)
	setSlowMainDBThreshold(t, 0) // 任何耗时的操作都算"慢"

	if err := AccumulateTraffic(uuid, 100, 200, 0, 0); err != nil {
		t.Fatalf("accumulate: %v", err)
	}

	entry, ok := logs.lastMessage("slow main database operation")
	if !ok {
		t.Fatalf("缺少慢操作 WARN，captured=%v", logs.records)
	}
	if entry.attrs["uuid"] != uuid || entry.attrs["operation"] == "" || entry.attrs["elapsed_ms"] == "" {
		t.Fatalf("慢操作 WARN 字段不完整：%v", entry.attrs)
	}
}

// 读配置失败**不得**被当成"禁用"：否则会顺手清空该节点的周期列（数据丢失）。
// 这里用"连接被占死"制造读失败，断言周期列原样保留。
func TestPeriodConfigReadFailureKeepsPeriodColumns(t *testing.T) {
	const (
		resetDay = 22
		tzName   = "UTC"
	)
	uuid := setupPeriodTrafficTest(t, resetDay, tzName)
	shrinkMainDBTimeout(t, 200*time.Millisecond)

	now := time.Now().UTC()
	periodStart, _, err := PeriodWindow(now, resetDay, tzName)
	if err != nil {
		t.Fatalf("PeriodWindow: %v", err)
	}
	seedTrafficRow(t, uuid, 5_000, 6_000, 700, 800, periodStart)

	holder := dbcore.GetDBInstance().Begin()
	if holder.Error != nil {
		t.Fatalf("begin holder transaction: %v", holder.Error)
	}
	if err := AccumulateTraffic(uuid, 5_100, 6_100, 100, 100); err == nil {
		_ = holder.Rollback()
		t.Fatal("读配置失败时应返回错误（而不是静默当成禁用）")
	}
	if err := holder.Rollback().Error; err != nil {
		t.Fatalf("rollback holder transaction: %v", err)
	}

	var row struct {
		PeriodUp        int64
		PeriodDown      int64
		PeriodStartedAt *time.Time
	}
	if err := dbcore.GetDBInstance().Model(&row).Table("client_traffic_totals").
		Where("uuid = ?", uuid).Take(&row).Error; err != nil {
		t.Fatalf("load persisted period columns: %v", err)
	}
	if row.PeriodUp != 700 || row.PeriodDown != 800 {
		t.Fatalf("周期值被清空：period=(%d,%d), want (700,800)", row.PeriodUp, row.PeriodDown)
	}
	if row.PeriodStartedAt == nil || !row.PeriodStartedAt.UTC().Equal(periodStart.UTC()) {
		t.Fatalf("周期起点被清空或改动：%v, want %s", row.PeriodStartedAt, periodStart.UTC())
	}
}
