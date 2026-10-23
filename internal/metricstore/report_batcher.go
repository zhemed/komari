package metricstore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	logger "github.com/komari-monitor/komari/utils/log"

	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/metric"
	v1 "github.com/komari-monitor/komari/protocol/v1"
)

type reportTrafficState struct {
	mu sync.Mutex

	reportTrafficValues
}

type reportTrafficValues struct {
	initialized bool
	timestamp   time.Time
	hasUp       bool
	totalUp     int64
	hasDown     bool
	totalDown   int64
	hasUptime   bool
	uptime      int64
}

var reportTrafficStates sync.Map

// trafficAccumulator 由上层（internal/server）注册：把重置感知的流量增量交给持久累计。
// metricstore 刻意不 import database/*，保持分层；未注册时（例如单元测试）什么也不做。
//
// 回调返回 error 只是为了**计数**（心跳里的 accumulate_errors_total）：
// "累加全都在报错"与"累加根本没被调用（静默停摆）"是两种故障，必须能分开看。
// 日志仍由注册方（internal/server）负责，这里不重复记录。
var (
	trafficAccumulatorMu sync.RWMutex
	trafficAccumulator   func(uuid string, totalUp, totalDown, deltaUp, deltaDown int64) error
)

// SetTrafficAccumulator 注册流量累计回调；传 nil 注销。
func SetTrafficAccumulator(fn func(uuid string, totalUp, totalDown, deltaUp, deltaDown int64) error) {
	trafficAccumulatorMu.Lock()
	trafficAccumulator = fn
	trafficAccumulatorMu.Unlock()
}

func currentTrafficAccumulator() func(string, int64, int64, int64, int64) error {
	trafficAccumulatorMu.RLock()
	defer trafficAccumulatorMu.RUnlock()
	return trafficAccumulator
}

const (
	reportBatchInterval     = 3 * time.Second
	pingBatchMaxRecords     = 512
	reportBatchWriteTimeout = 10 * time.Second
	// maxReportBatchAttempts 是"同一队头批次"允许的连续失败次数：达到后进死信（丢弃）。
	// 取 3：单次失败往往是瞬时抖动（锁竞争、瞬时 IO），重试 2 次足够；
	// 再失败的批次极可能是毒丸（数据/驱动层面的确定性失败），继续重试只会永久堵住后面的报告。
	maxReportBatchAttempts = 3
)

var (
	// reportBatchQueueSize 是报告队列容量。取 var 是为了让测试能缩小规模后覆写
	// （生产默认值不变：4096）。
	reportBatchQueueSize = 4096
	// maxPendingReports 是"已出队但未写成"的缓冲上限：2 × 队列容量。
	// 每条报告数百字节量级 ⇒ 上限对应几 MB 内存，同时留出约两个 flush 周期（3s）的余量；
	// 超出时丢**最旧**的（策略理由见 trimPendingReports）。
	maxPendingReports = reportBatchQueueSize * 2
)

var (
	reportBatcherMu sync.Mutex
	reportBatcher   *reportBatchWorker
)

var (
	ErrReportBatchQueueFull = errors.New("metric report batch queue is full")
	ErrReportBatchStopped   = errors.New("metric report batcher is stopped")
	ErrPingBatchQueueFull   = errors.New("ping batch queue is full")
)

type reportBatchRequest struct {
	ctx  context.Context
	done chan error
	stop bool
}

type reportBatchWorker struct {
	mu        sync.Mutex
	queue     chan v1.Report
	pingQueue chan models.PingRecord
	requests  chan reportBatchRequest
	done      chan struct{}
	stopping  bool
	// pendingLen 是"已出队但未写成"的报告数（心跳暴露，见 heartbeat.go）。
	pendingLen int
}

// StartReportBatcher starts the shared report writer. Exact samples are kept in
// the short raw window while the same writes update in-memory minute buckets.
func StartReportBatcher() {
	reportBatcherMu.Lock()
	defer reportBatcherMu.Unlock()
	if reportBatcher != nil {
		return
	}
	worker := &reportBatchWorker{
		queue:     make(chan v1.Report, reportBatchQueueSize),
		pingQueue: make(chan models.PingRecord, reportBatchQueueSize),
		requests:  make(chan reportBatchRequest, 1),
		done:      make(chan struct{}),
	}
	reportBatcher = worker
	go worker.run()
	// 看门狗与 worker 同生命周期：只在批处理真的跑着的时候才有"停摆"可言。
	startAccumulateWatchdog(time.Now().UTC())
}

// StopReportBatcher stops the report writer after flushing all queued reports.
func StopReportBatcher(ctx context.Context) error {
	reportBatcherMu.Lock()
	worker := reportBatcher
	if worker == nil {
		reportBatcherMu.Unlock()
		return nil
	}
	worker.mu.Lock()
	worker.stopping = true
	worker.mu.Unlock()
	reportBatcherMu.Unlock()

	request := reportBatchRequest{ctx: ctx, done: make(chan error, 1), stop: true}
	select {
	case worker.requests <- request:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-request.done:
		<-worker.done
		reportBatcherMu.Lock()
		if reportBatcher == worker {
			reportBatcher = nil
		}
		reportBatcherMu.Unlock()
		// worker 已退出 ⇒ 不存在"批处理停摆"可言，看门狗一并停掉，避免停机期间误报。
		stopAccumulateWatchdog()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// FlushReportBatch synchronously flushes the current queue. It is useful for
// controlled handoff points and deterministic tests; normal operation uses the
// worker ticker and the active mode's flush interval.
func FlushReportBatch(ctx context.Context) error {
	reportBatcherMu.Lock()
	worker := reportBatcher
	reportBatcherMu.Unlock()
	if worker == nil {
		return nil
	}
	request := reportBatchRequest{ctx: ctx, done: make(chan error, 1)}
	worker.mu.Lock()
	stopping := worker.stopping
	worker.mu.Unlock()
	if stopping {
		return ErrReportBatchStopped
	}
	select {
	case worker.requests <- request:
	case <-worker.done:
		return ErrReportBatchStopped
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-request.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WriteReport persists one agent report and adds it to in-memory minute
// summaries using the same server receive time. Traffic deltas remain summable
// after rollup.
func WriteReport(ctx context.Context, report v1.Report) (v1.Report, error) {
	if report.UUID == "" {
		return v1.Report{}, fmt.Errorf("report UUID is required")
	}
	if report.UpdatedAt.IsZero() {
		return v1.Report{}, fmt.Errorf("report receive time is required")
	}
	report.UpdatedAt = report.UpdatedAt.UTC()
	if GetStore() == nil {
		return v1.Report{}, fmt.Errorf("metric store not enabled")
	}

	reportBatcherMu.Lock()
	worker := reportBatcher
	reportBatcherMu.Unlock()
	if worker != nil {
		if err := worker.enqueue(ctx, report); err != nil {
			return v1.Report{}, err
		}
		return report, nil
	}

	saved, err := writeReportBatch(ctx, []v1.Report{report})
	if err != nil {
		return v1.Report{}, err
	}
	return saved[0], nil
}

func (w *reportBatchWorker) enqueue(ctx context.Context, report v1.Report) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopping {
		return ErrReportBatchStopped
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case w.queue <- report:
		return nil
	default:
		return ErrReportBatchQueueFull
	}
}

func (w *reportBatchWorker) run() {
	ticker := time.NewTicker(reportBatchInterval)
	defer ticker.Stop()

	var pending []v1.Report
	var pendingPings []models.PingRecord
	retry := &reportBatchRetry{}
	defer w.setPendingLen(0)
	for {
		select {
		case request := <-w.requests:
			pending = append(pending, drainReportQueue(w.queue, reportBatchQueueSize)...)
			pending = trimPendingReports(pending, maxPendingReports)
			w.setPendingLen(len(pending))
			pendingPings = append(pendingPings, drainPingQueue(w.pingQueue, reportBatchQueueSize)...)
			err := errors.Join(
				writePendingReports(request.ctx, &pending, retry),
				writePendingPingRecords(request.ctx, &pendingPings),
			)
			w.setPendingLen(len(pending))
			if request.stop {
				if err != nil {
					logger.Errorf("metricstore", "failed to flush metric report batch during shutdown: %v", err)
				}
				close(w.done)
				request.done <- err
				return
			}
			request.done <- err
		case <-ticker.C:
			pendingPings = append(pendingPings, drainPingQueue(w.pingQueue, reportBatchQueueSize)...)
			if err := writePendingPingRecords(context.Background(), &pendingPings); err != nil {
				logger.Errorf("metricstore", "failed to flush ping batch: %v", err)
			}
			pending = append(pending, drainReportQueue(w.queue, reportBatchQueueSize)...)
			pending = trimPendingReports(pending, maxPendingReports)
			w.setPendingLen(len(pending))
			if err := writePendingReports(context.Background(), &pending, retry); err != nil {
				logger.Errorf("metricstore", "failed to flush metric report batch: %v", err)
			}
			w.setPendingLen(len(pending))
		}
	}
}

// setPendingLen 发布"已出队但未写成"的报告数（心跳用，与 stopping 共用一把锁）。
func (w *reportBatchWorker) setPendingLen(length int) {
	w.mu.Lock()
	w.pendingLen = length
	w.mu.Unlock()
}

// pendingReportsLen 读取已发布的值（心跳快照用）。
func (w *reportBatchWorker) pendingReportsLen() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.pendingLen
}

func (w *reportBatchWorker) enqueuePing(ctx context.Context, record models.PingRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopping {
		return ErrReportBatchStopped
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case w.pingQueue <- record:
		return nil
	default:
		return ErrPingBatchQueueFull
	}
}

func drainReportQueue(queue <-chan v1.Report, limit int) []v1.Report {
	if limit <= 0 {
		return nil
	}
	capacity := len(queue)
	if capacity > limit {
		capacity = limit
	}
	if capacity == 0 {
		return nil
	}
	reports := make([]v1.Report, 0, capacity)
	for len(reports) < limit {
		select {
		case report := <-queue:
			reports = append(reports, report)
		default:
			return reports
		}
	}
	return reports
}

func drainPingQueue(queue <-chan models.PingRecord, limit int) []models.PingRecord {
	if limit <= 0 {
		return nil
	}
	capacity := len(queue)
	if capacity > limit {
		capacity = limit
	}
	if capacity == 0 {
		return nil
	}
	records := make([]models.PingRecord, 0, capacity)
	for len(records) < limit {
		select {
		case record := <-queue:
			records = append(records, record)
		default:
			return records
		}
	}
	return records
}

// reportBatchRetry 保存"当前 pending 队头批次"的连续失败次数（跨 tick 保留）。
type reportBatchRetry struct {
	attempts int
}

func (r *reportBatchRetry) reset() { r.attempts = 0 }

// writePendingReports 把 pending 里的报告按批写库。
//
// 健壮性（缺陷 1）：失败不再"原地无限重试同一前缀"——那样一个毒丸批次会永久堵住
// 它后面所有报告，而 pending 又无上限。现在的策略：
//
//   - 同一队头批次连续失败 maxReportBatchAttempts 次 ⇒ 进**死信**：丢弃该批次、
//     计数、写一条含批次摘要的 ERROR，然后继续写后面的报告（队头阻塞被解开）；
//   - 任何一批写成功就把重试计数清零（正常路径行为完全不变）；
//   - 失败但未达上限时仍然返回错误，让调用方照旧记 ERROR（保持"吵闹"）。
func writePendingReports(ctx context.Context, pending *[]v1.Report, retry *reportBatchRetry) error {
	if len(*pending) == 0 {
		retry.reset()
		return nil
	}
	batchSize := len(*pending)
	for len(*pending) > 0 {
		if batchSize > len(*pending) {
			batchSize = len(*pending)
		}
		writeCtx, cancel := context.WithTimeout(ctx, reportBatchWriteTimeout)
		_, err := writeReportBatch(writeCtx, (*pending)[:batchSize])
		cancel()
		if err != nil {
			// 批处理失败统一在这里计数：无论失败来自信号量等待、store 缺失还是写库，
			// 都算一次"批次没完成"，与"累计回调报错"分开统计。
			recordBatchError(err)
			retry.attempts++
			if retry.attempts < maxReportBatchAttempts {
				return err
			}
			deadLetterReportBatch((*pending)[:batchSize], err)
			*pending = (*pending)[batchSize:]
			retry.reset()
			continue
		}
		retry.reset()
		*pending = (*pending)[batchSize:]
	}
	return nil
}

// deadLetterReportBatch 丢弃一个"重试到上限仍失败"的批次：计数 + ERROR（含批次摘要），
// 让后面的报告能继续写出去。报告本身已在上报时被 agent 视为送达，这里只能丢弃并留痕。
func deadLetterReportBatch(reports []v1.Report, cause error) {
	recordDroppedReportBatch(len(reports))
	logger.Errorf("metricstore", "dropping metric report batch after %d failed attempts: %s (last error: %v)",
		maxReportBatchAttempts, summarizeReports(reports), cause)
}

// summarizeReports 生成批次摘要：条数、涉及节点数、时间窗与少量 uuid 样本。
// 只用于日志，不包含任何地址/端口等敏感信息。
func summarizeReports(reports []v1.Report) string {
	if len(reports) == 0 {
		return "empty batch"
	}
	uuids := make(map[string]struct{}, len(reports))
	earliest := reports[0].UpdatedAt
	latest := reports[0].UpdatedAt
	sample := make([]string, 0, reportSummarySampleSize)
	for _, report := range reports {
		uuids[report.UUID] = struct{}{}
		if report.UpdatedAt.Before(earliest) {
			earliest = report.UpdatedAt
		}
		if report.UpdatedAt.After(latest) {
			latest = report.UpdatedAt
		}
		if len(sample) < reportSummarySampleSize {
			sample = append(sample, report.UUID)
		}
	}
	return fmt.Sprintf("reports=%d nodes=%d window=[%s..%s] sample_uuids=%v",
		len(reports), len(uuids), earliest.UTC().Format(time.RFC3339), latest.UTC().Format(time.RFC3339), sample)
}

// reportSummarySampleSize 是批次摘要里附带的 uuid 样本数（足够定位问题，又不刷屏）。
const reportSummarySampleSize = 3

// trimPendingReports 把 pending 截到上限；超出时丢弃**最旧**的并计数 + 告警。
//
// 为什么丢最旧而不是拒绝最新：报告在 WriteReport 时已经对 agent 应答成功（不能事后拒绝），
// 而时序数据里"最新"的信息价值更高；同时队头往往正是卡住的那一批，
// 丢最旧也顺带把队头阻塞解开。返回新切片，避免旧数组继续被引用而不释放。
func trimPendingReports(pending []v1.Report, limit int) []v1.Report {
	if limit <= 0 || len(pending) <= limit {
		return pending
	}
	dropped := len(pending) - limit
	recordDroppedPendingReports(dropped)
	logger.Warn("metricstore", "dropping oldest pending metric reports to keep the buffer bounded",
		"dropped", dropped, "pending_len", len(pending), "pending_limit", limit)
	return append([]v1.Report(nil), pending[dropped:]...)
}

func writePendingPingRecords(ctx context.Context, pending *[]models.PingRecord) error {
	for len(*pending) > 0 {
		batchSize := len(*pending)
		if batchSize > pingBatchMaxRecords {
			batchSize = pingBatchMaxRecords
		}
		writeCtx, cancel := context.WithTimeout(ctx, reportBatchWriteTimeout)
		err := writePingRecords(writeCtx, (*pending)[:batchSize])
		cancel()
		if err != nil {
			return err
		}
		*pending = (*pending)[batchSize:]
	}
	return nil
}

// trafficAccumulation 是"这一次上报要累加多少"的中间结果。
type trafficAccumulation struct {
	uuid      string
	totalUp   int64
	totalDown int64
	deltaUp   int64
	deltaDown int64
}

func writeReportBatch(ctx context.Context, reports []v1.Report) ([]v1.Report, error) {
	if len(reports) == 0 {
		return nil, nil
	}
	if err := storeOperations.AcquireShared(ctx); err != nil {
		return nil, fmt.Errorf("wait for metric store operation before writing reports: %w", err)
	}
	defer storeOperations.ReleaseShared()

	s := GetStore()
	if s == nil {
		return nil, fmt.Errorf("metric store not enabled")
	}

	prepared := make([]v1.Report, len(reports))
	copy(prepared, reports)
	points := make([]metric.Point, 0, len(reports)*20)
	pendingStates := make(map[*reportTrafficState]reportTrafficValues)
	accumulations := make([]trafficAccumulation, 0, len(prepared))
	for i, report := range prepared {
		stateValue, _ := reportTrafficStates.LoadOrStore(report.UUID, &reportTrafficState{})
		state := stateValue.(*reportTrafficState)
		values, ok := pendingStates[state]
		if !ok {
			state.mu.Lock()
			values = state.reportTrafficValues
			state.mu.Unlock()
		}
		if !values.initialized {
			totalUp, hasUp, err := latestReportCounter(ctx, s, MetricNetTotalUp, report.UUID, report.UpdatedAt)
			if err == nil {
				values.totalUp = totalUp
				values.hasUp = hasUp
			} else if ctx.Err() == nil {
				logger.Errorf("metricstore", "failed to restore previous upload counter for %s: %v", report.UUID, err)
			}
			totalDown, hasDown, err := latestReportCounter(ctx, s, MetricNetTotalDown, report.UUID, report.UpdatedAt)
			if err == nil {
				values.totalDown = totalDown
				values.hasDown = hasDown
			} else if ctx.Err() == nil {
				logger.Errorf("metricstore", "failed to restore previous download counter for %s: %v", report.UUID, err)
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			values.initialized = true
		}

		if !values.timestamp.IsZero() && !report.UpdatedAt.After(values.timestamp) {
			report.UpdatedAt = values.timestamp.Add(time.Millisecond)
		}
		// 仅当两次上报都带有效的开机时长时才把"回退"当成 agent 重启：
		// v2 协议的报告没有 uptime 字段（服务端读到 0），拿这个 0 与 v1 报告的有效值比较，
		// 会在通道切换时（v2 整体失败降级到 v1、之后重连回 v2，见 agent/server/websocket.go
		// 的 runPostFallback/runV2PullLoop）被误判成 agent 重启，那条上报的流量增量被置 0。
		// 注意：agent 同一时刻只走一条通道，本地部署也从未观测到降级，因此**没有证据**表明
		// 这个缺陷曾在实测中触发；保留判据是防御性的正确性修复，不要据此声称它解释过任何实测现象。
		agentRestart := values.hasUptime && report.Uptime > 0 && values.uptime > 0 && report.Uptime < values.uptime
		counterResetUp := values.hasUp && report.Network.TotalUp < values.totalUp
		counterResetDown := values.hasDown && report.Network.TotalDown < values.totalDown
		trafficUp := int64(0)
		if values.hasUp && !agentRestart && !counterResetUp {
			trafficUp = TrafficCounterDelta(report.Network.TotalUp, values.totalUp)
		}
		trafficDown := int64(0)
		if values.hasDown && !agentRestart && !counterResetDown {
			trafficDown = TrafficCounterDelta(report.Network.TotalDown, values.totalDown)
		}
		points = append(points, reportMetricPoints(report, trafficUp, trafficDown)...)
		accumulations = append(accumulations, trafficAccumulation{
			uuid:      report.UUID,
			totalUp:   report.Network.TotalUp,
			totalDown: report.Network.TotalDown,
			deltaUp:   trafficUp,
			deltaDown: trafficDown,
		})
		values.timestamp = report.UpdatedAt
		values.hasUp = true
		values.totalUp = report.Network.TotalUp
		values.hasDown = true
		values.totalDown = report.Network.TotalDown
		values.hasUptime = true
		values.uptime = report.Uptime
		pendingStates[state] = values
		prepared[i] = report
	}

	if err := s.WriteBatch(ctx, points); err != nil {
		return nil, err
	}
	recordBatchSuccess(time.Now().UTC())
	// 指标写成功之后再把增量交给持久累计（写失败就不该计流量）。
	if accumulate := currentTrafficAccumulator(); accumulate != nil {
		accumulated := 0
		for _, a := range accumulations {
			if err := accumulate(a.uuid, a.totalUp, a.totalDown, a.deltaUp, a.deltaDown); err != nil {
				// 只计数，不在这里记日志（注册方 internal/server 已经记过，避免重复刷屏）。
				recordAccumulateError()
				continue
			}
			accumulated++
		}
		// 至少一次成功才算"累加心跳"：全是错误时心跳应当保持陈旧，
		// 由看门狗按"队列积压 + 心跳陈旧"报停摆，错误计数另行解释原因。
		if accumulated > 0 {
			recordAccumulateSuccess(time.Now().UTC())
		}
	}
	for state, values := range pendingStates {
		state.mu.Lock()
		state.reportTrafficValues = values
		state.mu.Unlock()
	}
	return prepared, nil
}
