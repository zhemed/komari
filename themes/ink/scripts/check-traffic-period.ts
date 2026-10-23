/* eslint-disable no-lone-blocks, node/prefer-global/process */
/**
 * 流量口径分支的最小自检脚本（本仓库没有测试框架，故用独立脚本而非 bun test/vitest）。
 *
 * 运行：bun scripts/check-traffic-period.ts
 * 退出码：0 = 全部通过；1 = 有断言失败
 *
 * 只验证「取哪个数字」：配置了周期（traffic_reset_day>=1 且 period_started_at 有效）⇒ 周期值；
 * 否则回落累计值；周期已配置但未开始 ⇒ 按 0。此版本不含任何界面文案断言。
 */
import { combineTraffic, resolveTrafficUsage } from '../src/utils/trafficPeriod'

const GIB = 1024 ** 3
const LIMIT_1TIB = 1024 * GIB

let failures = 0
let checks = 0

function ok(name: string, condition: boolean, detail = ''): void {
  checks += 1
  if (condition) {
    console.log(`  ✓ ${name}${detail ? ` ｜ ${detail}` : ''}`)
    return
  }
  failures += 1
  console.error(`  ✗ ${name}${detail ? ` ｜ ${detail}` : ''}`)
}

function approx(a: number, b: number, tolerance = 1e-6): boolean {
  return Math.abs(a - b) <= tolerance
}

console.log('① 配置了周期：用周期值（每月归零重新计）')
{
  const usage = resolveTrafficUsage(
    { traffic_reset_day: 22, traffic_reset_timezone: 'Asia/Shanghai', traffic_limit: LIMIT_1TIB, traffic_limit_type: 'sum' },
    {
      net_total_up: 274_600_000_000,
      net_total_down: 274_560_000_000,
      net_period_up: 2_200_000_000,
      net_period_down: 2_240_000_000,
      period_started_at: '2026-10-21T16:00:00Z',
    },
  )
  ok('periodMode/basis = period', usage.periodMode && usage.basis === 'period')
  ok('已用 = 周期上下行之和', approx(usage.usedBytes, 4_440_000_000), `${(usage.usedBytes / 1e9).toFixed(3)} GB`)
  ok('百分比按周期 ≈ 0.404%', approx(usage.percentage, 4_440_000_000 / LIMIT_1TIB * 100, 1e-9), `${usage.percentage.toFixed(3)}%`)
  ok('同时保留累计值（供原有「总流量」等展示）', approx(usage.totalBytes, 549_160_000_000), `${(usage.totalBytes / 1e9).toFixed(2)} GB`)
  ok('上下行分项取周期值', usage.usedUp === 2_200_000_000 && usage.usedDown === 2_240_000_000)
}

console.log('② 未配置周期：回落累计值（与改动前完全一致）')
{
  const usage = resolveTrafficUsage(
    { traffic_limit: LIMIT_1TIB, traffic_limit_type: 'sum' },
    { net_total_up: 274_600_000_000, net_total_down: 274_560_000_000 },
  )
  ok('periodMode = false / basis = cumulative', !usage.periodMode && usage.basis === 'cumulative')
  ok('已用 = 累计之和', approx(usage.usedBytes, 549_160_000_000))
  ok('百分比按累计 ≈ 49.946%', approx(usage.percentage, 549_160_000_000 / LIMIT_1TIB * 100, 1e-9), `${usage.percentage.toFixed(3)}%`)
}

console.log('③ 周期已配置但 period_started_at 无效 ⇒ 按 0（不拿累计顶替）')
{
  for (const bad of [null, '', '   ', 'not-a-date', undefined]) {
    const usage = resolveTrafficUsage(
      { traffic_reset_day: 22, traffic_reset_timezone: 'UTC', traffic_limit: LIMIT_1TIB, traffic_limit_type: 'sum' },
      { net_total_up: 500e9, net_total_down: 40e9, net_period_up: 1e9, net_period_down: 1e9, period_started_at: bad as any },
    )
    ok(`started_at=${JSON.stringify(bad)} ⇒ 0 且标记未开始`, usage.usedBytes === 0 && !usage.periodStarted && usage.percentage === 0)
  }
}

console.log('④ NaN / 负数 / Infinity 污染不产生异常展示')
{
  const usage = resolveTrafficUsage(
    { traffic_reset_day: 5, traffic_reset_timezone: 'UTC', traffic_limit: Number.NaN, traffic_limit_type: 'sum' },
    { net_total_up: Number.NaN, net_total_down: -1, net_period_up: Number.POSITIVE_INFINITY, net_period_down: 10, period_started_at: '2026-10-01T00:00:00Z' },
  )
  ok('限额 NaN ⇒ 百分比 0', usage.percentage === 0)
  ok('周期值仅保留有限正值', usage.usedUp === 0 && usage.usedDown === 10, `used=${usage.usedBytes}`)
}

console.log('⑤ limit_type 组合口径')
{
  ok('up', combineTraffic(3, 7, 'up') === 3)
  ok('down', combineTraffic(3, 7, 'down') === 7)
  ok('min', combineTraffic(3, 7, 'min') === 3)
  ok('max', combineTraffic(3, 7, 'max') === 7)
  ok('sum', combineTraffic(3, 7, 'sum') === 10)
  ok('未知类型回落 sum', combineTraffic(3, 7, 'bogus') === 10)
}

console.log('')
if (failures > 0) {
  console.error(`✗ 失败 ${failures} / ${checks} 项`)
  process.exit(1)
}
console.log(`✓ 全部通过（${checks} 项断言）`)
