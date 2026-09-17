package models

import "time"

// ClientTrafficTotal 是每个节点的**跨重启流量累计**（本仓库自有扩展，上游没有这张表）。
//
// 为什么需要它：上游面板的“总流量”直接展示 agent 上报的“开机以来计数器”，
// 机器一重启网卡计数器归零，那个数字就跟着归零。我们改为在服务端按 metric store
// 已经算好的**重置感知增量**持续累加并落库，于是：
//   - agent 重启、机器重启（计数器归零）、服务器重启，累计都只增不减；
//   - 首次见到某节点时用当时的计数器做基线，避免开启该功能后数字从 0 跳变。
//
// 写入方：database/clients.AccumulateTraffic（由 metricstore 批次写入的钩子驱动）
// 读取方：web/rpc/jsonrpc 的 getNodesLatestStatus（卡片“总流量”与流量阈值进度）
type ClientTrafficTotal struct {
	UUID      string    `json:"uuid" gorm:"type:varchar(64);primaryKey"`
	UpTotal   int64     `json:"up_total" gorm:"type:bigint;not null;default:0"`
	DownTotal int64     `json:"down_total" gorm:"type:bigint;not null;default:0"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 固定表名，避免 GORM 复数化规则变化导致读写到不同表。
func (ClientTrafficTotal) TableName() string {
	return "client_traffic_totals"
}
