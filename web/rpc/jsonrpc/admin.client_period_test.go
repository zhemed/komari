package jsonrpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/clients"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/protocol/v1"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

// P3 配置接口用例（design.md §7 第二行的"不落库"部分 + §6 接口契约）。
// 用独立 in-memory SQLite，不碰生产库。

func setupClientPeriodTest(t *testing.T) string {
	t.Helper()
	suffix := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	flags.DatabaseType = flags.DatabaseTypeSQLite
	flags.DatabaseFile = "file:jsonrpc_period_" + suffix + "?mode=memory&cache=shared"
	db := dbcore.GetDBInstance()
	if err := db.AutoMigrate(&models.Client{}, &models.ClientTrafficTotal{}, &models.Log{}); err != nil {
		t.Fatalf("migrate test tables: %v", err)
	}
	if err := clients.InitTrafficTotals(); err != nil {
		t.Fatalf("init traffic totals: %v", err)
	}
	uuid := "node-" + suffix
	client := models.Client{UUID: uuid, Token: "token-" + suffix, Name: "period-rpc-test"}
	if err := db.Create(&client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	return uuid
}

// newEditRequest 走 JSON 编解码，保证数值以 float64 形态进入 handler（与真实请求一致）。
func newEditRequest(t *testing.T, payload map[string]any) *rpc.JsonRpcRequest {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	var params map[string]any
	if err := json.Unmarshal(raw, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	return &rpc.JsonRpcRequest{Version: "2.0", Method: "editClient", Params: params, ID: "1"}
}

func savedClient(t *testing.T, uuid string) models.Client {
	t.Helper()
	var client models.Client
	if err := dbcore.GetDBInstance().Where("uuid = ?", uuid).First(&client).Error; err != nil {
		t.Fatalf("load client: %v", err)
	}
	return client
}

// 非法值必须被拒（InvalidParams）且**不落库**（PRD 验收：0/32/-1/非法时区）。
func TestAdminEditClientRejectsInvalidTrafficPeriod(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
	}{
		{name: "reset day -1", payload: map[string]any{"traffic_reset_day": -1}},
		{name: "reset day 32", payload: map[string]any{"traffic_reset_day": 32}},
		{name: "reset day 999", payload: map[string]any{"traffic_reset_day": 999}},
		{name: "reset day fractional", payload: map[string]any{"traffic_reset_day": 1.5}},
		{name: "reset day string", payload: map[string]any{"traffic_reset_day": "5"}},
		{name: "invalid timezone", payload: map[string]any{"traffic_reset_day": 5, "traffic_reset_timezone": "Not/AZone"}},
		{name: "timezone with wrong type", payload: map[string]any{"traffic_reset_day": 5, "traffic_reset_timezone": 8}},
		// day=0（禁用）却带非空时区：自相矛盾的请求，拒绝（时区在禁用态没有意义，
		// 否则库里会留下一个永远不会生效的时区）。
		{name: "reset day 0 with timezone", payload: map[string]any{"traffic_reset_day": 0, "traffic_reset_timezone": "Asia/Shanghai"}},
		{name: "reset day 0 with garbage timezone", payload: map[string]any{"traffic_reset_day": 0, "traffic_reset_timezone": "Mars/Olympus"}},
		// 只给时区、不给重置日：库里现值是禁用 ⇒ 同样拒绝（不能仅靠时区启用周期）。
		{name: "timezone only while disabled", payload: map[string]any{"traffic_reset_timezone": "Asia/Shanghai"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			uuid := setupClientPeriodTest(t)
			payload := map[string]any{"uuid": uuid, "name": "renamed"}
			for k, v := range test.payload {
				payload[k] = v
			}

			result, rpcErr := adminEditClient(context.Background(), newEditRequest(t, payload))
			if rpcErr == nil {
				t.Fatalf("expected error, got result=%v", result)
			}
			if rpcErr.Code != rpc.InvalidParams {
				t.Fatalf("error code = %d (%s), want InvalidParams(%d)", rpcErr.Code, rpcErr.Message, rpc.InvalidParams)
			}
			if rpcErr.Message == "" {
				t.Fatal("error message must be explicit")
			}

			// 校验失败 ⇒ 整条更新都不落库（不只是周期字段）。
			client := savedClient(t, uuid)
			if client.TrafficResetDay != 0 || client.TrafficResetTimezone != "" {
				t.Fatalf("invalid config was persisted: day=%d tz=%q", client.TrafficResetDay, client.TrafficResetTimezone)
			}
			if client.Name != "period-rpc-test" {
				t.Fatalf("name = %q, want unchanged (no partial write)", client.Name)
			}
		})
	}
}

// 合法值：保存成功、归一化落库（空时区 ⇒ UTC）；只给一个字段时另一个保持现值。
func TestAdminEditClientAcceptsValidTrafficPeriod(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
		wantDay int
		wantTZ  string
	}{
		{name: "day with timezone", payload: map[string]any{"traffic_reset_day": 15, "traffic_reset_timezone": "Asia/Shanghai"}, wantDay: 15, wantTZ: "Asia/Shanghai"},
		{name: "day 1 boundary", payload: map[string]any{"traffic_reset_day": 1, "traffic_reset_timezone": "UTC"}, wantDay: 1, wantTZ: "UTC"},
		{name: "day 31 boundary", payload: map[string]any{"traffic_reset_day": 31, "traffic_reset_timezone": "Asia/Shanghai"}, wantDay: 31, wantTZ: "Asia/Shanghai"},
		{name: "empty timezone normalizes to UTC", payload: map[string]any{"traffic_reset_day": 20, "traffic_reset_timezone": ""}, wantDay: 20, wantTZ: "UTC"},
		{name: "day 0 disables", payload: map[string]any{"traffic_reset_day": 0}, wantDay: 0, wantTZ: ""},
		{name: "day 0 with empty timezone disables", payload: map[string]any{"traffic_reset_day": 0, "traffic_reset_timezone": ""}, wantDay: 0, wantTZ: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			uuid := setupClientPeriodTest(t)
			payload := map[string]any{"uuid": uuid}
			for k, v := range test.payload {
				payload[k] = v
			}
			if _, rpcErr := adminEditClient(context.Background(), newEditRequest(t, payload)); rpcErr != nil {
				t.Fatalf("unexpected error: %+v", rpcErr)
			}
			client := savedClient(t, uuid)
			if client.TrafficResetDay != test.wantDay || client.TrafficResetTimezone != test.wantTZ {
				t.Fatalf("saved day/tz = (%d,%q), want (%d,%q)",
					client.TrafficResetDay, client.TrafficResetTimezone, test.wantDay, test.wantTZ)
			}
		})
	}
}

// 部分更新：已启用周期的节点只改时区（或只改重置日）时，另一个字段保持现值。
func TestAdminEditClientPartialTrafficPeriodUpdate(t *testing.T) {
	t.Run("timezone only keeps reset day", func(t *testing.T) {
		uuid := setupClientPeriodTest(t)
		if _, rpcErr := adminEditClient(context.Background(), newEditRequest(t, map[string]any{
			"uuid": uuid, "traffic_reset_day": 15, "traffic_reset_timezone": "UTC",
		})); rpcErr != nil {
			t.Fatalf("configure period: %+v", rpcErr)
		}
		if _, rpcErr := adminEditClient(context.Background(), newEditRequest(t, map[string]any{
			"uuid": uuid, "traffic_reset_timezone": "Asia/Shanghai",
		})); rpcErr != nil {
			t.Fatalf("update timezone only: %+v", rpcErr)
		}
		client := savedClient(t, uuid)
		if client.TrafficResetDay != 15 || client.TrafficResetTimezone != "Asia/Shanghai" {
			t.Fatalf("saved day/tz = (%d,%q), want (15,%q)",
				client.TrafficResetDay, client.TrafficResetTimezone, "Asia/Shanghai")
		}
	})

	t.Run("reset day only keeps timezone", func(t *testing.T) {
		uuid := setupClientPeriodTest(t)
		if _, rpcErr := adminEditClient(context.Background(), newEditRequest(t, map[string]any{
			"uuid": uuid, "traffic_reset_day": 15, "traffic_reset_timezone": "Asia/Shanghai",
		})); rpcErr != nil {
			t.Fatalf("configure period: %+v", rpcErr)
		}
		if _, rpcErr := adminEditClient(context.Background(), newEditRequest(t, map[string]any{
			"uuid": uuid, "traffic_reset_day": 28,
		})); rpcErr != nil {
			t.Fatalf("update day only: %+v", rpcErr)
		}
		client := savedClient(t, uuid)
		if client.TrafficResetDay != 28 || client.TrafficResetTimezone != "Asia/Shanghai" {
			t.Fatalf("saved day/tz = (%d,%q), want (28,%q)",
				client.TrafficResetDay, client.TrafficResetTimezone, "Asia/Shanghai")
		}
	})
}

// 保存周期配置 ⇒ 置 NeedsRebaseline 并写审计日志（design.md §5 / §6）。
func TestAdminEditClientMarksRebaselineAndAudits(t *testing.T) {
	uuid := setupClientPeriodTest(t)

	// 先造一条累计行，模拟该节点已经在正常上报。
	if err := clients.AccumulateTraffic(uuid, 1_000, 2_000, 0, 0); err != nil {
		t.Fatalf("seed traffic: %v", err)
	}
	if _, rpcErr := adminEditClient(context.Background(), newEditRequest(t, map[string]any{
		"uuid": uuid, "traffic_reset_day": 5, "traffic_reset_timezone": "Asia/Shanghai",
	})); rpcErr != nil {
		t.Fatalf("edit client: %+v", rpcErr)
	}

	total, ok := clients.GetTrafficTotal(uuid)
	if !ok {
		t.Fatal("traffic total disappeared after edit")
	}
	if !total.NeedsRebaseline {
		t.Fatal("NeedsRebaseline not set after saving traffic period config")
	}

	var entries []models.Log
	if err := dbcore.GetDBInstance().Where("msg_type = ?", "info").Find(&entries).Error; err != nil {
		t.Fatalf("load audit log: %v", err)
	}
	found := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Message, "edit client traffic period:"+uuid) {
			found = true
			if !strings.Contains(entry.Message, "day=5") || !strings.Contains(entry.Message, "tz=Asia/Shanghai") {
				t.Fatalf("audit message missing config details: %q", entry.Message)
			}
		}
	}
	if !found {
		t.Fatalf("no traffic period audit entry in %#v", entries)
	}

	// 再基线生效：下一次上报只做基线，累计不跳变。
	if err := clients.AccumulateTraffic(uuid, 9_999, 8_888, 8_999, 6_888); err != nil {
		t.Fatalf("accumulate after rebaseline: %v", err)
	}
	total, _ = clients.GetTrafficTotal(uuid)
	if total.Up != 1_000 || total.Down != 2_000 {
		t.Fatalf("cumulative = (%d,%d), want unchanged (1000,2000)", total.Up, total.Down)
	}
	if total.NeedsRebaseline {
		t.Fatal("NeedsRebaseline should be consumed by the first report")
	}
}

// 不带周期字段的编辑不触发再基线（保持既有行为）。
func TestAdminEditClientWithoutPeriodFieldsDoesNotRebaseline(t *testing.T) {
	uuid := setupClientPeriodTest(t)
	if err := clients.AccumulateTraffic(uuid, 1_000, 2_000, 0, 0); err != nil {
		t.Fatalf("seed traffic: %v", err)
	}
	if _, rpcErr := adminEditClient(context.Background(), newEditRequest(t, map[string]any{
		"uuid": uuid, "name": "just-a-rename",
	})); rpcErr != nil {
		t.Fatalf("edit client: %+v", rpcErr)
	}
	total, _ := clients.GetTrafficTotal(uuid)
	if total.NeedsRebaseline {
		t.Fatal("rename must not trigger a rebaseline")
	}
	if client := savedClient(t, uuid); client.Name != "just-a-rename" {
		t.Fatalf("name = %q, want renamed", client.Name)
	}
}

// getNodesLatestStatus 的流量契约：net_total_up/down 保持累计口径不变，
// 新增的 net_period_up/down 是"本周期用量"（PRD 需求 7：面板展示不变）。
func TestGetNodesLatestStatusAppendsPeriodFields(t *testing.T) {
	uuid := setupClientPeriodTest(t)
	// 配置周期并累计：累计 1_000/2_000，其中本周期 400/700 + 本次 100/200。
	if _, rpcErr := adminEditClient(context.Background(), newEditRequest(t, map[string]any{
		"uuid": uuid, "traffic_reset_day": 15, "traffic_reset_timezone": "Asia/Shanghai",
	})); rpcErr != nil {
		t.Fatalf("configure period: %+v", rpcErr)
	}
	if err := clients.AccumulateTraffic(uuid, 1_000, 2_000, 0, 0); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	if err := clients.AccumulateTraffic(uuid, 1_100, 2_300, 100, 300); err != nil {
		t.Fatalf("accumulate: %v", err)
	}

	agent_runtime.RecordReport(v1.Report{UUID: uuid})
	t.Cleanup(func() { agent_runtime.DeleteLatestReport(uuid) })

	// getNodesLatestStatus 要读 rpc.ContextMeta（判定 admin/hidden），
	// 直接传 context.Background() 会 nil 解引用；这里按真实调用链补上 meta。
	ctx := rpc.NewContextWithMeta(context.Background(), &rpc.ContextMeta{
		Principal:  rpc.PrincipalFromRole(rpc.RoleAdmin),
		Permission: rpc.RoleAdmin,
	})
	raw, rpcErr := getNodesLatestStatus(ctx, newEditRequest(t, map[string]any{"uuid": uuid}))
	if rpcErr != nil {
		t.Fatalf("getNodesLatestStatus: %+v", rpcErr)
	}

	// 走一次 JSON 编解码，断言的是**对外契约**（json tag），不是内部结构体。
	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}

	// net_total_up/down 语义不变：仍取持久累计。
	if got := decoded["net_total_up"]; got != float64(1_100) {
		t.Fatalf("net_total_up = %v, want 1100（累计口径不变）", got)
	}
	if got := decoded["net_total_down"]; got != float64(2_300) {
		t.Fatalf("net_total_down = %v, want 2300（累计口径不变）", got)
	}
	// 本周期用量：基线那次不计入周期，第二次上报的 100/300 才计入。
	if got := decoded["net_period_up"]; got != float64(100) {
		t.Fatalf("net_period_up = %v, want 100", got)
	}
	if got := decoded["net_period_down"]; got != float64(300) {
		t.Fatalf("net_period_down = %v, want 300", got)
	}
	if _, ok := decoded["period_started_at"]; !ok {
		t.Fatal("period_started_at should be present for a period-enabled node")
	}
}
