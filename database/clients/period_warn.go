package clients

import (
	"strconv"
	"sync"

	logger "github.com/komari-monitor/komari/utils/log"
)

// 非法周期配置的 WARN 去重。
//
// 为什么要去重：库里出现非法周期配置（外部改库所致；API 保存路径会拦住）时，
// AccumulateTraffic 每次都走 invalid 分支。若每次上报都打一条 WARN，长期非法会刷屏
// （2026-10-23 实测：临时实例连续两次上报 = 2 条）。
//
// 口径：
//   - **同一节点 + 同一配置指纹只告警一次**；
//   - 指纹（reset_day + timezone 任一）变化 ⇒ 视为新指纹 ⇒ 重新告警一次；
//   - 配置恢复合法（enabled/disabled）⇒ 清理该节点状态，以后再非法能重新告警；
//   - 节点删除（DeleteTrafficTotal，DeleteClient 走的就是这条路径）⇒ 一并清理。
//
// 有界性：状态只保存"当前处于非法的那些节点"，条目在配置恢复合法或节点被删除时删除，
// 因此上界是 clients 表的行数，**不随时间/上报次数增长**（不是每次上报攒一条）。
//
// 并发：AccumulateTraffic 会被 metricstore 的批次回调触发、可能并发进入，
// 本状态用独立的 periodWarnMu 保护，不与 trafficMu 耦合（不拖慢读路径）。

var (
	periodWarnMu sync.Mutex
	// periodWarned 记录 uuid → 已经告过警的配置指纹。
	periodWarned = map[string]string{}
)

// periodConfigFingerprint 生成配置指纹：至少含 reset_day 与 timezone，任一变化都算新指纹。
func periodConfigFingerprint(resetDay int, tzName string) string {
	return strconv.Itoa(resetDay) + "|" + tzName
}

// warnInvalidPeriodConfigOnce 对"同一节点 + 同一指纹"只告警一次；返回本次是否真的写出了日志。
//
// 日志保持既有级别/模块名与字段（uuid / reset_day / timezone / 错误信息），
// 只把"重复"的那些压掉。
func warnInvalidPeriodConfigOnce(uuid string, resetDay int, tzName string, cause error) bool {
	fingerprint := periodConfigFingerprint(resetDay, tzName)

	periodWarnMu.Lock()
	if previous, warned := periodWarned[uuid]; warned && previous == fingerprint {
		periodWarnMu.Unlock()
		return false
	}
	periodWarned[uuid] = fingerprint
	periodWarnMu.Unlock()

	logger.Warn("clients", "invalid traffic period config, period bookkeeping skipped for this report",
		"uuid", uuid, "reset_day", resetDay, "timezone", tzName, "error", cause)
	return true
}

// clearPeriodWarnState 清掉该节点的去重状态：配置恢复合法、或节点被删除时调用。
// 这是"有界"的实现落点——没有这处清理，指纹条目会一直留着（虽然仍以节点数为上界）。
func clearPeriodWarnState(uuid string) {
	periodWarnMu.Lock()
	delete(periodWarned, uuid)
	periodWarnMu.Unlock()
}
