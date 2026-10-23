/**
 * 流量口径单一来源（本周期 / 累计）。
 *
 * 背景：服务端已按「重置日 + 时区」自行划分周期边界，并在
 * `common:getNodesLatestStatus` 里给出 `net_period_up` / `net_period_down` /
 * `period_started_at`；节点配置（`common:getNodes`）里给出 `traffic_reset_day` /
 * `traffic_reset_timezone`。面板此前只用累计值 `net_total_up/down` 算进度，
 * 于是"数字永不下降"。本文件把「用哪个口径、已用多少、百分比多少」收敛成**一次**计算，
 * 供卡片 / 详情 / 表格共用；前端**不**自行推算周期边界（边界只认服务端字段）。
 *
 * 口径规则：
 * 1. `traffic_reset_day >= 1` ⇒ 周期模式：已用 = 服务端周期值，百分比按周期值算；
 *    `period_started_at` 为空 ⇒ 本周期尚未开始，已用按 0（不 NaN / 不除零 / 不为负）。
 * 2. `traffic_reset_day` 缺失或 0 ⇒ 未配置周期：沿用累计值，判定与升级前逐项一致
 *    （同一公式、同一兜底类型、同一"未知类型 ⇒ 0%"行为）。
 * 3. 累计值始终保留，供 UI 标注为「累计（历史）」。
 *
 * 该文件刻意**不 import 任何模块**（只用结构化参数类型），因此可以单独编译/执行做回归对照，
 * 也避免把 context / 组件层耦合进来。
 */

/** 受支持的限额类型，与后端 `traffic_limit_type` 一致。 */
export type TrafficLimitType = "sum" | "max" | "min" | "up" | "down";

const SUPPORTED_TRAFFIC_LIMIT_TYPES: readonly string[] = [
  "sum",
  "max",
  "min",
  "up",
  "down",
];

/**
 * `traffic_limit_type` 为 null / undefined 时的兜底类型。
 * 沿用改动前 `components/Node.tsx` 的 `basic.traffic_limit_type ?? "sum"`，
 * 以保持未配置周期节点的判定与升级前逐项一致（注意：空串 `""` 不兜底，与 `??` 语义一致）。
 */
export const DEFAULT_TRAFFIC_LIMIT_TYPE: TrafficLimitType = "sum";

/** 节点侧（`common:getNodes`）与流量口径相关的字段。 */
export interface TrafficPeriodNodeConfig {
  /** 流量限额（字节），<= 0 表示未配置限额。 */
  traffic_limit?: number | null;
  /** 限额类型；null / undefined 时按 {@link DEFAULT_TRAFFIC_LIMIT_TYPE}。 */
  traffic_limit_type?: string | null;
  /** 0 = 未配置周期；1..31 = 每月重置日。 */
  traffic_reset_day?: number | null;
  /** IANA 时区名；空 = UTC（与后端一致，不用浏览器时区）。 */
  traffic_reset_timezone?: string | null;
}

/** 实时记录侧（`common:getNodesLatestStatus`）与流量口径相关的字段。 */
export interface TrafficPeriodLiveValues {
  /** 累计上行（字节） */
  totalUp?: number | null;
  /** 累计下行（字节） */
  totalDown?: number | null;
  /** 本周期上行（字节） */
  periodUp?: number | null;
  /** 本周期下行（字节） */
  periodDown?: number | null;
  /** 本周期起点（RFC3339）；空 = 服务端未给出周期起点 */
  periodStartedAt?: string | null;
}

/** 解析后的口径结果（面板唯一数据来源）。 */
export interface TrafficUsage {
  /** true = 按服务端周期值；false = 未配置周期，按累计值（升级前行为） */
  periodMode: boolean;
  /** 周期模式下本周期是否已开始（服务端给了合法的 `period_started_at`） */
  periodStarted: boolean;
  /** 重置日（1..31；未配置为 0），直接来自节点配置 */
  resetDay: number;
  /** 重置时区名（节点配置；空值按后端约定显示为 UTC） */
  resetTimezone: string;
  /** 限额（字节；0 = 未配置） */
  limitBytes: number;
  /**
   * 限额类型的展示文本，与升级前一致：
   * 字段缺失（null / undefined / 空串）⇒ 空串；否则原样首字母大写。
   */
  limitTypeLabel: string;
  /** 限额类型是否为受支持的值；false ⇒ 判定不可用（百分比按 0，与升级前 default 分支一致） */
  limitTypeSupported: boolean;
  /** 当前口径下的上行「已用」（周期模式 = 本周期值） */
  usedUp: number;
  /** 当前口径下的下行「已用」（周期模式 = 本周期值） */
  usedDown: number;
  /** 按限额类型组合后的「已用」（字节）；类型不受支持时为 0 */
  usedBytes: number;
  /** 累计上行（历史统计，始终保留） */
  totalUp: number;
  /** 累计下行（历史统计，始终保留） */
  totalDown: number;
  /** 累计口径下按同一限额类型组合的「已用」（基准对照用） */
  legacyUsedBytes: number;
  /** 进度百分比（0 起，不封顶；未配置限额或类型不受支持时为 0） */
  percentage: number;
  /** 累计已超额、周期未超额 ⇒ UI 标注一次「口径已切换」 */
  basisSwitched: boolean;
}

/** 把任意输入收敛为 >= 0 的有限字节数（NaN / Infinity / 负数 / 非数字 ⇒ 0）。 */
export function normalizeBytes(value: number | null | undefined): number {
  if (typeof value !== "number" || !Number.isFinite(value) || value < 0) {
    return 0;
  }
  return value;
}

/** 节点的重置日是否有效（>= 1 ⇒ 服务端会划周期）。 */
export function hasTrafficPeriod(
  node: TrafficPeriodNodeConfig | null | undefined,
): boolean {
  const day = node?.traffic_reset_day;
  return typeof day === "number" && Number.isFinite(day) && day >= 1;
}

/** 解析限额类型：null / undefined 兜底为 sum（与升级前的 `?? "sum"` 一致）。 */
export function resolveTrafficLimitType(
  type: string | null | undefined,
): string {
  return type === null || type === undefined
    ? DEFAULT_TRAFFIC_LIMIT_TYPE
    : type;
}

/** 限额类型是否受支持（与升级前 switch 的 case 集合一致）。 */
export function isSupportedTrafficLimitType(
  type: string | null | undefined,
): boolean {
  return SUPPORTED_TRAFFIC_LIMIT_TYPES.includes(resolveTrafficLimitType(type));
}

/** 限额类型的展示文本（与升级前 `traffic_limit_type && capitalize(...)` 一致）。 */
export function trafficLimitTypeLabel(
  type: string | null | undefined,
): string {
  if (!type) return "";
  return type.charAt(0).toUpperCase() + type.slice(1);
}

/**
 * 按限额类型把上/下行组合成"已用"。
 *
 * 与改动前 `getTrafficPercentage` 的 switch **完全同构**：sum = 上下行之和，
 * max/min = 取大/取小，up/down = 单向；不受支持的类型 ⇒ 0（旧代码 `default: return 0`）。
 */
export function trafficUsedBytesByType(
  up: number,
  down: number,
  type: string | null | undefined,
): number {
  const upBytes = normalizeBytes(up);
  const downBytes = normalizeBytes(down);
  switch (resolveTrafficLimitType(type)) {
    case "max":
      return Math.max(upBytes, downBytes);
    case "min":
      return Math.min(upBytes, downBytes);
    case "up":
      return upBytes;
    case "down":
      return downBytes;
    case "sum":
      return upBytes + downBytes;
    default:
      return 0;
  }
}

/**
 * 「已用 / 限额」百分比。
 *
 * 与改动前 `components/Node.tsx` 的 `getTrafficPercentage` **同公式**（未配置周期节点的
 * 回归项），只额外加了 NaN / 除零 / 负值护栏：
 * - `limit <= 0` ⇒ 0（旧代码 `limit === 0` ⇒ 0；后端拒绝负数限额，故不可达差异）；
 * - 输入 NaN / Infinity / 负数 ⇒ 先归零，避免 `NaN.toFixed()` 之类的异常态。
 */
export function getTrafficPercentage(
  up: number,
  down: number,
  limit: number,
  type: string | null | undefined,
): number {
  const limitBytes = normalizeBytes(limit);
  if (limitBytes <= 0) return 0;
  const used = trafficUsedBytesByType(up, down, type);
  const percentage = (used / limitBytes) * 100;
  if (!Number.isFinite(percentage)) return 0;
  return Math.max(percentage, 0);
}

/** 周期起点是否有效（服务端给出非空字符串，且能解析成时间）。 */
export function isPeriodStarted(
  periodStartedAt: string | null | undefined,
): boolean {
  if (typeof periodStartedAt !== "string") return false;
  const trimmed = periodStartedAt.trim();
  if (trimmed === "") return false;
  return Number.isFinite(new Date(trimmed).getTime());
}

/**
 * 单一来源：解析出面板要用的口径、已用、限额与百分比。
 *
 * @param node 节点配置（`common:getNodes`）
 * @param live 实时值（`common:getNodesLatestStatus`）
 */
export function resolveTrafficUsage(
  node: TrafficPeriodNodeConfig | null | undefined,
  live: TrafficPeriodLiveValues | null | undefined,
): TrafficUsage {
  const periodMode = hasTrafficPeriod(node);
  const resetDay = periodMode ? normalizeBytes(node?.traffic_reset_day) : 0;
  const resetTimezone = (node?.traffic_reset_timezone ?? "").trim() || "UTC";
  const limitBytes = normalizeBytes(node?.traffic_limit);
  const limitType = resolveTrafficLimitType(node?.traffic_limit_type);
  const limitTypeSupported = isSupportedTrafficLimitType(limitType);
  const periodStarted = periodMode && isPeriodStarted(live?.periodStartedAt);

  const totalUp = normalizeBytes(live?.totalUp);
  const totalDown = normalizeBytes(live?.totalDown);
  // 周期模式下：尚未开始 ⇒ 已用按 0（不得 NaN / 除零 / 负值）；否则取服务端周期值。
  const usedUp = periodMode
    ? periodStarted
      ? normalizeBytes(live?.periodUp)
      : 0
    : totalUp;
  const usedDown = periodMode
    ? periodStarted
      ? normalizeBytes(live?.periodDown)
      : 0
    : totalDown;

  const usedBytes = trafficUsedBytesByType(usedUp, usedDown, limitType);
  const legacyUsedBytes = trafficUsedBytesByType(totalUp, totalDown, limitType);

  return {
    periodMode,
    periodStarted,
    resetDay,
    resetTimezone,
    limitBytes,
    limitTypeLabel: trafficLimitTypeLabel(node?.traffic_limit_type),
    limitTypeSupported,
    usedUp,
    usedDown,
    usedBytes,
    totalUp,
    totalDown,
    legacyUsedBytes,
    percentage: getTrafficPercentage(usedUp, usedDown, limitBytes, limitType),
    basisSwitched:
      periodMode &&
      limitTypeSupported &&
      limitBytes > 0 &&
      legacyUsedBytes >= limitBytes &&
      usedBytes < limitBytes,
  };
}
