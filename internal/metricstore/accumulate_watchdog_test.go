package metricstore

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// utils/log 在未 Setup 时走 slog.Default()（utils/log/log.go 的 logger()），
// 所以测试里换掉 slog 默认 handler 就能断言 WARN/INFO——
// 与仓库既有"替换全局状态 + t.Cleanup 还原"的手法一致（quality-guidelines.md §4.7）。
type logEntry struct {
	level   slog.Level
	message string
	attrs   map[string]string
}

type logRecorder struct {
	mu      sync.Mutex
	entries []logEntry
}

func (r *logRecorder) Enabled(_ context.Context, level slog.Level) bool { return true }

func (r *logRecorder) Handle(_ context.Context, record slog.Record) error {
	entry := logEntry{level: record.Level, message: record.Message, attrs: map[string]string{}}
	record.Attrs(func(attr slog.Attr) bool {
		entry.attrs[attr.Key] = attr.Value.String()
		return true
	})
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entry)
	return nil
}

func (r *logRecorder) WithAttrs([]slog.Attr) slog.Handler { return r }
func (r *logRecorder) WithGroup(string) slog.Handler      { return r }

// count 统计指定级别、消息包含 substr 的日志条数。
func (r *logRecorder) count(level slog.Level, substr string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, entry := range r.entries {
		if entry.level == level && strings.Contains(entry.message, substr) {
			count++
		}
	}
	return count
}

func (r *logRecorder) last(level slog.Level, substr string) (logEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.entries) - 1; i >= 0; i-- {
		if r.entries[i].level == level && strings.Contains(r.entries[i].message, substr) {
			return r.entries[i], true
		}
	}
	return logEntry{}, false
}

func installLogRecorder(t *testing.T) *logRecorder {
	t.Helper()
	previous := slog.Default()
	recorder := &logRecorder{}
	slog.SetDefault(slog.New(recorder))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return recorder
}

// resetTrafficHeartbeat 清空心跳计数（同包测试共享全局状态，避免互相串味）。
func resetTrafficHeartbeat() {
	heartbeatMu.Lock()
	lastBatchSuccess = 0
	lastAccumulateSuccess = 0
	batchErrorsTotal = 0
	accumulateErrorsTotal = 0
	stallEventsTotal = 0
	lastBatchErrorText = ""
	heartbeatMu.Unlock()
}

// 健康态不告警：心跳新鲜 + 队列有积压（正常处理中）。
func TestAccumulateWatchdogStaysQuietWhenHealthy(t *testing.T) {
	logs := installLogRecorder(t)
	resetTrafficHeartbeat()
	now := time.Now().UTC()
	watchdog := newAccumulateWatchdog(now.Add(-time.Hour))

	heartbeat := TrafficPipelineHeartbeat{
		LastAccumulateSuccessUnix: now.Add(-5 * time.Second).Unix(),
		LastBatchSuccessUnix:      now.Add(-5 * time.Second).Unix(),
		QueueLen:                  7,
		WorkerAlive:               true,
	}
	for i := 0; i < 10; i++ {
		if got := watchdog.check(now, heartbeat); got != watchdogSteady {
			t.Fatalf("check #%d = %v, want steady（健康态不应告警）", i, got)
		}
	}
	if got := logs.count(slog.LevelWarn, "traffic accumulation stalled"); got != 0 {
		t.Fatalf("WARN 数 = %d, want 0", got)
	}
}

// 空闲不告警：队列为空时心跳再旧也不算停摆（夜里没人上报是正常的）。
func TestAccumulateWatchdogStaysQuietWhenIdle(t *testing.T) {
	logs := installLogRecorder(t)
	resetTrafficHeartbeat()
	now := time.Now().UTC()
	watchdog := newAccumulateWatchdog(now.Add(-time.Hour))

	heartbeat := TrafficPipelineHeartbeat{
		LastAccumulateSuccessUnix: now.Add(-30 * time.Minute).Unix(),
		LastBatchSuccessUnix:      now.Add(-30 * time.Minute).Unix(),
		QueueLen:                  0, // 没有上报在等
		WorkerAlive:               true,
	}
	for i := 0; i < 10; i++ {
		if got := watchdog.check(now, heartbeat); got != watchdogSteady {
			t.Fatalf("check #%d = %v, want steady（空闲不是停摆）", i, got)
		}
	}
	if got := logs.count(slog.LevelWarn, "traffic accumulation stalled"); got != 0 {
		t.Fatalf("WARN 数 = %d, want 0（空闲误报）", got)
	}
}

// 启动宽限：还没有过一次累加成功时以看门狗启动时刻为基线，不能在启动瞬间误报。
func TestAccumulateWatchdogRespectsStartupGrace(t *testing.T) {
	logs := installLogRecorder(t)
	resetTrafficHeartbeat()
	now := time.Now().UTC()

	// 刚启动 + 队列有积压：仍在宽限期内 ⇒ 不告警。
	fresh := newAccumulateWatchdog(now.Add(-5 * time.Second))
	heartbeat := TrafficPipelineHeartbeat{QueueLen: 3, WorkerAlive: true}
	if got := fresh.check(now, heartbeat); got != watchdogSteady {
		t.Fatalf("刚启动 check = %v, want steady（宽限期内不误报）", got)
	}

	// 启动很久且从未成功过 + 队列有积压 ⇒ 真停摆。
	stale := newAccumulateWatchdog(now.Add(-10 * time.Minute))
	if got := stale.check(now, heartbeat); got != watchdogStalled {
		t.Fatalf("启动 10 分钟仍无累加成功 check = %v, want stalled", got)
	}
	if got := logs.count(slog.LevelWarn, "traffic accumulation stalled"); got != 1 {
		t.Fatalf("WARN 数 = %d, want 1", got)
	}
}

// 停摆告警一次：持续停摆不重复告警，但计数只加一次。
func TestAccumulateWatchdogWarnsOncePerStall(t *testing.T) {
	logs := installLogRecorder(t)
	resetTrafficHeartbeat()
	now := time.Now().UTC()
	watchdog := newAccumulateWatchdog(now.Add(-time.Hour))

	stalled := TrafficPipelineHeartbeat{
		LastAccumulateSuccessUnix: now.Add(-200 * time.Second).Unix(),
		LastBatchSuccessUnix:      now.Add(-200 * time.Second).Unix(),
		QueueLen:                  42,
		WorkerAlive:               true,
		BatchErrorsTotal:          3,
		AccumulateErrorsTotal:     1,
		LastBatchError:            "boom",
	}

	if got := watchdog.check(now, stalled); got != watchdogStalled {
		t.Fatalf("first check = %v, want stalled", got)
	}
	// 持续停摆：后续巡检不得重复告警。
	for i := 0; i < 8; i++ {
		if got := watchdog.check(now.Add(time.Duration(i+1)*accumulateWatchdogInterval), stalled); got != watchdogSteady {
			t.Fatalf("repeat check #%d = %v, want steady（同一次停摆只告一次）", i, got)
		}
	}
	if got := logs.count(slog.LevelWarn, "traffic accumulation stalled"); got != 1 {
		t.Fatalf("WARN 数 = %d, want 1（持续停摆不得重复告警）", got)
	}
	entry, ok := logs.last(slog.LevelWarn, "traffic accumulation stalled")
	if !ok {
		t.Fatal("missing stall WARN")
	}
	if entry.attrs["queue_len"] != "42" || entry.attrs["silent_seconds"] == "" ||
		entry.attrs["batch_errors_total"] != "3" || entry.attrs["accumulate_errors_total"] != "1" {
		t.Fatalf("停摆 WARN 字段不完整：%v", entry.attrs)
	}
	if got := TrafficPipelineStatus().StallEventsTotal; got != 1 {
		t.Fatalf("StallEventsTotal = %d, want 1（停摆计数只加一次）", got)
	}
}

// 恢复告 INFO 一次：恢复后继续健康不得重复告警，再停摆才会重新计数。
func TestAccumulateWatchdogLogsRecoveryOnce(t *testing.T) {
	logs := installLogRecorder(t)
	resetTrafficHeartbeat()
	now := time.Now().UTC()
	watchdog := newAccumulateWatchdog(now.Add(-time.Hour))

	stalled := TrafficPipelineHeartbeat{
		LastAccumulateSuccessUnix: now.Add(-300 * time.Second).Unix(),
		QueueLen:                  5,
		WorkerAlive:               true,
	}
	if got := watchdog.check(now, stalled); got != watchdogStalled {
		t.Fatalf("check = %v, want stalled", got)
	}

	recovered := TrafficPipelineHeartbeat{
		LastAccumulateSuccessUnix: now.Unix(),
		QueueLen:                  0,
		WorkerAlive:               true,
	}
	if got := watchdog.check(now.Add(time.Second), recovered); got != watchdogRecovered {
		t.Fatalf("recovery check = %v, want recovered", got)
	}
	for i := 0; i < 5; i++ {
		if got := watchdog.check(now.Add(time.Duration(i+2)*time.Second), recovered); got != watchdogSteady {
			t.Fatalf("post-recovery check #%d = %v, want steady（恢复只告一次）", i, got)
		}
	}
	if got := logs.count(slog.LevelInfo, "traffic accumulation recovered"); got != 1 {
		t.Fatalf("INFO 数 = %d, want 1", got)
	}
	if got := logs.count(slog.LevelWarn, "traffic accumulation stalled"); got != 1 {
		t.Fatalf("WARN 数 = %d, want 1", got)
	}

	// 再次停摆（第二次事件）⇒ 重新告警并计数。
	if got := watchdog.check(now.Add(5*time.Minute), stalled); got != watchdogStalled {
		t.Fatalf("second stall check = %v, want stalled（新事件应重新告警）", got)
	}
	if got := logs.count(slog.LevelWarn, "traffic accumulation stalled"); got != 2 {
		t.Fatalf("WARN 数 = %d, want 2（新一次停摆应再告一次）", got)
	}
	if got := TrafficPipelineStatus().StallEventsTotal; got != 2 {
		t.Fatalf("StallEventsTotal = %d, want 2", got)
	}
}
