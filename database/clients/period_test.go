package clients

import (
	"testing"
	"time"
)

// P1 边界计算用例（design.md §7 第一行）。全部用固定时刻，不依赖 time.Now()。
func TestPeriodWindowBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		now       time.Time
		resetDay  int
		tz        string
		wantStart time.Time
		wantNext  time.Time
	}{
		{
			name:      "UTC 常规日内",
			now:       time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
			resetDay:  5,
			tz:        "UTC",
			wantStart: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
			wantNext:  time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "UTC 空时区名等价 UTC",
			now:       time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
			resetDay:  5,
			tz:        "",
			wantStart: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
			wantNext:  time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "UTC 正好在边界上（含边界）",
			now:       time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
			resetDay:  5,
			tz:        "UTC",
			wantStart: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
			wantNext:  time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "UTC 边界前一刻仍属上一周期",
			now:       time.Date(2026, 3, 4, 23, 59, 59, 0, time.UTC),
			resetDay:  5,
			tz:        "UTC",
			wantStart: time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC),
			wantNext:  time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "Asia/Shanghai 本地日内",
			now:       time.Date(2026, 3, 15, 4, 0, 0, 0, time.UTC), // 本地 12:00
			resetDay:  10,
			tz:        "Asia/Shanghai",
			wantStart: time.Date(2026, 3, 9, 16, 0, 0, 0, time.UTC), // 本地 3-10 00:00
			wantNext:  time.Date(2026, 4, 9, 16, 0, 0, 0, time.UTC), // 本地 4-10 00:00
		},
		{
			name:      "Asia/Shanghai UTC 仍是前一天",
			now:       time.Date(2026, 3, 14, 20, 0, 0, 0, time.UTC), // 本地 3-15 04:00
			resetDay:  15,
			tz:        "Asia/Shanghai",
			wantStart: time.Date(2026, 3, 14, 16, 0, 0, 0, time.UTC), // 本地 3-15 00:00
			wantNext:  time.Date(2026, 4, 14, 16, 0, 0, 0, time.UTC),
		},
		{
			name:      "Asia/Shanghai 跨月边界",
			now:       time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), // 本地 2-1 08:00
			resetDay:  1,
			tz:        "Asia/Shanghai",
			wantStart: time.Date(2026, 1, 31, 16, 0, 0, 0, time.UTC), // 本地 2-1 00:00
			wantNext:  time.Date(2026, 2, 28, 16, 0, 0, 0, time.UTC), // 本地 3-1 00:00
		},
		{
			name:      "小月顺延：UTC 2 月 15 日 + 重置日 31",
			now:       time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC),
			resetDay:  31,
			tz:        "UTC",
			wantStart: time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), // 1 月有 31 日
			wantNext:  time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),  // 2 月没有 ⇒ 顺延 3-1
		},
		{
			name:      "小月顺延：UTC 3 月 1 日 + 重置日 31（本周期含顺延边界）",
			now:       time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			resetDay:  31,
			tz:        "UTC",
			wantStart: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), // 顺延边界当天即起点
			wantNext:  time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "小月顺延：UTC 3 月 15 日 + 重置日 31（本周期起点是顺延边界）",
			now:       time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
			resetDay:  31,
			tz:        "UTC",
			wantStart: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			wantNext:  time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "跨年：UTC 1 月 15 日 + 重置日 31（12 月 31 日起点，2 月无 31 日）",
			now:       time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			resetDay:  31,
			tz:        "UTC",
			wantStart: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
			wantNext:  time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), // 1 月有 31 日 ⇒ 就是本月边界
		},
		{
			name:      "跨年：UTC 12 月 31 日当天（1 月有 31 日 ⇒ 下一边界 1 月 31 日）",
			now:       time.Date(2026, 12, 31, 8, 0, 0, 0, time.UTC),
			resetDay:  31,
			tz:        "UTC",
			wantStart: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
			wantNext:  time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "跨年 + 顺延：UTC 1 月 31 日之后（2 月无 31 日 ⇒ 下一边界 3 月 1 日）",
			now:       time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC),
			resetDay:  31,
			tz:        "UTC",
			wantStart: time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC),
			wantNext:  time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "跨年：UTC 1 月 31 日之后（下一边界顺延到 3 月 1 日）",
			now:       time.Date(2026, 2, 1, 0, 30, 0, 0, time.UTC),
			resetDay:  31,
			tz:        "UTC",
			wantStart: time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
			wantNext:  time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "夏令时：America/New_York 3 月（边界取本地 00:00）",
			now:       time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
			resetDay:  10,
			tz:        "America/New_York", // 3-10 已进入 EDT（UTC-4）
			wantStart: time.Date(2026, 3, 10, 4, 0, 0, 0, time.UTC),
			wantNext:  time.Date(2026, 4, 10, 4, 0, 0, 0, time.UTC),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start, next, err := PeriodWindow(test.now, test.resetDay, test.tz)
			if err != nil {
				t.Fatalf("PeriodWindow(%s, %d, %q) error: %v", test.now, test.resetDay, test.tz, err)
			}
			if !start.Equal(test.wantStart) {
				t.Fatalf("start = %s, want %s", start.UTC(), test.wantStart.UTC())
			}
			if !next.Equal(test.wantNext) {
				t.Fatalf("next = %s, want %s", next.UTC(), test.wantNext.UTC())
			}
			// 不变量：起点 <= now < 下一边界（周期窗口必须包含 now）。
			if start.After(test.now) {
				t.Fatalf("start %s is after now %s", start, test.now)
			}
			if !next.After(test.now) {
				t.Fatalf("next %s is not after now %s", next, test.now)
			}
		})
	}
}

// 非法配置必须报错，不能静默退化成"恒 0"（design.md §7 第二行）。
func TestPeriodWindowRejectsInvalidConfig(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		resetDay int
		tz       string
	}{
		{name: "resetDay 0 由调用方解释为禁用，不进入本函数", resetDay: 0, tz: "UTC"},
		{name: "resetDay 负数", resetDay: -1, tz: "UTC"},
		{name: "resetDay 越界 32", resetDay: 32, tz: "UTC"},
		{name: "非法时区", resetDay: 5, tz: "Not/AZone"},
		{name: "空时区是合法的（UTC），这里用非法时区配越界日", resetDay: 99, tz: "Not/AZone"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := PeriodWindow(now, test.resetDay, test.tz); err == nil {
				t.Fatalf("PeriodWindow(%d, %q) = nil error, want error", test.resetDay, test.tz)
			}
		})
	}
}

// ValidatePeriodConfig 是保存配置时的校验入口：合法值归一化，非法值报错。
func TestValidatePeriodConfig(t *testing.T) {
	tests := []struct {
		name     string
		resetDay int
		tz       string
		wantTZ   string
		wantErr  bool
	}{
		{name: "禁用", resetDay: 0, tz: "", wantTZ: ""},
		{name: "禁用时忽略非法时区（不落库）", resetDay: 0, tz: "Not/AZone", wantTZ: ""},
		{name: "启用 + 空时区 ⇒ UTC", resetDay: 15, tz: "", wantTZ: "UTC"},
		{name: "启用 + 显式 UTC", resetDay: 15, tz: "UTC", wantTZ: "UTC"},
		{name: "启用 + Asia/Shanghai", resetDay: 1, tz: "Asia/Shanghai", wantTZ: "Asia/Shanghai"},
		{name: "启用 + 边界 31", resetDay: 31, tz: "Asia/Shanghai", wantTZ: "Asia/Shanghai"},
		{name: "越界 -1", resetDay: -1, tz: "UTC", wantErr: true},
		{name: "越界 32", resetDay: 32, tz: "UTC", wantErr: true},
		{name: "非法时区", resetDay: 5, tz: "Not/AZone", wantErr: true},
		{name: "非法时区（带斜杠但不存在）", resetDay: 5, tz: "Mars/Olympus", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ValidatePeriodConfig(test.resetDay, test.tz)
			if test.wantErr {
				if err == nil {
					t.Fatalf("ValidatePeriodConfig(%d, %q) = %q, nil; want error", test.resetDay, test.tz, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidatePeriodConfig(%d, %q) error: %v", test.resetDay, test.tz, err)
			}
			if got != test.wantTZ {
				t.Fatalf("ValidatePeriodConfig(%d, %q) = %q, want %q", test.resetDay, test.tz, got, test.wantTZ)
			}
		})
	}
}

// tzdata 兜底回归：服务器产物是静态链接（KOMARI_STATIC=1），裸机/systemd 环境可能
// 没有 /usr/share/zoneinfo。main 包的 `import _ "time/tzdata"` 把时区库嵌进二进制；
// 如果有人删掉那行，构建出来的二进制在精简环境里会让 LoadLocation("Asia/Shanghai")
// 失败、节点配置直接存不进去。
//
// 本机（有系统 zoneinfo）这条用例无法区分"嵌入"与"读系统库"，但它至少钉住
// "Asia/Shanghai 必须可用"这一契约，属**推断**级防护；真正的验证需要在无 tzdata 的
// 容器里跑（见 implement.md 风险清单第 1 条）。
func TestPeriodWindowTimezoneDatabaseAvailable(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("time.LoadLocation(%q) failed: %v（检查 main.go 是否还有 import _ \"time/tzdata\"）", "Asia/Shanghai", err)
	}
	if _, offset := time.Date(2026, 3, 15, 12, 0, 0, 0, loc).Zone(); offset != 8*60*60 {
		t.Fatalf("Asia/Shanghai offset = %d, want 28800", offset)
	}

	now := time.Date(2026, 3, 15, 4, 0, 0, 0, time.UTC)
	start, next, err := PeriodWindow(now, 10, "Asia/Shanghai")
	if err != nil {
		t.Fatalf("PeriodWindow with embedded tzdata: %v", err)
	}
	if want := time.Date(2026, 3, 9, 16, 0, 0, 0, time.UTC); !start.Equal(want) {
		t.Fatalf("start = %s, want %s", start, want)
	}
	if want := time.Date(2026, 4, 9, 16, 0, 0, 0, time.UTC); !next.Equal(want) {
		t.Fatalf("next = %s, want %s", next, want)
	}
}
