package metricstore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	v1 "github.com/komari-monitor/komari/protocol/v1"
)

// 心跳在健康路径上被更新：批写入成功 + 累加回调成功 ⇒ 两个时间戳都要推进。
func TestTrafficHeartbeatUpdatesOnHealthyBatch(t *testing.T) {
	ctx := context.Background()
	useReportTestStore(t, nil)
	resetTrafficHeartbeat()

	SetTrafficAccumulator(func(string, int64, int64, int64, int64) error { return nil })
	t.Cleanup(func() { SetTrafficAccumulator(nil) })

	base := time.Now().UTC().Truncate(time.Minute).Add(5 * time.Second)
	report := v1.Report{UUID: "hb-node", UpdatedAt: base, Network: v1.NetworkReport{TotalUp: 100, TotalDown: 200}}
	if _, err := WriteReport(ctx, report); err != nil {
		t.Fatalf("write report: %v", err)
	}

	heartbeat := TrafficPipelineStatus()
	if heartbeat.LastBatchSuccessUnix == 0 {
		t.Fatal("LastBatchSuccessUnix = 0，健康路径未更新批处理心跳")
	}
	if heartbeat.LastAccumulateSuccessUnix == 0 {
		t.Fatal("LastAccumulateSuccessUnix = 0，健康路径未更新累加心跳")
	}
	if heartbeat.BatchErrorsTotal != 0 || heartbeat.AccumulateErrorsTotal != 0 {
		t.Fatalf("错误计数 = (%d,%d), want (0,0)", heartbeat.BatchErrorsTotal, heartbeat.AccumulateErrorsTotal)
	}
	// 未启动批处理 worker：队列为空、worker 不在跑（心跳要如实反映）。
	if heartbeat.QueueLen != 0 || heartbeat.WorkerAlive {
		t.Fatalf("queue_len=%d worker_alive=%v, want 0/false（未启动 batcher）",
			heartbeat.QueueLen, heartbeat.WorkerAlive)
	}
}

// 累加回调报错 ⇒ 只加 accumulate_errors_total，不动"上次成功"心跳
// （这样看门狗才能把"一直在报错"与"根本没被调用"分开）。
func TestTrafficHeartbeatCountsAccumulateErrors(t *testing.T) {
	ctx := context.Background()
	useReportTestStore(t, nil)
	resetTrafficHeartbeat()

	SetTrafficAccumulator(func(string, int64, int64, int64, int64) error {
		return errors.New("accumulate failed")
	})
	t.Cleanup(func() { SetTrafficAccumulator(nil) })

	base := time.Now().UTC().Truncate(time.Minute).Add(5 * time.Second)
	if _, err := WriteReport(ctx, v1.Report{
		UUID: "hb-err-node", UpdatedAt: base, Network: v1.NetworkReport{TotalUp: 10, TotalDown: 20},
	}); err != nil {
		t.Fatalf("write report: %v", err)
	}

	heartbeat := TrafficPipelineStatus()
	if heartbeat.AccumulateErrorsTotal != 1 {
		t.Fatalf("AccumulateErrorsTotal = %d, want 1", heartbeat.AccumulateErrorsTotal)
	}
	if heartbeat.LastAccumulateSuccessUnix != 0 {
		t.Fatalf("LastAccumulateSuccessUnix = %d, want 0（失败不得推进成功心跳）", heartbeat.LastAccumulateSuccessUnix)
	}
	if heartbeat.LastBatchSuccessUnix == 0 {
		t.Fatal("批写入本身是成功的，LastBatchSuccessUnix 应已推进")
	}
}

// 批处理失败 ⇒ batch_errors_total + 截断后的 last_batch_error。
func TestTrafficHeartbeatCountsBatchErrors(t *testing.T) {
	useReportTestStore(t, nil)
	resetTrafficHeartbeat()

	// 已取消的 ctx：writeReportBatch 在等共享租约时就失败，是确定性的失败注入。
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pending := []v1.Report{{UUID: "hb-batch-err", UpdatedAt: time.Now().UTC()}}
	if err := writePendingReports(ctx, &pending, &reportBatchRetry{}); err == nil {
		t.Fatal("cancelled context should fail the batch")
	}

	heartbeat := TrafficPipelineStatus()
	if heartbeat.BatchErrorsTotal != 1 {
		t.Fatalf("BatchErrorsTotal = %d, want 1", heartbeat.BatchErrorsTotal)
	}
	if heartbeat.LastBatchSuccessUnix != 0 {
		t.Fatalf("LastBatchSuccessUnix = %d, want 0（失败不得推进成功心跳）", heartbeat.LastBatchSuccessUnix)
	}
	if heartbeat.LastBatchError == "" {
		t.Fatal("LastBatchError 为空，失败原因应被记录")
	}
}

// 错误串对外暴露时按 200 字节截断（长错误不刷屏，且不泄露超长 DSN 片段）。
func TestTruncateError(t *testing.T) {
	long := strings.Repeat("x", lastBatchErrorMaxLen+50)
	got := truncateError(long, lastBatchErrorMaxLen)
	if len(got) != lastBatchErrorMaxLen {
		t.Fatalf("len(truncateError) = %d, want %d", len(got), lastBatchErrorMaxLen)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("截断结果应以 ... 结尾：%q", got[len(got)-8:])
	}
	short := "boom"
	if got := truncateError(short, lastBatchErrorMaxLen); got != short {
		t.Fatalf("短错误不应被改动：%q", got)
	}
}

// 心跳的 queue_len / worker_alive 来自真实 worker：启停之间必须如实变化。
func TestTrafficPipelineStatusTracksWorkerLifecycle(t *testing.T) {
	ctx := context.Background()
	useReportTestStore(t, nil)
	resetTrafficHeartbeat()

	before := TrafficPipelineStatus()
	if before.WorkerAlive {
		t.Fatal("未启动时 worker_alive 应为 false")
	}

	StartReportBatcher()
	alive := TrafficPipelineStatus()
	if !alive.WorkerAlive {
		t.Fatal("启动后 worker_alive 应为 true")
	}

	if _, err := WriteReport(ctx, v1.Report{
		UUID: "hb-lifecycle", UpdatedAt: time.Now().UTC(),
		Network: v1.NetworkReport{TotalUp: 1, TotalDown: 2},
	}); err != nil {
		t.Fatalf("queue report: %v", err)
	}
	if queued := TrafficPipelineStatus(); queued.QueueLen != 1 {
		t.Fatalf("QueueLen = %d, want 1（入队后应可见）", queued.QueueLen)
	}

	if err := FlushReportBatch(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}
	drained := TrafficPipelineStatus()
	if drained.QueueLen != 0 {
		t.Fatalf("QueueLen = %d, want 0（flush 后应排空）", drained.QueueLen)
	}

	if err := StopReportBatcher(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}
	stopped := TrafficPipelineStatus()
	if stopped.WorkerAlive {
		t.Fatal("停止后 worker_alive 应为 false")
	}
}
