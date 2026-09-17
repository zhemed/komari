package metricstore

import (
	"testing"

	"github.com/komari-monitor/komari/pkg/metric"
)

// 面板对 traffic.up/down 的默认聚合是 avg，会把"每次上报的字节数"再平均一次：
// 点值 = 该桶真实流量 ÷ 桶内采样条数。这个测试锁死语义默认，防止有人把 avg 又放回来。
func TestSemanticAggregationCoversByteAmountAndCounterMetrics(t *testing.T) {
	cases := []struct {
		metricName string
		want       metric.Aggregation
		wantOK     bool
	}{
		{MetricTrafficUp, metric.AggSum, true},
		{MetricTrafficDown, metric.AggSum, true},
		{MetricNetTotalUp, metric.AggLast, true},
		{MetricNetTotalDown, metric.AggLast, true},
		{MetricCPU, "", false},
		{MetricNetIn, "", false},
		{MetricRAM, "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := SemanticAggregation(c.metricName)
		if ok != c.wantOK || got != c.want {
			t.Errorf("SemanticAggregation(%q) = (%q, %v), 期望 (%q, %v)", c.metricName, got, ok, c.want, c.wantOK)
		}
	}
}

// records 路径（面板节点详情的旧记录接口）必须继续得到同一套聚合。
func TestRecordMetricAggregationFollowsSemanticDefaults(t *testing.T) {
	cases := map[string]metric.Aggregation{
		MetricTrafficUp:    metric.AggSum,
		MetricTrafficDown:  metric.AggSum,
		MetricNetTotalUp:   metric.AggLast,
		MetricNetTotalDown: metric.AggLast,
		MetricCPU:          metric.AggAvg,
		MetricRAM:          metric.AggAvg,
		MetricPingLoss:     metric.AggAvg,
	}
	for name, want := range cases {
		if got := recordMetricAggregation(name); got != want {
			t.Errorf("recordMetricAggregation(%q) = %q, 期望 %q", name, got, want)
		}
	}
}
