package metricstore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/komari-monitor/komari/pkg/metric"
	v1 "github.com/komari-monitor/komari/protocol/v1"
)

// 批处理健壮性（task-13）：
//   - 缺陷 1：失败批次限次重试 ⇒ 死信（丢弃 + 计数 + ERROR 摘要），pending 有上限；
//   - 判别测试 E2a：阻塞型累加 ⇒ 后续批次不再产生指标点，且队列灌满后 WriteReport 拒绝；
//   - 判别测试 E2b：共享租约泄漏 ⇒ 信号量阻塞是"吵闹的"（~10-13s 内出现 ERROR）。

// shrinkBatcherForTest 缩小队列/缓冲上限，注册还原（t.Cleanup，LIFO：后注册的先执行）。
func shrinkBatcherForTest(t *testing.T, queueSize, pendingLimit int) {
	t.Helper()
	previousQueueSize := reportBatchQueueSize
	previousPendingLimit := maxPendingReports
	reportBatchQueueSize = queueSize
	maxPendingReports = pendingLimit
	t.Cleanup(func() {
		reportBatchQueueSize = previousQueueSize
		maxPendingReports = previousPendingLimit
	})
}

// brokenStoreForTest 装一个**已关闭**的 store：批次写入立刻失败（而不是阻塞 10s），
// 同时 GetStore() 非 nil，所以 WriteReport 仍能正常入队。注册还原（t.Cleanup）。
func brokenStoreForTest(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:broken-%d?mode=memory&cache=shared", time.Now().UnixNano())
	closed, err := metric.Open(context.Background(), metric.SQLite(dsn, metric.WithMaxOpenConns(1)))
	if err != nil {
		t.Fatalf("open throwaway store: %v", err)
	}
	if err := closed.Close(); err != nil {
		t.Fatalf("close throwaway store: %v", err)
	}
	storeMu.Lock()
	previous := store
	store = closed
	storeMu.Unlock()
	t.Cleanup(func() {
		storeMu.Lock()
		store = previous
		storeMu.Unlock()
	})
}

func cpuPoints(t *testing.T, s *metric.Store, uuid string, start, end time.Time) []metric.Point {
	t.Helper()
	points, err := s.Query(context.Background(), metric.Query{
		MetricName: MetricCPU,
		EntityID:   uuid,
		Start:      start,
		End:        end,
		Order:      metric.OrderAsc,
	})
	if err != nil {
		t.Fatalf("query %s points for %s: %v", MetricCPU, uuid, err)
	}
	return points
}

// 缺陷 1 核心：重试到上限的批次进死信后，**后续报告不再被堵住**。
func TestPoisonReportBatchDoesNotBlockFollowingReports(t *testing.T) {
	ctx := context.Background()
	s := useReportTestStore(t, nil)
	resetTrafficHeartbeat()
	logs := installLogRecorder(t)
	SetTrafficAccumulator(func(string, int64, int64, int64, int64) error { return nil })
	t.Cleanup(func() { SetTrafficAccumulator(nil) })

	// 确定性失败注入：已取消的 ctx ⇒ writeReportBatch 在等共享租约时就失败。
	failedCtx, cancel := context.WithCancel(ctx)
	cancel()

	base := time.Now().UTC().Truncate(time.Minute).Add(5 * time.Second)
	pending := []v1.Report{{
		UUID:      "poison-node",
		UpdatedAt: base,
		CPU:       v1.CPUReport{Usage: 11},
		Network:   v1.NetworkReport{TotalUp: 100, TotalDown: 200},
	}}
	retry := &reportBatchRetry{}

	for attempt := 1; attempt <= maxReportBatchAttempts; attempt++ {
		err := writePendingReports(failedCtx, &pending, retry)
		if attempt < maxReportBatchAttempts {
			if err == nil {
				t.Fatalf("attempt %d 返回 nil，未达上限时应把错误上抛（保持吵闹）", attempt)
			}
			if len(pending) != 1 {
				t.Fatalf("attempt %d 后 pending = %d, want 1（未达上限不能丢）", attempt, len(pending))
			}
			continue
		}
		// 第 maxReportBatchAttempts 次：死信丢弃，pending 被清空，调用方不再看到错误。
		if err != nil {
			t.Fatalf("死信后应返回 nil（已处理），got %v", err)
		}
	}
	if len(pending) != 0 {
		t.Fatalf("死信后 pending = %d, want 0（否则队头仍被堵住）", len(pending))
	}

	heartbeat := TrafficPipelineStatus()
	if heartbeat.DroppedBatchesTotal != 1 || heartbeat.DroppedReportsTotal != 1 {
		t.Fatalf("死信计数 = (batches=%d, reports=%d), want (1,1)",
			heartbeat.DroppedBatchesTotal, heartbeat.DroppedReportsTotal)
	}
	if got := logs.count(slog.LevelError, "dropping metric report batch"); got != 1 {
		t.Fatalf("死信 ERROR 数 = %d, want 1", got)
	}
	entry, ok := logs.last(slog.LevelError, "dropping metric report batch")
	if !ok {
		t.Fatal("missing dead-letter ERROR")
	}
	// 批次摘要必须能定位问题：条数 + 节点样本 uuid。
	if !strings.Contains(entry.message, "reports=1") || !strings.Contains(entry.message, "poison-node") {
		t.Fatalf("死信日志缺少批次摘要：%q", entry.message)
	}
	if cpuPoints(t, s, "poison-node", base.Add(-time.Second), base.Add(time.Minute)) != nil {
		t.Fatal("失败的批次不应写入指标")
	}

	// 后续报告照常写入（毒丸不再堵住队列）。
	pending = append(pending, v1.Report{
		UUID:      "healthy-node",
		UpdatedAt: base,
		CPU:       v1.CPUReport{Usage: 42},
		Network:   v1.NetworkReport{TotalUp: 1, TotalDown: 2},
	})
	if err := writePendingReports(ctx, &pending, retry); err != nil {
		t.Fatalf("后续批次应成功：%v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("后续批次后 pending = %d, want 0", len(pending))
	}
	assertMetricValues(t, s, MetricCPU, "healthy-node", base.Add(-time.Second), base.Add(time.Minute), []float64{42})
}

// pending 上限策略：超限丢**最旧**并计数 + 告警。
func TestTrimPendingReportsDropsOldestAndCounts(t *testing.T) {
	resetTrafficHeartbeat()
	logs := installLogRecorder(t)

	pending := make([]v1.Report, 0, 10)
	for i := 0; i < 10; i++ {
		pending = append(pending, v1.Report{UUID: fmt.Sprintf("node-%d", i)})
	}
	trimmed := trimPendingReports(pending, 4)
	if len(trimmed) != 4 {
		t.Fatalf("trimmed len = %d, want 4", len(trimmed))
	}
	if trimmed[0].UUID != "node-6" || trimmed[3].UUID != "node-9" {
		t.Fatalf("应变保留最新 4 条，got [%s..%s]", trimmed[0].UUID, trimmed[3].UUID)
	}
	if got := TrafficPipelineStatus().DroppedReportsTotal; got != 6 {
		t.Fatalf("DroppedReportsTotal = %d, want 6", got)
	}
	if got := logs.count(slog.LevelWarn, "dropping oldest pending metric reports"); got != 1 {
		t.Fatalf("超限 WARN 数 = %d, want 1", got)
	}

	// 未超限时不得改动、不得计数。
	before := TrafficPipelineStatus().DroppedReportsTotal
	if got := trimPendingReports(pending[:3], 4); len(got) != 3 {
		t.Fatalf("未超限不应裁剪：len = %d", len(got))
	}
	if after := TrafficPipelineStatus().DroppedReportsTotal; after != before {
		t.Fatalf("未超限不应计数：%d → %d", before, after)
	}
}

// 上限确实接在 worker 循环上：写入持续失败时 pending 不会无限增长。
func TestPendingBufferIsBoundedInWorkerLoop(t *testing.T) {
	ctx := context.Background()
	useReportTestStore(t, nil)
	resetTrafficHeartbeat()
	installLogRecorder(t)
	shrinkBatcherForTest(t, 200, 16) // 队列 200、pending 上限 16

	SetTrafficAccumulator(func(string, int64, int64, int64, int64) error { return nil })
	t.Cleanup(func() { SetTrafficAccumulator(nil) })
	StartReportBatcher()
	t.Cleanup(func() {
		if err := StopReportBatcher(ctx); err != nil {
			t.Errorf("stop report batcher: %v", err)
		}
	})
	// 注册在 stop 之后 ⇒ LIFO 下先还原 store，再停 worker（避免停机 flush 撞上坏 store）。
	brokenStoreForTest(t)

	base := time.Now().UTC().Truncate(time.Minute)
	for i := 0; i < 200; i++ {
		if _, err := WriteReport(ctx, v1.Report{
			UUID:      fmt.Sprintf("flood-%d", i),
			UpdatedAt: base.Add(time.Duration(i) * time.Millisecond),
		}); err != nil {
			t.Fatalf("queue report %d: %v", i, err)
		}
	}

	// 每 3s 一个 tick：等一次 tick 后上限必须已经把 pending 截住。
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		heartbeat := TrafficPipelineStatus()
		if heartbeat.PendingLen > 0 {
			if heartbeat.PendingLen > 16 {
				t.Fatalf("PendingLen = %d，超过上限 16（pending 无界）", heartbeat.PendingLen)
			}
			if heartbeat.DroppedReportsTotal == 0 {
				t.Fatal("超过上限应计数 DroppedReportsTotal")
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("等待 tick 超时：worker 未把队列排进 pending")
}

// E2a：阻塞型累加 ⇒ 后续批次不再产生指标点；队列灌满后 WriteReport 返回 ErrReportBatchQueueFull。
func TestBlockedAccumulatorStallsBatchesAndFillsQueue(t *testing.T) {
	ctx := context.Background()
	s := useReportTestStore(t, nil)
	resetTrafficHeartbeat()
	shrinkBatcherForTest(t, 32, 64)

	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	SetTrafficAccumulator(func(string, int64, int64, int64, int64) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		return nil
	})

	StartReportBatcher()
	// 注册顺序 = 执行顺序的逆序：先放行回调，再停 worker，最后还原队列尺寸。
	t.Cleanup(func() {
		reportBatchQueueSize = 4096
		maxPendingReports = 4096 * 2
	})
	t.Cleanup(func() {
		if err := StopReportBatcher(ctx); err != nil {
			t.Errorf("stop report batcher: %v", err)
		}
		SetTrafficAccumulator(nil)
	})
	t.Cleanup(func() { close(release) })

	base := time.Now().UTC().Truncate(time.Minute).Add(5 * time.Second)
	first := v1.Report{
		UUID:      "stall-first",
		UpdatedAt: base,
		CPU:       v1.CPUReport{Usage: 1},
		Network:   v1.NetworkReport{TotalUp: 10, TotalDown: 20},
	}
	if _, err := WriteReport(ctx, first); err != nil {
		t.Fatalf("queue first report: %v", err)
	}

	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("worker 未进入累加回调（无法构造阻塞现场）")
	}

	// 卡在回调里之后的新上报：不会再产生指标点。
	second := v1.Report{
		UUID:      "stall-second",
		UpdatedAt: base.Add(time.Second),
		CPU:       v1.CPUReport{Usage: 2},
		Network:   v1.NetworkReport{TotalUp: 11, TotalDown: 21},
	}
	if _, err := WriteReport(ctx, second); err != nil {
		t.Fatalf("queue second report: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	if points := cpuPoints(t, s, "stall-second", base.Add(-time.Second), base.Add(time.Minute)); len(points) != 0 {
		t.Fatalf("批次卡住后仍写出指标点：%#v", points)
	}
	// 反证"互斥"：第一批的指标是写成功的（卡的是它之后的累加回调，不是写库）。
	if points := cpuPoints(t, s, "stall-first", base.Add(-time.Second), base.Add(time.Minute)); len(points) == 0 {
		t.Fatal("第一批指标应已写入（说明卡点在写库之后的回调里）")
	}

	// 灌满队列 ⇒ 拒绝新上报（不再无限吃内存）。
	full := false
	for i := 0; i < reportBatchQueueSize+8; i++ {
		_, err := WriteReport(ctx, v1.Report{
			UUID:      fmt.Sprintf("flood-%d", i),
			UpdatedAt: base.Add(time.Duration(i+2) * time.Second),
		})
		if errors.Is(err, ErrReportBatchQueueFull) {
			full = true
			break
		}
		if err != nil {
			t.Fatalf("unexpected write error: %v", err)
		}
	}
	if !full {
		t.Fatalf("队列灌满后应返回 ErrReportBatchQueueFull（容量 %d）", reportBatchQueueSize)
	}

	heartbeat := TrafficPipelineStatus()
	if heartbeat.QueueLen == 0 {
		t.Fatal("心跳应显示队列积压")
	}
	if !heartbeat.WorkerAlive {
		t.Fatal("worker 只是卡在回调里，不应被判成已退出")
	}
}

// E2b：共享租约泄漏 ⇒ 信号量阻塞必须在 ~10-13s 内以 ERROR 暴露（与"日志无 error"互斥）。
func TestLeakedSharedLeaseSurfacesAsBatchError(t *testing.T) {
	ctx := context.Background()
	useReportTestStore(t, nil)
	resetTrafficHeartbeat()
	logs := installLogRecorder(t)
	SetTrafficAccumulator(func(string, int64, int64, int64, int64) error { return nil })

	StartReportBatcher()
	t.Cleanup(func() {
		if err := StopReportBatcher(ctx); err != nil {
			t.Errorf("stop report batcher: %v", err)
		}
		SetTrafficAccumulator(nil)
	})
	// 泄漏共享租约：独占整个 gate 且不主动释放（t.Cleanup 还原，且必须在停 worker 之前执行）。
	if err := storeOperations.Acquire(ctx); err != nil {
		t.Fatalf("acquire exclusive lease: %v", err)
	}
	t.Cleanup(func() { storeOperations.Release() })

	// 入队一条报告：下一个 tick 的 flush 会卡在 AcquireShared 上，直到 10s 写超时。
	if _, err := WriteReport(ctx, v1.Report{
		UUID:      "lease-node",
		UpdatedAt: time.Now().UTC().Truncate(time.Minute),
	}); err != nil {
		t.Fatalf("queue report: %v", err)
	}

	// tick(≤3s) + 写超时(10s) ⇒ 期望 ~13s 内出现 ERROR；这里给到 15s 余量。
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if logs.count(slog.LevelError, "failed to flush metric report batch") > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if got := logs.count(slog.LevelError, "failed to flush metric report batch"); got == 0 {
		t.Fatal("共享租约被占死时批次失败必须以 ERROR 暴露（不能静默挂起）")
	}
	if got := TrafficPipelineStatus().BatchErrorsTotal; got == 0 {
		t.Fatal("批处理失败应计入 BatchErrorsTotal")
	}
}
