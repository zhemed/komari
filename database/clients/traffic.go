package clients

import (
	"sync"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TrafficTotal 是一个节点的累计流量（字节，跨重启只增不减）。
type TrafficTotal struct {
	Up   int64
	Down int64
}

var (
	trafficMu     sync.RWMutex
	trafficTotals = map[string]TrafficTotal{}
)

// InitTrafficTotals 把持久累计读进内存。写路径（AccumulateTraffic）会同步更新内存，
// 因此面板读取不需要每次都查库。由 internal/server 在 metric store 就绪后调用。
func InitTrafficTotals() error {
	db := dbcore.GetDBInstance()
	var rows []models.ClientTrafficTotal
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	cache := make(map[string]TrafficTotal, len(rows))
	for _, r := range rows {
		cache[r.UUID] = TrafficTotal{Up: r.UpTotal, Down: r.DownTotal}
	}
	trafficMu.Lock()
	trafficTotals = cache
	trafficMu.Unlock()
	return nil
}

// AccumulateTraffic 把一次上报的流量增量累加到持久累计上。
//
// totalUp/totalDown 是该次上报的**计数器原值**（agent 的开机以来计数器）：
// 首次见到该节点时用它们做基线，此时 deltaUp/deltaDown 已经包含在基线里，不再重复累加；
// 之后只累加重置感知的增量（由 metricstore 算好，计数器归零时增量不会变成负数），
// 所以机器重启、agent 重启、服务器重启都不会让累计回退。
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
		trafficMu.Lock()
		trafficTotals[uuid] = TrafficTotal{Up: totalUp, Down: totalDown}
		trafficMu.Unlock()
		return nil
	}

	if deltaUp == 0 && deltaDown == 0 {
		return nil
	}
	if err := db.Model(&models.ClientTrafficTotal{}).Where("uuid = ?", uuid).
		Updates(map[string]any{
			"up_total":   gorm.Expr("up_total + ?", deltaUp),
			"down_total": gorm.Expr("down_total + ?", deltaDown),
			"updated_at": now,
		}).Error; err != nil {
		return err
	}
	trafficMu.Lock()
	cur := trafficTotals[uuid] // 不存在时为零值，等价于从增量开始累加
	cur.Up += deltaUp
	cur.Down += deltaDown
	trafficTotals[uuid] = cur
	trafficMu.Unlock()
	return nil
}

// GetTrafficTotal 返回某节点的累计流量；不存在时 ok=false（调用方应回退到实时计数器）。
func GetTrafficTotal(uuid string) (TrafficTotal, bool) {
	trafficMu.RLock()
	defer trafficMu.RUnlock()
	t, ok := trafficTotals[uuid]
	return t, ok
}

// GetAllTrafficTotals 返回累计流量的快照（面板批量取数用；返回副本，调用方可随意改动）。
func GetAllTrafficTotals() map[string]TrafficTotal {
	trafficMu.RLock()
	defer trafficMu.RUnlock()
	out := make(map[string]TrafficTotal, len(trafficTotals))
	for k, v := range trafficTotals {
		out[k] = v
	}
	return out
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
