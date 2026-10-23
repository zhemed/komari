package metricstore

import (
	"sync"
	"time"
)

// 报告写入流水线的心跳与计数。
//
// 为什么需要：生产实测过一次"累计停摆"——`client_traffic_totals.updated_at` 冻结 150 秒，
// 同时指标库仍在正常写入、日志无 error/warn/panic、把周期配置回退为禁用也不恢复，
// 只有重启服务端才恢复。结构性事实见 report_batcher.go：批处理是**单 goroutine 串行**，
// 且流量累计回调在指标写成功之后的**同一个 goroutine**里 ⇒ "指标仍在写"与"批处理卡住"互斥。
//
// 因此只看"有没有报错"无法区分三种情况：
//
//	① 没人上报（正常空闲）；② 上报在写、累计在跑（健康）；③ 队列有积压但累计不动（停摆）。
//
// 心跳把①③切开（看时间戳），错误计数把"报错"与"静默卡住"切开（见 accumulate_watchdog.go）。

// TrafficPipelineHeartbeat 是报告写入流水线的心跳快照（由管理端状态接口暴露）。
type TrafficPipelineHeartbeat struct {
	// LastBatchSuccessUnix / LastAccumulateSuccessUnix：最近一次"指标批写入成功"与
	// "流量累加回调成功"的时间（Unix 秒，UTC）。0 表示进程启动以来还没成功过。
	LastBatchSuccessUnix      int64 `json:"last_batch_success_unix"`
	LastAccumulateSuccessUnix int64 `json:"last_accumulate_success_unix"`
	// BatchErrorsTotal / AccumulateErrorsTotal：批写入失败次数与累加回调失败次数。
	// 分开计数才能区分"写不进去（报错刷屏）"与"没动静（静默停摆）"。
	BatchErrorsTotal      uint64 `json:"batch_errors_total"`
	AccumulateErrorsTotal uint64 `json:"accumulate_errors_total"`
	// QueueLen / WorkerAlive：批处理队列积压与 worker 存活，是停摆判据的并发条件。
	QueueLen    int  `json:"queue_len"`
	WorkerAlive bool `json:"worker_alive"`
	// StallEventsTotal：看门狗判定为"停摆"的事件次数（同一次停摆只 +1）。
	StallEventsTotal uint64 `json:"stall_events_total"`
	// LastBatchError：最近一次批处理错误（已截断），便于事后核对。
	LastBatchError string `json:"last_batch_error,omitempty"`
}

// lastBatchErrorMaxLen 是对外暴露的错误串长度上限：错误可能很长（含 DSN 片段等），
// 截断只影响展示，不影响日志（日志里仍由各自的调用点完整记录）。
const lastBatchErrorMaxLen = 200

var (
	heartbeatMu           sync.Mutex
	lastBatchSuccess      int64
	lastAccumulateSuccess int64
	batchErrorsTotal      uint64
	accumulateErrorsTotal uint64
	stallEventsTotal      uint64
	lastBatchErrorText    string
)

func recordBatchSuccess(now time.Time) {
	heartbeatMu.Lock()
	lastBatchSuccess = now.UTC().Unix()
	heartbeatMu.Unlock()
}

func recordBatchError(err error) {
	message := ""
	if err != nil {
		message = truncateError(err.Error(), lastBatchErrorMaxLen)
	}
	heartbeatMu.Lock()
	batchErrorsTotal++
	lastBatchErrorText = message
	heartbeatMu.Unlock()
}

func recordAccumulateSuccess(now time.Time) {
	heartbeatMu.Lock()
	lastAccumulateSuccess = now.UTC().Unix()
	heartbeatMu.Unlock()
}

func recordAccumulateError() {
	heartbeatMu.Lock()
	accumulateErrorsTotal++
	heartbeatMu.Unlock()
}

func recordAccumulateStall() {
	heartbeatMu.Lock()
	stallEventsTotal++
	heartbeatMu.Unlock()
}

// TrafficPipelineStatus 组装对外心跳快照。
// queue_len / worker_alive 取实时值（来自批处理 worker），其余取计数器的快照。
func TrafficPipelineStatus() TrafficPipelineHeartbeat {
	heartbeatMu.Lock()
	heartbeat := TrafficPipelineHeartbeat{
		LastBatchSuccessUnix:      lastBatchSuccess,
		LastAccumulateSuccessUnix: lastAccumulateSuccess,
		BatchErrorsTotal:          batchErrorsTotal,
		AccumulateErrorsTotal:     accumulateErrorsTotal,
		StallEventsTotal:          stallEventsTotal,
		LastBatchError:            lastBatchErrorText,
	}
	heartbeatMu.Unlock()

	reportBatcherMu.Lock()
	worker := reportBatcher
	reportBatcherMu.Unlock()
	if worker != nil {
		heartbeat.QueueLen = len(worker.queue)
		heartbeat.WorkerAlive = reportWorkerAlive(worker)
	}
	return heartbeat
}

// reportWorkerAlive 以"worker.done 是否已关闭"判定存活：
// run() 退出前一定会 close(done)（正常停机或 panic 后的 defer 都不会漏），
// 因此它比"goroutine 数量"之类的近似判据更可靠。
func reportWorkerAlive(worker *reportBatchWorker) bool {
	select {
	case <-worker.done:
		return false
	default:
		return true
	}
}

// truncateError 按字节截断错误串；截断时附上省略号，避免看起来像完整错误。
func truncateError(message string, limit int) string {
	if limit <= 0 || len(message) <= limit {
		return message
	}
	const ellipsis = "..."
	if limit <= len(ellipsis) {
		return message[:limit]
	}
	return message[:limit-len(ellipsis)] + ellipsis
}
