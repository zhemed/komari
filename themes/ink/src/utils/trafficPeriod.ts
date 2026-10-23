/**
 * 流量口径单一真源（本周期 / 累计）。
 *
 * 背景：Komari 服务端 0.0.22 起提供「服务端自算周期」能力（节点级 `traffic_reset_day` +
 * `traffic_reset_timezone`，RPC 下发 `net_period_up` / `net_period_down` / `period_started_at`），
 * 0.0.24 起管理端已切到周期口径。本主题（ink）此前所有流量展示都直接取 `net_total_up/down`
 * （累计），导致「配置了周期重置也看不到归零」。
 *
 * 本模块把口径判定收敛到一处，**所有展示流量的地方都必须走这里**，不要各算一遍：
 * - `traffic_reset_day >= 1` 且 `period_started_at` 有效 ⇒ 用周期值；
 * - 否则（未配置周期、或周期尚未开始）⇒ 回落累计值；
 * - 无论哪种口径，都同时给出「累计（历史）」值，便于并列展示。
 *
 * 语义对齐管理端参考实现（komari 仓库 `frontend/src/utils/trafficPeriod.ts#resolveTrafficUsage`）。
 */

export type TrafficLimitType = 'sum' | 'up' | 'down' | 'min' | 'max'

export interface TrafficPeriodConfig {
  /** 每月重置日：0/未配置 = 关闭；1..31 = 该日 00:00（时区内）归零 */
  traffic_reset_day?: number | null
  /** IANA 时区名，空 = UTC */
  traffic_reset_timezone?: string | null
  traffic_limit?: number | null
  traffic_limit_type?: string | null
}

export interface TrafficLiveValues {
  net_total_up?: number | null
  net_total_down?: number | null
  net_period_up?: number | null
  net_period_down?: number | null
  period_started_at?: string | null
}

export interface TrafficUsage {
  /** 是否处于周期口径（配置了重置日） */
  periodMode: boolean
  /** 周期是否已开始（period_started_at 有效） */
  periodStarted: boolean
  /** 主数值分子（按 limit_type 组合后的**当前口径**用量） */
  usedUp: number
  usedDown: number
  usedBytes: number
  /** 累计值（始终给出，用于「累计（历史）」） */
  totalUp: number
  totalDown: number
  totalBytes: number
  /** 若按累计口径计算会得到的用量（用于判断是否发生口径切换） */
  legacyUsedBytes: number
  percentage: number
  /** 累计已超额但周期未超额 ⇒ 建议展示「口径已切换」徽标 */
  basisSwitched: boolean
  resetDay: number
  resetTimezone: string
  /** 'period' = 当前展示的是周期值；'cumulative' = 展示的是累计值 */
  basis: 'period' | 'cumulative'
}

const SUPPORTED_LIMIT_TYPES: readonly TrafficLimitType[] = ['sum', 'max', 'min', 'up', 'down']
const DEFAULT_LIMIT_TYPE: TrafficLimitType = 'sum'
const DEFAULT_TIMEZONE = 'UTC'

/** 数值兜底：非有限值或负数一律按 0（防 NaN/Infinity 污染展示） */
function normalizeCounter(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : 0
}

function normalizeLimitType(value: unknown): TrafficLimitType {
  const type = typeof value === 'string' ? value : ''
  return (SUPPORTED_LIMIT_TYPES as readonly string[]).includes(type)
    ? type as TrafficLimitType
    : DEFAULT_LIMIT_TYPE
}

function isLimitTypeSupported(value: unknown): boolean {
  return typeof value === 'string' && (SUPPORTED_LIMIT_TYPES as readonly string[]).includes(value)
}

/** 周期模式：traffic_reset_day 为 >= 1 的有限数 */
export function isPeriodMode(config: TrafficPeriodConfig | null | undefined): boolean {
  const day = config?.traffic_reset_day
  return typeof day === 'number' && Number.isFinite(day) && day >= 1
}

/** period_started_at 是否为可解析的时间串（去空白后非空且可 Date.parse） */
export function isPeriodStarted(periodStartedAt: unknown): boolean {
  if (typeof periodStartedAt !== 'string')
    return false

  const trimmed = periodStartedAt.trim()
  if (trimmed === '')
    return false

  return Number.isFinite(new Date(trimmed).getTime())
}

/** 按 traffic_limit_type 组合上下行（与后端/管理端口径一致） */
export function combineTraffic(up: number, down: number, limitType?: string | null): number {
  switch (normalizeLimitType(limitType)) {
    case 'up': return up
    case 'down': return down
    case 'min': return Math.min(up, down)
    case 'max': return Math.max(up, down)
    case 'sum':
    default: return up + down
  }
}

function resolveResetTimezone(config: TrafficPeriodConfig | null | undefined): string {
  const raw = config?.traffic_reset_timezone
  const trimmed = typeof raw === 'string' ? raw.trim() : ''
  return trimmed === '' ? DEFAULT_TIMEZONE : trimmed
}

/**
 * 解析一个节点的流量口径与数值。
 *
 * @param config 节点配置（`traffic_reset_day` / `traffic_reset_timezone` / `traffic_limit` / `traffic_limit_type`）
 * @param live   实时状态（`net_total_*` / `net_period_*` / `period_started_at`）
 */
export function resolveTrafficUsage(
  config: TrafficPeriodConfig | null | undefined,
  live: TrafficLiveValues | null | undefined,
): TrafficUsage {
  const periodMode = isPeriodMode(config)
  const periodStarted = periodMode && isPeriodStarted(live?.period_started_at)
  const resetDay = periodMode ? normalizeCounter(config?.traffic_reset_day) : 0
  const resetTimezone = resolveResetTimezone(config)
  const limit = normalizeCounter(config?.traffic_limit)
  const limitType = normalizeLimitType(config?.traffic_limit_type)

  const totalUp = normalizeCounter(live?.net_total_up)
  const totalDown = normalizeCounter(live?.net_total_down)

  // 周期口径：未开始则按 0（服务端未给周期值时不能拿累计冒充周期）
  const usedUp = periodMode ? (periodStarted ? normalizeCounter(live?.net_period_up) : 0) : totalUp
  const usedDown = periodMode ? (periodStarted ? normalizeCounter(live?.net_period_down) : 0) : totalDown

  const usedBytes = combineTraffic(usedUp, usedDown, limitType)
  const totalBytes = totalUp + totalDown
  const legacyUsedBytes = combineTraffic(totalUp, totalDown, limitType)
  const percentage = limit > 0 ? Math.min(Math.max(usedBytes / limit * 100, 0), 100) : 0

  return {
    periodMode,
    periodStarted,
    usedUp,
    usedDown,
    usedBytes,
    totalUp,
    totalDown,
    totalBytes,
    legacyUsedBytes,
    percentage,
    basisSwitched: periodMode
      && isLimitTypeSupported(config?.traffic_limit_type)
      && limit > 0
      && legacyUsedBytes >= limit
      && usedBytes < limit,
    resetDay,
    resetTimezone,
    // 配置了周期即处于周期口径；若周期尚未开始，服务端语义是「按 0」而不是拿累计顶替
    basis: periodMode ? 'period' : 'cumulative',
  }
}
