package jsonrpc

import (
	"context"
	"encoding/json"
	"testing"
)

// 管理端状态接口必须暴露报告流水线心跳（对外 JSON 契约）。
//
// 本用例只断言"接口暴露了哪些字段"；心跳取值是否真的随健康/失败路径变化，
// 由 internal/metricstore 的 heartbeat_test.go / accumulate_watchdog_test.go 覆盖。
func TestAdminGetDatabaseSizeExposesTrafficPipelineHeartbeat(t *testing.T) {
	response, rpcErr := adminGetDatabaseSize(context.Background(), nil)
	if rpcErr != nil {
		t.Fatalf("adminGetDatabaseSize: %+v", rpcErr)
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// 既有字段不得被动过（老客户端契约）。
	for _, key := range []string{"type", "size", "main", "monitoring", "local_total"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("既有字段 %q 丢失：%s", key, encoded)
		}
	}

	pipeline, ok := decoded["traffic_pipeline"].(map[string]any)
	if !ok {
		t.Fatalf("traffic_pipeline 缺失或类型不对：%s", encoded)
	}
	for _, key := range []string{
		"last_batch_success_unix",
		"last_accumulate_success_unix",
		"batch_errors_total",
		"accumulate_errors_total",
		"queue_len",
		"worker_alive",
		"pending_len",
		"stall_events_total",
		"dropped_batches_total",
		"dropped_reports_total",
	} {
		if _, ok := pipeline[key]; !ok {
			t.Fatalf("traffic_pipeline 缺少字段 %q：%s", key, encoded)
		}
	}
	// 便于在提交/复审时直接看到真实返回形状。
	t.Logf("GET /api/admin/database/size → traffic_pipeline = %s", mustJSON(t, pipeline))
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(encoded)
}
