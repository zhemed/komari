/* eslint-disable no-lone-blocks, node/prefer-global/process */
/**
 * 流量口径分支的最小自检脚本（本仓库没有测试框架，故用独立脚本而非 bun test/vitest）。
 *
 * 运行：bun scripts/check-traffic-period.ts
 * 退出码：0 = 全部通过；1 = 有断言失败
 *
 * 覆盖两种关键数据形态：
 *   ① 配置了周期（traffic_reset_day=22 + Asia/Shanghai）且有周期值 ⇒ 卡片应为周期口径；
 *   ② 未配置周期 ⇒ 回落累计口径（与改动前行为一致）。
 * 另覆盖边界：周期尚未开始、NaN/负数/Infinity 污染、限额缺失、limit_type 组合。
 */
import { formatResetNote, getTrafficPeriodText } from '../src/constants/trafficPeriodText'
import { combineTraffic, resolveTrafficUsage } from '../src/utils/trafficPeriod'

const GIB = 1024 ** 3
const LIMIT_1TIB = 1024 * GIB // 1 TiB = 1099.51 GB

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

console.log('① 周期形态：traffic_reset_day=22 + Asia/Shanghai，周期值很小、累计很大')
{
  const usage = resolveTrafficUsage(
    {
      traffic_reset_day: 22,
      traffic_reset_timezone: 'Asia/Shanghai',
      traffic_limit: LIMIT_1TIB,
      traffic_limit_type: 'sum',
    },
    {
      net_total_up: 274_600_000_000,
      net_total_down: 274_560_000_000,
      net_period_up: 2_200_000_000,
      net_period_down: 2_240_000_000,
      period_started_at: '2026-10-21T16:00:00Z',
    },
  )
  ok('判定为周期口径', usage.periodMode && usage.basis === 'period')
  ok('周期已开始', usage.periodStarted)
  ok('主数值 = 周期上下行之和', approx(usage.usedBytes, 4_440_000_000), `${(usage.usedBytes / 1e9).toFixed(3)} GB`)
  ok('百分比 ≈ 0.40%（非累计的 ~49.9%）', approx(usage.percentage, 4_440_000_000 / LIMIT_1TIB * 100, 1e-9), `${usage.percentage.toFixed(3)}%`)
  ok('同时给出累计（历史）值', approx(usage.totalBytes, 549_160_000_000), `${(usage.totalBytes / 1e9).toFixed(2)} GB`)
  ok('累计未超额 ⇒ 不显示「口径已切换」', usage.basisSwitched === false)
  ok('重置文案正确', formatResetNote(usage.resetDay, usage.resetTimezone) === '每月 22 日 00:00（Asia/Shanghai）重置', formatResetNote(usage.resetDay, usage.resetTimezone))
}

console.log('①-b 周期形态 + 累计已超额（应提示口径已切换）')
{
  const usage = resolveTrafficUsage(
    { traffic_reset_day: 22, traffic_reset_timezone: 'Asia/Shanghai', traffic_limit: 100 * GIB, traffic_limit_type: 'sum' },
    { net_total_up: 80 * GIB, net_total_down: 80 * GIB, net_period_up: 1 * GIB, net_period_down: 1 * GIB, period_started_at: '2026-10-21T16:00:00Z' },
  )
  ok('baseSwitched = true', usage.basisSwitched === true)
  ok('百分比按周期 ≈ 2%', approx(usage.percentage, 2, 1e-9), `${usage.percentage.toFixed(2)}%`)
}

console.log('② 无周期配置：回落累计口径（与改动前一致）')
{
  const usage = resolveTrafficUsage(
    { traffic_limit: LIMIT_1TIB, traffic_limit_type: 'sum' },
    { net_total_up: 274_600_000_000, net_total_down: 274_560_000_000 },
  )
  ok('periodMode = false ｜ basis = cumulative', !usage.periodMode && usage.basis === 'cumulative')
  ok('主数值 = 累计之和', approx(usage.usedBytes, 549_160_000_000))
  ok('百分比 ≈ 49.95%', approx(usage.percentage, 549_160_000_000 / LIMIT_1TIB * 100, 1e-9), `${usage.percentage.toFixed(3)}%`)
  ok('无重置文案', formatResetNote(usage.resetDay, usage.resetTimezone) === '')
}

console.log('③ 边界：周期已配置但 period_started_at 无效 ⇒ 按 0（不拿累计顶替）')
{
  for (const bad of [null, '', '   ', 'not-a-date', undefined]) {
    const usage = resolveTrafficUsage(
      { traffic_reset_day: 22, traffic_reset_timezone: 'UTC', traffic_limit: LIMIT_1TIB, traffic_limit_type: 'sum' },
      { net_total_up: 500e9, net_total_down: 40e9, net_period_up: 1e9, net_period_down: 1e9, period_started_at: bad as any },
    )
    ok(`started_at=${JSON.stringify(bad)} ⇒ usedBytes=0 且标记未开始`, usage.usedBytes === 0 && !usage.periodStarted && usage.percentage === 0)
  }
}

console.log('④ 边界：NaN / 负数 / Infinity 污染不产生 NaN 展示')
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

console.log('⑥ 双语文案表可用')
{
  const zh = getTrafficPeriodText('zh-CN')
  const en = getTrafficPeriodText('en')
  ok('zh-CN 含「本周期」「累计（历史）」', zh.period === '本周期' && zh.cumulative === '累计（历史）')
  ok('en 含周期/累计文案', en.period.length > 0 && en.cumulative.includes('Lifetime'))
  ok('未知语言回落 zh-CN', getTrafficPeriodText('xx-YY').period === '本周期')
}

console.log('')
if (failures > 0) {
  console.error(`✗ 失败 ${failures} / ${checks} 项`)
  process.exit(1)
}
console.log(`✓ 全部通过（${checks} 项断言）`)
