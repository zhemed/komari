package clients

import (
	"fmt"
	"sync"
	"time"

	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	logger "github.com/komari-monitor/komari/utils/log"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TrafficTotal 是一个节点的累计流量（字节，跨重启只增不减）。
//
// Period* / NeedsRebaseline 是与累计值**并列**的服务端周期状态（不改变累计语义）：
// 节点未配置重置日（Client.TrafficResetDay = 0）时周期字段保持零值。
type TrafficTotal struct {
	Up   int64
	Down int64
	// PeriodUp / PeriodDown 是本周期用量；PeriodStart 为零值表示"该节点没有周期"。
	PeriodUp    int64
	PeriodDown  int64
	PeriodStart time.Time
	// NeedsRebaseline 为真时下一次上报只更新基线（见 AccumulateTraffic）。
	NeedsRebaseline bool
}

var (
	trafficMu     sync.RWMutex
	trafficTotals = map[string]TrafficTotal{}
)

// InitTrafficTotals 把持久累计与周期状态读进内存。写路径（AccumulateTraffic）会同步更新内存，
// 因此面板读取不需要每次都查库。由 internal/server 在 metric store 就绪后调用。
//
// 注意：周期与累计状态在内存里有快照（本函数启动时载入）。直接改库后，**下一次上报会重新读库
// 并回写快照**，此时面板与库一致；若该节点不再上报，则必须重启服务端才生效
// （见 docs/MAINTAINING.md §3.7）。
func InitTrafficTotals() error {
	db := dbcore.GetDBInstance()
	var rows []models.ClientTrafficTotal
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	cache := make(map[string]TrafficTotal, len(rows))
	for _, r := range rows {
		entry := TrafficTotal{
			Up:              r.UpTotal,
			Down:            r.DownTotal,
			PeriodUp:        r.PeriodUp,
			PeriodDown:      r.PeriodDown,
			NeedsRebaseline: r.NeedsRebaseline,
		}
		if r.PeriodStartedAt != nil {
			entry.PeriodStart = r.PeriodStartedAt.UTC()
		}
		cache[r.UUID] = entry
	}
	trafficMu.Lock()
	trafficTotals = cache
	trafficMu.Unlock()
	return nil
}

// AccumulateTraffic 把一次上报的流量增量累加到持久累计上，并维护服务端定义的流量周期。
//
// totalUp/totalDown 是该次上报的**计数器原值**（agent 的开机以来计数器）：
// 首次见到该节点时用它们做基线，此时 deltaUp/deltaDown 已经包含在基线里，不再重复累加；
// 之后只累加重置感知的增量（由 metricstore 算好，计数器归零时增量不会变成负数），
// 所以机器重启、agent 重启、服务器重启都不会让累计回退。
//
// 周期规则（节点配置了 Client.TrafficResetDay ∈ 1..31 时启用，见 period.go）：
//   - 首次建行：周期从 max(周期边界, 启用时刻) 起算，本次增量不计入（与累计的基线一致）；
//   - 常规：增量同时加到累计与本周期；
//   - now 已越过下一边界：本周期归零、起点前移到该边界，并写审计日志；
//   - NeedsRebaseline：本次只更新计数器基线（delta 视为 0）——agent 上报口径切换时
//     "周期值 → 开机计数器"的跳变不能被当成增量灌进累计；
//   - **只有跨周期边界才归零周期值**：计数器归零（机器/agent 重启）不归零周期值。
//
// resetDay = 0（默认）⇒ 周期字段不更新，行为与引入周期概念之前完全一致。
func AccumulateTraffic(uuid string, totalUp, totalDown, deltaUp, deltaDown int64) error {
	if uuid == "" {
		return nil
	}
	now := time.Now().UTC()
	db := dbcore.GetDBInstance()

	created := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&models.ClientTrafficTotal{
		UUID:      uuid,
		UpTotal:   totalUp,
		DownTotal: totalDown,
		UpdatedAt: now,
	})
	if created.Error != nil {
		return created.Error
	}
	if created.RowsAffected > 0 {
		// 新建成功 = 这是首次见到该节点，基线已经写入，本次增量不再重复加。
		// 周期同样只建基线：起点取 max(周期边界, 现在)，本次增量不计入。
		entry := TrafficTotal{Up: totalUp, Down: totalDown}
		if _, _, state := periodConfigFor(db, uuid); state == periodConfigEnabled {
			// PeriodWindow 返回的是"包含 now 的那个周期"的起点，按定义不会晚于 now
			// （顺延出来的未来边界是 next，不是 start），所以 max(boundary, now) 恒等于 now：
			// 即"周期从启用时刻起算"。
			start := now
			updates := map[string]any{
				"period_up":         0,
				"period_down":       0,
				"period_started_at": start,
				"needs_rebaseline":  false,
				"updated_at":        now,
			}
			if err := db.Model(&models.ClientTrafficTotal{}).Where("uuid = ?", uuid).Updates(updates).Error; err != nil {
				return err
			}
			entry.PeriodStart = start
		}
		trafficMu.Lock()
		trafficTotals[uuid] = entry
		trafficMu.Unlock()
		return nil
	}

	// 已存在的行：先取配置（启用/禁用/非法三态），再读当前周期状态——
	// 直接从库里读，避免内存与库不一致时误判边界。
	resetDay, tzName, periodState := periodConfigFor(db, uuid)

	var row models.ClientTrafficTotal
	if err := db.Where("uuid = ?", uuid).First(&row).Error; err != nil {
		return err
	}
	cur := TrafficTotal{
		Up:              row.UpTotal,
		Down:            row.DownTotal,
		PeriodUp:        row.PeriodUp,
		PeriodDown:      row.PeriodDown,
		NeedsRebaseline: row.NeedsRebaseline,
	}
	if row.PeriodStartedAt != nil {
		cur.PeriodStart = row.PeriodStartedAt.UTC()
	}

	resetNow := false
	// periodStartDirty 表示"周期起点被本次上报改动了，必须落库"，避免内存与库不一致。
	periodStartDirty := false
	if periodState == periodConfigInvalid {
		// 库里配置非法（越界重置日/非法时区；正常保存路径会拦住，只可能来自外部改库）：
		// 本次**只跳过周期记账**：既不返回错误（否则每次上报都会失败、流量不再累加），
		// 也不清空周期列（外部写坏配置不该销毁已有周期数据），累计照常增长。留 WARN 便于发现。
		logger.Warn("clients", "invalid traffic period config, period bookkeeping skipped for this report",
			"uuid", uuid, "reset_day", resetDay, "timezone", tzName)
	}
	if periodState == periodConfigEnabled {
		// 判断"是否跨过周期边界"必须拿**已记录的周期起点**去比，不能直接用
		// PeriodWindow(now).next——那个窗口按定义一定包含 now，next 永远晚于 now，
		// 于是永远不会归零。这里改判"已记录的周期起点是否还是 now 所在周期的起点"：
		// 更早 ⇒ 至少跨过了一个完整边界，本周期从 now 所在周期的边界重新起算。
		start, _, err := PeriodWindow(now, resetDay, tzName)
		if err != nil {
			// 兜底：periodConfigFor 已用同一套校验拦过，正常走不到这里。
			logger.Warn("clients", "invalid traffic period config, period bookkeeping skipped for this report",
				"uuid", uuid, "reset_day", resetDay, "timezone", tzName, "error", err)
			periodState = periodConfigInvalid
		} else if cur.PeriodStart.IsZero() {
			// 老节点（周期功能上线前就有累计行）刚启用周期配置：行里没有周期起点。
			// 补成"现在所处周期的真实边界"——**不置 resetNow**：没有跨过任何边界，
			// 就不该有重置事件；周期值本来就从 0 开始累加，这里只是把起点补上。
			// （若写成零值时间，下次上报会把它判成"很久以前的起点"而误报一次重置。）
			cur.PeriodStart = start
			periodStartDirty = true
		} else if cur.PeriodStart.Before(start) {
			// 越过（或正好踩到）周期边界 ⇒ 本周期归零，起点前移到该边界。
			// 用边界而不是 now：这样周期起点始终是一个真实的边界时刻。
			cur.PeriodUp = 0
			cur.PeriodDown = 0
			cur.PeriodStart = start
			periodStartDirty = true
			resetNow = true
		}
	}

	deltaUp, deltaDown = effectiveDelta(cur, deltaUp, deltaDown)

	if resetNow {
		auditlog.Log("", "", fmt.Sprintf(
			"traffic period reset:%s period_start=%s timezone=%s reset_day=%d period_up=%d period_down=%d",
			uuid, cur.PeriodStart.Format(time.RFC3339), periodTimezoneLabel(tzName), resetDay, cur.PeriodUp, cur.PeriodDown,
		), "info")
	}

	// 累计口径完全不变：增量照旧累加（再基线时增量为 0）。
	updates := map[string]any{
		"up_total":   gorm.Expr("up_total + ?", deltaUp),
		"down_total": gorm.Expr("down_total + ?", deltaDown),
		"updated_at": now,
	}
	switch periodState {
	case periodConfigEnabled:
		updates["period_up"] = cur.PeriodUp + deltaUp
		updates["period_down"] = cur.PeriodDown + deltaDown
		if periodStartDirty {
			updates["period_started_at"] = cur.PeriodStart
		}
	case periodConfigDisabled:
		// 明确禁用（resetDay = 0）：把周期列清空，保持"禁用 ⇒ 周期字段为空"的不变量。
		// 这不影响累计语义（up_total/down_total 照旧累加），也让内存与库一致。
		updates["period_up"] = int64(0)
		updates["period_down"] = int64(0)
		updates["period_started_at"] = nil
	case periodConfigInvalid:
		// 库里配置非法：本次**不动周期列**，保留现值（既不记账也不销毁数据）。
	}
	if cur.NeedsRebaseline {
		// 口径切换：只更新基线，本次增量丢弃（delta 已被 effectiveDelta 置 0）。
		updates["needs_rebaseline"] = false
		cur.NeedsRebaseline = false
	}

	if err := db.Model(&models.ClientTrafficTotal{}).Where("uuid = ?", uuid).Updates(updates).Error; err != nil {
		return err
	}

	cur.Up += deltaUp
	cur.Down += deltaDown
	switch periodState {
	case periodConfigEnabled:
		cur.PeriodUp += deltaUp
		cur.PeriodDown += deltaDown
	case periodConfigDisabled:
		// 禁用周期：内存里的周期字段保持"没有周期"的零值，与库里的空值一致。
		cur.PeriodUp = 0
		cur.PeriodDown = 0
		cur.PeriodStart = time.Time{}
	case periodConfigInvalid:
		// 跳过记账：内存保留从库里读回的现值（cur 已是库中值），否则面板会读到 0。
	}
	trafficMu.Lock()
	trafficTotals[uuid] = cur
	trafficMu.Unlock()
	return nil
}

// effectiveDelta 按再基线规则处理本次增量：需要再基线时本次不累加任何增量。
func effectiveDelta(cur TrafficTotal, deltaUp, deltaDown int64) (int64, int64) {
	if cur.NeedsRebaseline {
		return 0, 0
	}
	return deltaUp, deltaDown
}

// periodConfigFor 读取节点的流量周期配置并判定其运行期状态（见 periodConfigState）。
//
//	节点不存在 / TrafficResetDay = 0 ⇒ periodConfigDisabled
//	配置合法（1..31 + 可加载的时区）⇒ periodConfigEnabled
//	越界重置日或非法时区（外部改库）⇒ periodConfigInvalid
//
// 每次上报多一条按主键的 SELECT（SQLite 单连接）：不引入新的全局缓存，
// 因此没有新的可变状态竞争，也不会出现"直接改库后缓存陈旧"的问题。
func periodConfigFor(db *gorm.DB, uuid string) (resetDay int, tzName string, state periodConfigState) {
	var client models.Client
	if err := db.Select("traffic_reset_day", "traffic_reset_timezone").
		Where("uuid = ?", uuid).First(&client).Error; err != nil {
		return 0, "", periodConfigDisabled
	}
	normalizedTZ, err := ValidatePeriodConfig(client.TrafficResetDay, client.TrafficResetTimezone)
	if err != nil {
		return client.TrafficResetDay, client.TrafficResetTimezone, periodConfigInvalid
	}
	if client.TrafficResetDay == 0 {
		return 0, "", periodConfigDisabled
	}
	return client.TrafficResetDay, normalizedTZ, periodConfigEnabled
}

// periodTimezoneLabel 是审计日志里展示用的时区名（空 ⇒ 显式标出实际生效的 UTC）。
func periodTimezoneLabel(tzName string) string {
	if tzName == "" {
		return DefaultPeriodTimezone
	}
	return tzName
}

// GetTrafficTotal 返回某节点的累计流量与周期状态；不存在时 ok=false（调用方应回退到实时计数器）。
func GetTrafficTotal(uuid string) (TrafficTotal, bool) {
	trafficMu.RLock()
	defer trafficMu.RUnlock()
	t, ok := trafficTotals[uuid]
	return t, ok
}

// GetAllTrafficTotals 返回累计流量与周期状态的快照（面板批量取数用；返回副本，调用方可随意改动）。
func GetAllTrafficTotals() map[string]TrafficTotal {
	trafficMu.RLock()
	defer trafficMu.RUnlock()
	out := make(map[string]TrafficTotal, len(trafficTotals))
	for k, v := range trafficTotals {
		out[k] = v
	}
	return out
}

// MarkTrafficRebaseline 要求该节点的下一次上报只更新计数器基线、不累加增量。
//
// 调用时机：节点流量周期配置发生变化（含从禁用改为启用、或反过来）之后——
// agent 上报口径切换会让上报值跳变，跳变不能被当成流量。
// 配置变化后至少一次上报会丢失增量（最多损失一个上报周期），这是刻意的取舍：
// 比起把"周期值 → 开机计数器"的跳变灌进累计，少算一次更安全。
//
// 只更新已有行：还没有任何累计行的节点，首次上报本来就是"只建基线、不累加"，
// 这里若插一行零值占位，反而会把首次上报的基线覆盖成 0（累计值从 0 起跳）。
func MarkTrafficRebaseline(uuid string) error {
	if uuid == "" {
		return fmt.Errorf("invalid client UUID")
	}
	db := dbcore.GetDBInstance()
	result := db.Model(&models.ClientTrafficTotal{}).Where("uuid = ?", uuid).
		Update("needs_rebaseline", true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// 还没有累计行（节点从未上报过）：首次上报本来就是"只建基线、不累加"，
		// 报错会让配置保存看起来失败，因此这里直接成功返回。
		return nil
	}
	trafficMu.Lock()
	entry := trafficTotals[uuid]
	entry.NeedsRebaseline = true
	trafficTotals[uuid] = entry
	trafficMu.Unlock()
	return nil
}

// DeleteTrafficTotal 在删除节点时清掉它的累计流量（避免残留孤儿行）。
func DeleteTrafficTotal(uuid string) error {
	if uuid == "" {
		return nil
	}
	if err := dbcore.GetDBInstance().Delete(&models.ClientTrafficTotal{}, "uuid = ?", uuid).Error; err != nil {
		return err
	}
	trafficMu.Lock()
	delete(trafficTotals, uuid)
	trafficMu.Unlock()
	return nil
}
