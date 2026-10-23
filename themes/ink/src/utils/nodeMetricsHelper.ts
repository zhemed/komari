import type { NodeData } from '@/stores/nodes'
import type { TrafficUsage } from '@/utils/trafficPeriod'
import { getDaysUntilExpired } from '@/utils/tagHelper'
import { resolveTrafficUsage } from '@/utils/trafficPeriod'

export interface TopNodeMetric {
  node: NodeData
  value: number
}

export interface HighLoadMetric {
  key: 'cpu' | 'memory' | 'disk'
  label: string
  percentage: number
}

export function hasTrafficLimit(node: Pick<NodeData, 'traffic_limit'>): boolean {
  return (node.traffic_limit || 0) > 0
}

/** 参与流量口径判定的字段集合（配置 + 实时状态） */
export type TrafficUsageSource = Pick<NodeData, | 'traffic_limit'
  | 'traffic_limit_type'
  | 'traffic_reset_day'
  | 'traffic_reset_timezone'
  | 'net_total_up'
  | 'net_total_down'
  | 'net_period_up'
  | 'net_period_down'
  | 'period_started_at'>

/**
 * 流量口径与数值的**单一真源**：所有展示流量的地方都走这里，不要各算一遍。
 * - 配置了周期（`traffic_reset_day >= 1`）且周期已开始 ⇒ 周期值；
 * - 否则回落累计值。
 * 具体语义见 `@/utils/trafficPeriod`。
 */
export function getTrafficUsage(node: TrafficUsageSource): TrafficUsage {
  return resolveTrafficUsage(
    {
      traffic_reset_day: node.traffic_reset_day,
      traffic_reset_timezone: node.traffic_reset_timezone,
      traffic_limit: node.traffic_limit,
      traffic_limit_type: node.traffic_limit_type,
    },
    {
      net_total_up: node.net_total_up,
      net_total_down: node.net_total_down,
      net_period_up: node.net_period_up,
      net_period_down: node.net_period_down,
      period_started_at: node.period_started_at,
    },
  )
}

/** 当前口径下的「已用」字节数（周期优先，回落累计） */
export function getTrafficUsed(node: TrafficUsageSource): number {
  return getTrafficUsage(node).usedBytes
}

/** 当前口径下的用量百分比（与 `getTrafficUsed` 同源） */
export function getTrafficUsedPercentage(node: TrafficUsageSource): number {
  if (!hasTrafficLimit(node))
    return 0

  return clampPercentage(getTrafficUsage(node).percentage)
}

export function getRealtimeTotalSpeed(node: Pick<NodeData, 'net_in' | 'net_out'>): number {
  return (node.net_in || 0) + (node.net_out || 0)
}

export function getRealtimePeakSpeed(node: Pick<NodeData, 'net_in' | 'net_out'>): number {
  return Math.max(node.net_in || 0, node.net_out || 0)
}

export function getTotalTraffic(node: Pick<NodeData, 'net_total_up' | 'net_total_down'>): number {
  return (node.net_total_up || 0) + (node.net_total_down || 0)
}

export function getConnectionCount(node: Pick<NodeData, 'connections' | 'connections_udp'>): number {
  return (node.connections || 0) + (node.connections_udp || 0)
}

export function getMemoryPercentage(node: Pick<NodeData, 'ram' | 'mem_total'>): number {
  return clampPercentage((node.ram || 0) / (node.mem_total || 1) * 100)
}

export function getDiskPercentage(node: Pick<NodeData, 'disk' | 'disk_total'>): number {
  return clampPercentage((node.disk || 0) / (node.disk_total || 1) * 100)
}

export function getHighLoadMetrics(node: NodeData, threshold: number): HighLoadMetric[] {
  if (!node.online)
    return []

  const metrics: HighLoadMetric[] = []
  const safeThreshold = clampThreshold(threshold, 80)
  const cpu = clampPercentage(node.cpu || 0)
  const memory = getMemoryPercentage(node)
  const disk = getDiskPercentage(node)

  if (cpu >= safeThreshold)
    metrics.push({ key: 'cpu', label: 'CPU', percentage: cpu })
  if (memory >= safeThreshold)
    metrics.push({ key: 'memory', label: '内存', percentage: memory })
  if (disk >= safeThreshold)
    metrics.push({ key: 'disk', label: '硬盘', percentage: disk })

  return metrics
}

export function isHighLoadNode(node: NodeData, threshold: number): boolean {
  return getHighLoadMetrics(node, threshold).length > 0
}

export function hasValidExpiry(node: Pick<NodeData, 'expired_at'>): boolean {
  if (!node.expired_at)
    return false

  const time = Date.parse(String(node.expired_at))
  return Number.isFinite(time)
}

export function getExpiryDays(node: Pick<NodeData, 'expired_at'>): number | null {
  if (!hasValidExpiry(node))
    return null

  return getDaysUntilExpired(node.expired_at)
}

export function isExpiringNode(node: NodeData, days: number): boolean {
  if (node.price === 0)
    return false

  const expiryDays = getExpiryDays(node)
  if (expiryDays === null)
    return false

  return expiryDays <= Math.max(1, days)
}

export function isTrafficWarningNode(node: NodeData, threshold: number): boolean {
  if (!hasTrafficLimit(node))
    return false

  return getTrafficUsedPercentage(node) >= clampThreshold(threshold, 80)
}

export function getTopNodeBy(nodes: readonly NodeData[], selector: (node: NodeData) => number): TopNodeMetric | null {
  let topNode: NodeData | null = null
  let topValue = -Infinity

  for (const node of nodes) {
    const value = selector(node)
    if (!Number.isFinite(value))
      continue

    if (value > topValue) {
      topValue = value
      topNode = node
    }
  }

  if (!topNode)
    return null

  return { node: topNode, value: Math.max(0, topValue) }
}

export function clampThreshold(value: number, fallback: number): number {
  if (!Number.isFinite(value))
    return fallback

  return Math.min(Math.max(value, 1), 100)
}

function clampPercentage(value: number): number {
  if (!Number.isFinite(value))
    return 0

  return Math.min(Math.max(value, 0), 100)
}
