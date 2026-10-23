package clients

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
)

// 非法周期配置的 WARN 去重（PRD：.trellis/tasks/10-23-traffic-warn-dedup/prd.md）。
//
// 现状实测：库里配置非法时**每次上报 1 条 WARN**；本组用例先在未去重实现上跑出 FAIL，
// 再验证去重后的行为（红→绿证据见提交说明）。

const invalidConfigWarnMessage = "invalid traffic period config"

// mustAccumulate 上报一次，失败即 Fatal（这些用例只在语义断言上做细节）。
func mustAccumulate(t *testing.T, uuid string, totalUp, totalDown, deltaUp, deltaDown int64) {
	t.Helper()
	if err := AccumulateTraffic(uuid, totalUp, totalDown, deltaUp, deltaDown); err != nil {
		t.Fatalf("accumulate(%s): %v", uuid, err)
	}
}

// addPeriodTestClient 在同一测试里再加一个节点配置。
// （setupPeriodTrafficTest 用测试名生成 uuid/token，无法在同一个测试里调用两次。）
func addPeriodTestClient(t *testing.T, suffix string, resetDay int, tzName string) string {
	t.Helper()
	uuid := "node-" + t.Name() + "-" + suffix
	client := models.Client{
		UUID:                 uuid,
		Token:                "token-" + t.Name() + "-" + suffix,
		Name:                 "period-test-" + suffix,
		TrafficResetDay:      resetDay,
		TrafficResetTimezone: tzName,
	}
	if err := dbcore.GetDBInstance().Create(&client).Error; err != nil {
		t.Fatalf("create client config (%s): %v", suffix, err)
	}
	return uuid
}

// tamperTrafficPeriodConfig 直接改库模拟"外部把周期配置改坏/改回"，绕过 API 校验。
func tamperTrafficPeriodConfig(t *testing.T, uuid string, resetDay int, tzName string) {
	t.Helper()
	if err := dbcore.GetDBInstance().Model(&models.Client{}).Where("uuid = ?", uuid).
		Updates(map[string]any{
			"traffic_reset_day":      resetDay,
			"traffic_reset_timezone": tzName,
		}).Error; err != nil {
		t.Fatalf("tamper traffic period config: %v", err)
	}
}

// warnedFingerprint 读取该节点当前的告警去重状态（同包白盒断言）。
func warnedFingerprint(uuid string) (string, bool) {
	periodWarnMu.Lock()
	defer periodWarnMu.Unlock()
	fingerprint, ok := periodWarned[uuid]
	return fingerprint, ok
}

// warnedStateSize 返回去重状态当前的条目数（有界性断言用）。
func warnedStateSize() int {
	periodWarnMu.Lock()
	defer periodWarnMu.Unlock()
	return len(periodWarned)
}

// 同一非法配置连续上报只告警一次；指纹变化（reset_day / timezone）后重新告警一次。
func TestAccumulateTrafficInvalidConfigWarnsOncePerFingerprint(t *testing.T) {
	const (
		resetDay = 22
		tzName   = "Asia/Shanghai"
	)
	uuid := setupPeriodTrafficTest(t, resetDay, tzName)
	logs := installWarnRecorder(t)

	// 先正常建行并记账出一组非零周期值：非法期间要断言"语义不变"。
	mustAccumulate(t, uuid, 1_000, 2_000, 0, 0)
	mustAccumulate(t, uuid, 1_300, 2_400, 300, 400)
	before := mustTrafficRow(t, uuid)
	if before.PeriodUp != 300 || before.PeriodDown != 400 || before.PeriodStart.IsZero() {
		t.Fatalf("precondition: want non-zero period values, got %+v", before)
	}

	// 1) 同一非法配置（99 + 原时区）连续 3 次上报 ⇒ 只 1 条 WARN。
	tamperTrafficPeriodConfig(t, uuid, 99, tzName)
	const (
		reports   = 3
		deltaUp   = 10
		deltaDown = 20
	)
	for i := 0; i < reports; i++ {
		mustAccumulate(t, uuid, int64(1_400+i), 2_600, deltaUp, deltaDown)
	}
	if got := logs.countMessage(invalidConfigWarnMessage); got != 1 {
		t.Fatalf("WARN 数 = %d, want 1（同一非法配置重复上报只告警一次）records=%v", got, logs.records)
	}

	// 语义不变：累计继续增长、周期现值与起点保留。
	afterInvalid := mustTrafficRow(t, uuid)
	wantUp := before.Up + reports*deltaUp
	wantDown := before.Down + reports*deltaDown
	if afterInvalid.Up != wantUp || afterInvalid.Down != wantDown {
		t.Fatalf("cumulative = (%d,%d), want (%d,%d)（累计照常增长）",
			afterInvalid.Up, afterInvalid.Down, wantUp, wantDown)
	}
	if afterInvalid.PeriodUp != before.PeriodUp || afterInvalid.PeriodDown != before.PeriodDown {
		t.Fatalf("period = (%d,%d), want unchanged (%d,%d)",
			afterInvalid.PeriodUp, afterInvalid.PeriodDown, before.PeriodUp, before.PeriodDown)
	}
	if !afterInvalid.PeriodStart.Equal(before.PeriodStart) {
		t.Fatalf("period start = %s, want unchanged %s", afterInvalid.PeriodStart, before.PeriodStart)
	}

	// 日志内容保留 uuid / reset_day / timezone / 错误信息。
	entry, ok := logs.lastMessage(invalidConfigWarnMessage)
	if !ok {
		t.Fatal("no invalid-config WARN captured")
	}
	if entry.attrs["uuid"] != uuid {
		t.Fatalf("WARN uuid = %q, want %q（字段=%v）", entry.attrs["uuid"], uuid, entry.attrs)
	}
	if entry.attrs["reset_day"] != "99" || entry.attrs["timezone"] != tzName {
		t.Fatalf("WARN 配置字段 = reset_day:%q timezone:%q, want 99 / %q",
			entry.attrs["reset_day"], entry.attrs["timezone"], tzName)
	}
	if entry.attrs["error"] == "" {
		t.Fatalf("WARN 缺少错误信息字段：%v", entry.attrs)
	}

	// 2) 指纹变化：reset_day 99 → 32 ⇒ 重新告警一次。
	tamperTrafficPeriodConfig(t, uuid, 32, tzName)
	mustAccumulate(t, uuid, 1_500, 2_700, deltaUp, deltaDown)
	if got := logs.countMessage(invalidConfigWarnMessage); got != 2 {
		t.Fatalf("WARN 数 = %d, want 2（reset_day 变化 ⇒ 新指纹应重新告警）", got)
	}

	// 3) 指纹变化：timezone 换成非法值 ⇒ 再告警一次。
	tamperTrafficPeriodConfig(t, uuid, 32, "Bad/Zone")
	mustAccumulate(t, uuid, 1_600, 2_800, deltaUp, deltaDown)
	if got := logs.countMessage(invalidConfigWarnMessage); got != 3 {
		t.Fatalf("WARN 数 = %d, want 3（timezone 变化 ⇒ 新指纹应重新告警）", got)
	}

	// 4) 恢复合法 ⇒ 记账恢复、去重状态清理；再次变回同一非法指纹 ⇒ 又能告警一次。
	tamperTrafficPeriodConfig(t, uuid, resetDay, tzName)
	beforeRestore := mustTrafficRow(t, uuid)
	const delta = 50
	mustAccumulate(t, uuid, 1_700, 2_900, delta, delta)
	// 清理发生在"配置恢复合法后的第一次上报"里（本包状态只在 AccumulateTraffic 内维护）。
	if _, warned := warnedFingerprint(uuid); warned {
		t.Fatal("配置恢复合法后应清理去重状态")
	}
	restored := mustTrafficRow(t, uuid)
	// 若期间恰好跨过周期边界，周期值会被归零后重记本次增量，两种都算"记账已恢复"。
	if restored.PeriodUp != beforeRestore.PeriodUp+delta && restored.PeriodUp != delta {
		t.Fatalf("period_up = %d, want %d（累加）或 %d（跨边界后重置）",
			restored.PeriodUp, beforeRestore.PeriodUp+delta, delta)
	}

	// 再次非法：指纹与第 1 步相同（99 + 原时区）——若恢复时没清理，这里会被去重压掉。
	tamperTrafficPeriodConfig(t, uuid, 99, tzName)
	mustAccumulate(t, uuid, 1_800, 3_000, deltaUp, deltaDown)
	if got := logs.countMessage(invalidConfigWarnMessage); got != 4 {
		t.Fatalf("WARN 数 = %d, want 4（恢复合法后去重状态应被清理，可重新告警）", got)
	}
}

// 节点删除（DeleteTrafficTotal；DeleteClient 走的就是这条路径）⇒ 去重状态一并清理。
func TestAccumulateTrafficDeleteClearsInvalidConfigWarnState(t *testing.T) {
	const (
		resetDay = 22
		tzName   = "Asia/Shanghai"
	)
	uuid := setupPeriodTrafficTest(t, resetDay, tzName)
	logs := installWarnRecorder(t)

	// 建行后把配置改坏，连续两次上报：只 1 条 WARN（去重生效）。
	mustAccumulate(t, uuid, 1_000, 2_000, 0, 0)
	tamperTrafficPeriodConfig(t, uuid, 99, tzName)
	mustAccumulate(t, uuid, 1_100, 2_100, 100, 100)
	mustAccumulate(t, uuid, 1_200, 2_200, 100, 100)
	if got := logs.countMessage(invalidConfigWarnMessage); got != 1 {
		t.Fatalf("WARN 数 = %d, want 1", got)
	}
	if _, warned := warnedFingerprint(uuid); !warned {
		t.Fatal("precondition: 去重状态应存在")
	}

	// 删除节点的累计行（配置仍非法、节点配置行还在）：去重状态必须被清理。
	if err := DeleteTrafficTotal(uuid); err != nil {
		t.Fatalf("delete traffic total: %v", err)
	}
	if fingerprint, warned := warnedFingerprint(uuid); warned {
		t.Fatalf("删除节点后去重状态仍残留：%q", fingerprint)
	}

	// 行为验证：重建累计行后再非法上报，应该重新告警（
	// 第一次上报是"建行"分支、不告警；第二次走已有行分支）。
	mustAccumulate(t, uuid, 2_000, 3_000, 0, 0)
	mustAccumulate(t, uuid, 2_100, 3_100, 100, 100)
	if got := logs.countMessage(invalidConfigWarnMessage); got != 2 {
		t.Fatalf("WARN 数 = %d, want 2（删除后状态清理，重建节点应能重新告警）", got)
	}
	// 再报一次仍只有 2 条：新指纹的去重同样生效。
	mustAccumulate(t, uuid, 2_200, 3_200, 100, 100)
	if got := logs.countMessage(invalidConfigWarnMessage); got != 2 {
		t.Fatalf("WARN 数 = %d, want 2（重建后同一指纹仍只告警一次）", got)
	}
}

// 去重状态必须并发安全：AccumulateTraffic 会被 metricstore 的批次回调触发。
// 直接并发打这个入口（不碰 SQLite），配合 `go test -race` 验证互斥与"只告警一次"。
func TestWarnInvalidPeriodConfigOnceIsConcurrencySafe(t *testing.T) {
	installWarnRecorder(t)
	const uuid = "node-concurrent-warn"
	t.Cleanup(func() { clearPeriodWarnState(uuid) })

	const goroutines = 32
	var (
		wg      sync.WaitGroup
		warned  int64
		release = make(chan struct{})
	)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-release
			if warnInvalidPeriodConfigOnce(uuid, 99, "Bad/Zone", errors.New("boom")) {
				atomic.AddInt64(&warned, 1)
			}
		}()
	}
	close(release)
	wg.Wait()

	if got := atomic.LoadInt64(&warned); got != 1 {
		t.Fatalf("并发下告警次数 = %d, want 1（同一节点+同一指纹只告警一次）", got)
	}
	if fingerprint, ok := warnedFingerprint(uuid); !ok || fingerprint != "99|Bad/Zone" {
		t.Fatalf("去重状态 = %q ok=%v, want 99|Bad/Zone", fingerprint, ok)
	}
}

// 去重状态有界：不随上报次数增长（同一节点重复上报只占 1 条），健康节点不留条目，
// 配置恢复合法后条目被清理。
func TestInvalidConfigWarnStateIsBounded(t *testing.T) {
	const resetDay = 22
	healthy := setupPeriodTrafficTest(t, resetDay, "UTC")
	broken := addPeriodTestClient(t, "broken", resetDay, "UTC")
	installWarnRecorder(t)

	// 健康节点反复上报：不应产生任何去重条目。
	for i := 0; i < 3; i++ {
		mustAccumulate(t, healthy, int64(1_000+i), 2_000, 10, 10)
	}
	if fingerprint, warned := warnedFingerprint(healthy); warned {
		t.Fatalf("健康节点不应留下去重条目：%q", fingerprint)
	}

	// 非法节点：先正常建行（首次上报走"建行"分支、本来就不告警），再把配置改坏。
	mustAccumulate(t, broken, 1_000, 2_000, 0, 0)
	tamperTrafficPeriodConfig(t, broken, 99, "UTC")
	mustAccumulate(t, broken, 1_100, 2_000, 10, 10)
	if _, warned := warnedFingerprint(broken); !warned {
		t.Fatal("非法节点应留下去重条目")
	}
	sizeAfterFirst := warnedStateSize()

	// 连续 5 次上报：条目数**不增长**（有界 = 与上报次数无关）。
	for i := 0; i < 5; i++ {
		mustAccumulate(t, broken, int64(1_101+i), 2_000, 10, 10)
	}
	if size := warnedStateSize(); size != sizeAfterFirst {
		t.Fatalf("去重状态随上报次数增长：%d → %d", sizeAfterFirst, size)
	}

	// 指纹变化：同一节点只更新指纹，不新增条目。
	tamperTrafficPeriodConfig(t, broken, 32, "UTC")
	mustAccumulate(t, broken, 9_000, 9_000, 10, 10)
	if size := warnedStateSize(); size != sizeAfterFirst {
		t.Fatalf("指纹变化不应新增条目：%d → %d", sizeAfterFirst, size)
	}

	// 配置恢复合法 ⇒ 该节点条目在"恢复后的第一次上报"里被清理（有界性的落点）。
	tamperTrafficPeriodConfig(t, broken, resetDay, "UTC")
	mustAccumulate(t, broken, 9_100, 9_100, 10, 10)
	if fingerprint, warned := warnedFingerprint(broken); warned {
		t.Fatalf("恢复合法后条目应被清理，仍残留 %q", fingerprint)
	}
	if size := warnedStateSize(); size != sizeAfterFirst-1 {
		t.Fatalf("恢复合法后条目数 = %d, want %d", size, sizeAfterFirst-1)
	}
}
