package metricstore

import (
	"context"
	"sync"
	"time"

	logger "github.com/komari-monitor/komari/utils/log"
)

// 累加停摆看门狗：把"静默停摆"变成一条 WARN，并在恢复时给一条 INFO。
//
// 判据刻意用**并发条件**，避免两类误报：
//
//	queue_len > 0（确实有上报在等）  且  now - last_accumulate_success > 90s
//
//   - 只看"心跳变旧"：夜里没人上报时会整夜告警（正常空闲被误判成停摆）；
//   - 只看"队列非空"：上报刚入队、worker 还没到 tick 时会误判（正常抖动）。
//
// 90s 的取法：正常一批是 3s（reportBatchInterval），90s = 30 个批次周期，
// 足以排除单次慢写与 GC 抖动，又远小于人工发现问题的时延。
//
// 动作刻意保守：只告警与计数，**不**重启 worker、**不**改累计语义——
// 停摆的根因（writePendingReports 的队头阻塞、主库无 ctx 调用）是独立缺陷，另行处理。

const (
	// accumulateStallThreshold 是判定"停摆"的静默阈值。
	accumulateStallThreshold = 90 * time.Second
	// accumulateWatchdogInterval 是巡检间隔。
	accumulateWatchdogInterval = 15 * time.Second
)

// watchdogTransition 是一次巡检的状态变迁；日志只在变迁时写，天然做到"同一次停摆只告一次"。
type watchdogTransition int

const (
	watchdogSteady    watchdogTransition = iota // 状态不变
	watchdogStalled                             // 由健康转为停摆
	watchdogRecovered                           // 由停摆转为恢复
)

// accumulateWatchdog 是停摆判定的状态机（纯逻辑 + 一把互斥锁，便于单测）。
type accumulateWatchdog struct {
	mu        sync.Mutex
	startedAt time.Time
	stalled   bool
}

func newAccumulateWatchdog(startedAt time.Time) *accumulateWatchdog {
	return &accumulateWatchdog{startedAt: startedAt.UTC()}
}

// silence 返回"距最近一次累加成功（或看门狗启动）已经过去多久"。
//
// 进程刚起来还没成功过一次时（LastAccumulateSuccessUnix == 0），以看门狗启动时刻为基线：
// 否则 now - 0 是个天文数字，会在启动瞬间就误报。
func (w *accumulateWatchdog) silence(now time.Time, heartbeat TrafficPipelineHeartbeat) time.Duration {
	baseline := w.startedAt
	if heartbeat.LastAccumulateSuccessUnix > 0 {
		baseline = time.Unix(heartbeat.LastAccumulateSuccessUnix, 0).UTC()
	}
	return now.UTC().Sub(baseline)
}

// evaluate 依据一次心跳快照推进状态机，返回状态变迁。
func (w *accumulateWatchdog) evaluate(now time.Time, heartbeat TrafficPipelineHeartbeat) watchdogTransition {
	stalled := heartbeat.QueueLen > 0 && w.silence(now, heartbeat) > accumulateStallThreshold

	w.mu.Lock()
	defer w.mu.Unlock()
	switch {
	case stalled && !w.stalled:
		w.stalled = true
		return watchdogStalled
	case !stalled && w.stalled:
		w.stalled = false
		return watchdogRecovered
	default:
		return watchdogSteady
	}
}

// check 依据给定的心跳快照推进状态机，并按变迁写日志。
// 心跳由调用方传入（而不是在里面直接读全局），这样单测可以用合成快照驱动状态机、
// 并断言"一次 WARN / 一次 INFO"；生产路径由 run() 传实时快照。
func (w *accumulateWatchdog) check(now time.Time, heartbeat TrafficPipelineHeartbeat) watchdogTransition {
	transition := w.evaluate(now, heartbeat)
	switch transition {
	case watchdogStalled:
		recordAccumulateStall()
		logger.Warn("metricstore", "traffic accumulation stalled",
			"silent_seconds", int64(w.silence(now, heartbeat).Seconds()),
			"queue_len", heartbeat.QueueLen,
			"worker_alive", heartbeat.WorkerAlive,
			"batch_errors_total", heartbeat.BatchErrorsTotal,
			"accumulate_errors_total", heartbeat.AccumulateErrorsTotal,
			"last_batch_error", heartbeat.LastBatchError,
		)
	case watchdogRecovered:
		logger.Info("metricstore", "traffic accumulation recovered",
			"silent_seconds", int64(w.silence(now, heartbeat).Seconds()),
			"queue_len", heartbeat.QueueLen,
			"stall_events_total", heartbeat.StallEventsTotal+1,
		)
	}
	return transition
}

// run 按固定间隔巡检，直到 ctx 结束。
func (w *accumulateWatchdog) run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			w.check(now.UTC(), TrafficPipelineStatus())
		}
	}
}

// 看门狗与批处理 worker 同生命周期：StartReportBatcher 启动，StopReportBatcher 停止。
var (
	accumulateWatchdogLifecycleMu sync.Mutex
	accumulateWatchdogCancel      context.CancelFunc
)

func startAccumulateWatchdog(now time.Time) {
	accumulateWatchdogLifecycleMu.Lock()
	defer accumulateWatchdogLifecycleMu.Unlock()
	if accumulateWatchdogCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	accumulateWatchdogCancel = cancel
	go newAccumulateWatchdog(now).run(ctx, accumulateWatchdogInterval)
}

func stopAccumulateWatchdog() {
	accumulateWatchdogLifecycleMu.Lock()
	cancel := accumulateWatchdogCancel
	accumulateWatchdogCancel = nil
	accumulateWatchdogLifecycleMu.Unlock()
	if cancel != nil {
		cancel()
	}
}
