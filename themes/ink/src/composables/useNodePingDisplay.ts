import type { MaybeRefOrGetter } from 'vue'
import type { NodePingHistoryPoint } from '@/composables/useNodePingStats'
import { computed, ref, toValue } from 'vue'
import { useNodePingStats } from '@/composables/useNodePingStats'
import { PING_SUMMARY_MAX_COUNT } from '@/constants/load'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/helper'

export type NodePingMetric = 'latency' | 'loss'

export interface NodePingBar {
  key: string
  className: string
  tooltip: string
}

// 「三网」行：按 ping 任务分项展示延迟（如电信/联通/移动各自独立测速），
// 不做跨任务平均。后台可能只配置了 1 个任务，也可能配置了 2、3 个甚至更多——
// UI 侧固定只取前 N_TASK_LATENCY_ITEMS 项，其余的本阶段不处理。
export interface NodePingTaskLatencyItem {
  key: string
  valueText: string
  tooltip: string
}

// 首页卡片「按任务分行」的一行：左侧任务名（来自后台 ping 任务 name），
// 右侧该任务自己的延迟/丢包，下方该任务自己的延迟历史点阵。
// 行数据只做展示拼装，不改变跨任务聚合的既有导出（taskLatencyItems / latencyDisplay 等）。
export interface NodePingRegionRow {
  key: string
  label: string
  latencyText: string
  lossText: string
  lossApproximate: boolean
  bars: NodePingBar[]
  latencyTooltip: string
  lossTooltip: string
}

const TASK_LATENCY_DISPLAY_LIMIT = 3

interface UseNodePingDisplayOptions {
  enabled?: MaybeRefOrGetter<boolean>
  loadingDisplayText?: string
  emptyDisplayText?: string
  loadingPanelTooltipText?: Partial<Record<NodePingMetric, string>>
  emptyPanelTooltipText?: Partial<Record<NodePingMetric, string>>
}

const EMPTY_PING_BAR_COUNT = 20

function getLatencyToneClass(latency: number): string {
  if (latency <= 60)
    return 'bg-signal-1'
  if (latency <= 100)
    return 'bg-signal-2'
  if (latency <= 160)
    return 'bg-signal-3 ping-signal-pattern-2'
  if (latency <= 200)
    return 'bg-signal-4 ping-signal-pattern-3'
  return 'bg-signal-5 ping-signal-pattern-4'
}

function getLossToneClass(loss: number): string {
  if (loss <= 1)
    return 'bg-signal-1'
  if (loss <= 3)
    return 'bg-signal-2'
  if (loss <= 6)
    return 'bg-signal-3 ping-signal-pattern-2'
  if (loss <= 9)
    return 'bg-signal-4 ping-signal-pattern-3'
  return 'bg-signal-5 ping-signal-pattern-4'
}

export function useNodePingDisplay(
  uuid: MaybeRefOrGetter<string>,
  options: UseNodePingDisplayOptions = {},
) {
  const appStore = useAppStore()

  const pingStatsEnabled = computed(() => {
    if (toValue(options.enabled) === false)
      return false
    if (appStore.publicSettings?.record_enabled === false)
      return false
    return appStore.publicSettings?.ping_record_preserve_time !== 0
  })

  const pingStatsHours = computed(() => {
    const preserveTime = appStore.publicSettings?.ping_record_preserve_time
    if (typeof preserveTime === 'number' && preserveTime > 0)
      return Math.min(preserveTime, 1)
    return 1
  })

  const pingStats = useNodePingStats(uuid, {
    hours: pingStatsHours,
    enabled: pingStatsEnabled,
    maxCount: PING_SUMMARY_MAX_COUNT,
  })

  function buildPingBars(metric: NodePingMetric): NodePingBar[] {
    const points = pingStats.history.value
    if (!points.length)
      return []

    return points.map((point, index) => {
      const value = point[metric]

      return {
        key: `${point.time}-${index}`,
        className: value === null
          ? 'bg-muted-foreground/15'
          : metric === 'latency'
            ? getLatencyToneClass(value)
            : getLossToneClass(value),
        tooltip: value === null
          ? `${formatDateTime(point.time, 'HH:mm:ss')}\n无采样数据`
          : metric === 'latency'
            ? `${formatDateTime(point.time, 'HH:mm:ss')}\n${Math.round(value)} ms`
            : `${formatDateTime(point.time, 'HH:mm:ss')}\n${value.toFixed(1)}%`,
      }
    })
  }

  function buildEmptyPingBars(metric: NodePingMetric): NodePingBar[] {
    const tooltip = pingStats.loading.value
      ? '加载中'
      : pingStats.error.value
        ? '加载失败'
        : !pingStatsEnabled.value
            ? '未启用记录'
            : metric === 'latency'
              ? '无采样数据'
              : '无采样数据'

    return Array.from({ length: EMPTY_PING_BAR_COUNT }, (_, index) => ({
      key: `${metric}-empty-${index}`,
      className: 'bg-muted-foreground/10',
      tooltip,
    }))
  }

  // 每任务历史点阵：复用全局色阶与既有 tooltip 口径（HH:mm:ss + ms），
  // 不足 EMPTY_PING_BAR_COUNT 格时用灰色空格补齐，保证不同任务行点阵宽度一致。
  function buildTaskHistoryBars(taskId: string, history: NodePingHistoryPoint[]): NodePingBar[] {
    const bars = history.map((point, index) => {
      const value = point.latency
      const hasValue = typeof value === 'number' && Number.isFinite(value)

      return {
        key: `${taskId}-${point.time}-${index}`,
        className: hasValue
          ? getLatencyToneClass(value)
          : 'bg-muted-foreground/15',
        tooltip: hasValue
          ? `${formatDateTime(point.time, 'HH:mm:ss')}\n${Math.round(value)} ms`
          : `${formatDateTime(point.time, 'HH:mm:ss')}\n无采样数据`,
      }
    })

    while (bars.length < EMPTY_PING_BAR_COUNT) {
      bars.push({
        key: `${taskId}-empty-${bars.length}`,
        className: 'bg-muted-foreground/15',
        tooltip: '无采样数据',
      })
    }

    return bars
  }

  // 行顺序按 taskId 数字升序（线上任务 id 与后台 weight 顺序一致）；非数字 id 保持原相对顺序排到最后。
  function getTaskOrderValue(taskId: string): number {
    const numericTaskId = Number(taskId)
    return Number.isFinite(numericTaskId) ? numericTaskId : Number.POSITIVE_INFINITY
  }

  function compareTaskId(left: string, right: string): number {
    const leftValue = getTaskOrderValue(left)
    const rightValue = getTaskOrderValue(right)
    if (leftValue === rightValue)
      return 0
    return leftValue < rightValue ? -1 : 1
  }

  const latencyBars = computed(() => buildPingBars('latency'))
  const lossBars = computed(() => buildPingBars('loss'))
  // 路由跳转/选中切换瞬间 enabled 变 false，会导致节点卡片 ping 数据被清空闪烁。
  // 记忆最后一次有数据的显示，避免降级为占位符（-）。
  const lastLatencyBars = ref<NodePingBar[]>([])
  const lastLossBars = ref<NodePingBar[]>([])
  const lastLatencyText = ref('')
  const lastLossText = ref('')

  const latencyRenderBars = computed(() => {
    if (latencyBars.value.length) {
      lastLatencyBars.value = latencyBars.value
      return latencyBars.value
    }
    if (lastLatencyBars.value.length)
      return lastLatencyBars.value
    return buildEmptyPingBars('latency')
  })
  const lossRenderBars = computed(() => {
    if (lossBars.value.length) {
      lastLossBars.value = lossBars.value
      return lossBars.value
    }
    if (lastLossBars.value.length)
      return lastLossBars.value
    return buildEmptyPingBars('loss')
  })

  const latencyDisplay = computed(() => {
    if (pingStats.hasData.value) {
      const text = `${Math.round(pingStats.avgLatency.value)} ms`
      lastLatencyText.value = text
      return text
    }
    if (lastLatencyText.value)
      return lastLatencyText.value
    if (pingStats.loading.value)
      return options.loadingDisplayText ?? '加载中'
    return options.emptyDisplayText ?? '-'
  })

  const lossDisplay = computed(() => {
    if (pingStats.hasData.value) {
      const text = `${pingStats.avgLoss.value.toFixed(1)}%`
      lastLossText.value = text
      return text
    }
    if (lastLossText.value)
      return lastLossText.value
    if (pingStats.loading.value)
      return options.loadingDisplayText ?? '加载中'
    return options.emptyDisplayText ?? '-'
  })

  const latencyPanelTooltip = computed(() => {
    if (!pingStats.hasData.value) {
      if (pingStats.loading.value)
        return options.loadingPanelTooltipText?.latency ?? ''
      return options.emptyPanelTooltipText?.latency ?? ''
    }
    return `平均延迟 ${Math.round(pingStats.avgLatency.value)} ms`
  })

  const lossPanelTooltip = computed(() => {
    if (!pingStats.hasData.value) {
      if (pingStats.loading.value)
        return options.loadingPanelTooltipText?.loss ?? ''
      return options.emptyPanelTooltipText?.loss ?? ''
    }

    const volatility = pingStats.avgVolatility.value > 0
      ? `，平均波动 ${pingStats.avgVolatility.value.toFixed(2)}`
      : ''
    return `平均丢包 ${pingStats.avgLoss.value.toFixed(1)}%${volatility}`
  })

  // 只要该节点配置了 ping 任务（不管 1 个、2 个还是 3 个以上），就取前
  // TASK_LATENCY_DISPLAY_LIMIT 项渲染「三网」行；单任务节点这里就是一个数字，
  // 不带分隔符——由使用方（模板）按数组长度自然渲染，这里不做数量上的特殊分支。
  const taskLatencyItemsRaw = computed<NodePingTaskLatencyItem[]>(() => {
    return pingStats.taskLatencies.value
      .slice(0, TASK_LATENCY_DISPLAY_LIMIT)
      .map((task) => {
        const valueText = task.latency === null ? '--' : `${Math.round(task.latency)}ms`
        const label = task.name?.trim() || `任务 ${task.taskId}`
        return {
          key: task.taskId,
          valueText,
          tooltip: `${label} ${valueText}`,
        }
      })
  })
  // 与 lastLatencyBars 同理：enabled 短暂变 false（如路由切换）时不应该让这一行闪烁消失。
  const lastTaskLatencyItems = ref<NodePingTaskLatencyItem[]>([])
  const taskLatencyItems = computed(() => {
    if (taskLatencyItemsRaw.value.length) {
      lastTaskLatencyItems.value = taskLatencyItemsRaw.value
      return taskLatencyItemsRaw.value
    }
    return lastTaskLatencyItems.value
  })
  const hasTaskLatencyItems = computed(() => taskLatencyItems.value.length > 0)

  // 首页卡片按任务分行：最多 TASK_LATENCY_DISPLAY_LIMIT 行，按 taskId 数字升序（线上任务 id 即后台 weight 顺序）。
  // 每行右侧是该任务自己的延迟/丢包，下方是该任务自己的延迟历史点阵（不足 20 格灰显补齐）。
  const taskRegionRowsRaw = computed<NodePingRegionRow[]>(() => {
    return [...pingStats.taskLatencies.value]
      .sort((left, right) => compareTaskId(left.taskId, right.taskId))
      .slice(0, TASK_LATENCY_DISPLAY_LIMIT)
      .map((task) => {
        const label = task.name?.trim() || `任务 ${task.taskId}`
        const latencyText = typeof task.latency === 'number' && Number.isFinite(task.latency)
          ? `${Math.round(task.latency)} ms`
          : '--'
        const loss = typeof task.loss === 'number' && Number.isFinite(task.loss) ? task.loss : null
        // 与 PingChart 的任务统计口径一致：近似丢包率加 ≈ 标注；行内文本统一带「丢包 」前缀（对齐参考版式）。
        const lossText = loss === null ? '-' : `丢包 ${task.lossApproximate ? '≈' : ''}${loss.toFixed(1)}%`

        return {
          key: task.taskId,
          label,
          latencyText,
          lossText,
          lossApproximate: task.lossApproximate === true,
          bars: buildTaskHistoryBars(task.taskId, task.history ?? []),
          latencyTooltip: `${label} ${latencyText}`,
          lossTooltip: `${label} ${lossText}`,
        }
      })
  })
  // 与 lastTaskLatencyItems 同理：enabled 短暂变 false（如路由切换）时保留上一次的行数据，
  // 避免整块列表闪成占位行；真正启用但确实没有任务数据时仍回到占位行。
  const lastTaskRegionRows = ref<NodePingRegionRow[]>([])
  const taskRegionRows = computed(() => {
    if (taskRegionRowsRaw.value.length) {
      lastTaskRegionRows.value = taskRegionRowsRaw.value
      return taskRegionRowsRaw.value
    }
    return pingStatsEnabled.value ? [] : lastTaskRegionRows.value
  })
  const hasTaskRegionRows = computed(() => taskRegionRows.value.length > 0)
  // 无任务级数据时的占位行点阵：沿用既有 loading / error / 未启用 的 tooltip 口径。
  const taskRowPlaceholderBars = computed(() => buildEmptyPingBars('latency'))

  return {
    pingStats,
    pingStatsEnabled,
    pingStatsHours,
    latencyRenderBars,
    lossRenderBars,
    latencyDisplay,
    lossDisplay,
    latencyPanelTooltip,
    lossPanelTooltip,
    taskLatencyItems,
    hasTaskLatencyItems,
    taskRegionRows,
    hasTaskRegionRows,
    taskRowPlaceholderBars,
  }
}
