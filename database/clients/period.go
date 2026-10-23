package clients

import (
	"fmt"
	"time"
)

// 服务端流量周期：按节点配置的「重置日 + 时区」自行计算周期边界。
//
// 为什么在服务端算：agent 侧的 `--month-rotate` 把上报值变成"本周期用量"，
// 服务端把它当单调计数器累加，跨重置日的下降会被判成"计数器归零"⇒ 增量 0
// ⇒ 累计只增不减（排查见 .trellis/tasks/archive/2026-10/10-23-traffic-month-rotate）。
// 方案 B 让 agent 回报单调计数器，周期口径由服务端定义。
//
// 边界语义与 agent 既有实现对齐（agent/utils/utils.go 的 getActualResetDate）：
// 当月有该日 ⇒ 该日 00:00（配置时区）；当月无该日（如 31 日遇小月）⇒ 顺延次月 1 日 00:00。

// DefaultPeriodTimezone 是没有配置时区时使用的时区。
//
// 显式用 UTC，而不是服务器本地时区：节点配置要与服务器所在时区解耦，
// 否则迁移服务器（或容器 TZ 变化）会让所有节点的周期边界跟着漂移。
const DefaultPeriodTimezone = "UTC"

// ValidatePeriodConfig 校验节点级流量周期配置。
//
//	resetDay == 0 ⇒ 禁用周期，此时不做任何校验（时区被忽略，归一化为空）。
//	resetDay 为 1..31 ⇒ 周期启用；tzName 允许为空（= UTC），否则必须是可加载的 IANA 名称。
//
// 返回归一化后的时区名（空 ⇒ DefaultPeriodTimezone），供调用方落库。
// 越界与非法值一律返回错误，**不做静默降级**——这是本功能的硬要求
// （agent 侧 GetLastResetDate 越界返回 currentDate 导致区间恒空，是反面教材）。
func ValidatePeriodConfig(resetDay int, tzName string) (string, error) {
	if resetDay < 0 || resetDay > 31 {
		return "", fmt.Errorf("traffic_reset_day must be 0 (disabled) or 1..31, got %d", resetDay)
	}
	if resetDay == 0 {
		return "", nil
	}
	if tzName == "" {
		return DefaultPeriodTimezone, nil
	}
	if _, err := time.LoadLocation(tzName); err != nil {
		return "", fmt.Errorf("traffic_reset_timezone must be a valid IANA timezone name (e.g. Asia/Shanghai, UTC): %w", err)
	}
	return tzName, nil
}

// PeriodWindow 返回包含 now 的周期边界：start 是当前周期起点，next 是下一个周期起点。
//
//	resetDay：1..31（每月重置日）——越界返回错误；`0` 由调用方解释为"禁用"，不进入本函数。
//	tzName：  IANA 名称；空字符串 ⇒ UTC；非法名称返回错误。
//
// 返回值都转换为 UTC 存储/比较；日志与展示按配置时区还原。
func PeriodWindow(now time.Time, resetDay int, tzName string) (start, next time.Time, err error) {
	if resetDay < 1 || resetDay > 31 {
		return time.Time{}, time.Time{}, fmt.Errorf("traffic_reset_day must be 1..31, got %d", resetDay)
	}
	loc, err := loadPeriodLocation(tzName)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	startLocal, nextLocal := periodWindowIn(now.In(loc), resetDay)
	return startLocal.UTC(), nextLocal.UTC(), nil
}

// loadPeriodLocation 解析配置时区；空 ⇒ UTC。非法 IANA 名称在这里返回错误。
//
// 依赖 Go 的时区数据库：容器/静态二进制若没有系统 zoneinfo，靠 main 包的
// `import _ "time/tzdata"` 兜底（见 main.go），否则这里会失败、节点配置存不进去。
func loadPeriodLocation(tzName string) (*time.Location, error) {
	if tzName == "" {
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, fmt.Errorf("load timezone %q: %w", tzName, err)
	}
	return loc, nil
}

// periodWindowIn 是纯函数形式的周期窗口（传入的时刻必须已经在目标时区里，
// 这样单测可以完全不依赖 time.Local 与服务器时区）。
func periodWindowIn(nowLocal time.Time, resetDay int) (start, next time.Time) {
	loc := nowLocal.Location()
	// 本月边界按"顺延"规则落位：当月有该日 ⇒ 该日 00:00；没有 ⇒ 次月 1 日 00:00。
	thisMonth := resetBoundary(nowLocal.Year(), nowLocal.Month(), resetDay, loc)
	if !nowLocal.Before(thisMonth) {
		return thisMonth, advanceBoundary(nowLocal, resetDay, loc)
	}
	// now 在本月边界之前（典型：resetDay 大于今天）⇒ 当前周期起点在上一个月。
	// 月份加减一律从"当月 1 日"出发，避免 AddDate 在 29/30/31 日上被规范化到别的月份。
	prev := monthStart(nowLocal).AddDate(0, -1, 0)
	return resetBoundary(prev.Year(), prev.Month(), resetDay, loc), thisMonth
}

// resetBoundary 返回 loc 时区下 year/month 这个月的重置时刻
// （该月 resetDay 日 00:00；该月没有那一天则顺延次月 1 日 00:00）。
func resetBoundary(year int, month time.Month, resetDay int, loc *time.Location) time.Time {
	firstOfNext := time.Date(year, month, 1, 0, 0, 0, 0, loc).AddDate(0, 1, 0)
	if resetDay <= firstOfNext.AddDate(0, 0, -1).Day() {
		return time.Date(year, month, resetDay, 0, 0, 0, 0, loc)
	}
	return firstOfNext
}

// monthStart 返回 ref 所在月份的 1 日 00:00（保留时区）。
func monthStart(ref time.Time) time.Time {
	return time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, ref.Location())
}

// advanceBoundary 返回 now 之后的第一个边界（这里假定 now 已经 >= 当前周期起点）。
//
// 边界按月推进，相邻边界最多相隔约两个月：例如 resetDay = 31、当前起点是 1 月 31 日时，
// 下一个边界是 3 月 1 日（2 月没有 31 日 ⇒ 顺延到 3 月 1 日，与 agent 语义一致；
// 注意"2 月边界顺延到 3 月 1 日"与"3 月边界 3 月 31 日"是两个不同边界）。
func advanceBoundary(nowLocal time.Time, resetDay int, loc *time.Location) time.Time {
	// 先算本月边界；它若落在 now 之后，就是我们要的下一边界。
	thisMonth := resetBoundary(nowLocal.Year(), nowLocal.Month(), resetDay, loc)
	if thisMonth.After(nowLocal) {
		return thisMonth
	}
	// 本月边界已过（或正好是 now）⇒ 下一边界由"本月 1 日的下一个月"决定。
	next := monthStart(nowLocal).AddDate(0, 1, 0)
	return resetBoundary(next.Year(), next.Month(), resetDay, loc)
}
