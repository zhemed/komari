/**
 * 流量口径相关文案（最小双语表）。
 *
 * 说明：本主题当前没有 i18n 框架（界面文案为内联中文，`index.html` 固定 `lang="zh-CN"`）。
 * 为避免为一件小事引入 i18n 依赖，这里用一张最小文案表覆盖 zh-CN / en 两种语言，
 * 语言按 `<html lang>` 选择（无法识别时回落 zh-CN）。若日后主题引入正式 i18n，应把本表迁过去。
 */

export interface TrafficPeriodText {
  /** 指标名：本周期 */
  period: string
  /** 指标名：累计（历史） */
  cumulative: string
  /** 徽标：口径已切换 */
  basisSwitched: string
  /** 徽标提示 */
  basisSwitchedTip: string
  /** 周期尚未开始 */
  periodNotStarted: string
  /** 重置边界模板，`{day}` / `{timezone}` 会被替换 */
  resetNote: string
}

const ZH_CN: TrafficPeriodText = {
  period: '本周期',
  cumulative: '累计（历史）',
  basisSwitched: '口径已切换',
  basisSwitchedTip: '该节点按累计值已超额、按本周期值未超额；面板已按本周期值判定。',
  periodNotStarted: '本周期尚未开始',
  resetNote: '每月 {day} 日 00:00（{timezone}）重置',
}

const EN: TrafficPeriodText = {
  period: 'This cycle',
  cumulative: 'Lifetime (history)',
  basisSwitched: 'Basis switched',
  basisSwitchedTip: 'This node exceeds the limit on lifetime usage but not on this cycle; the panel now judges by cycle usage.',
  periodNotStarted: 'This cycle has not started yet',
  resetNote: 'Resets on day {day} of each month at 00:00 ({timezone})',
}

const TABLES: Record<string, TrafficPeriodText> = {
  'zh-CN': ZH_CN,
  'zh': ZH_CN,
  'en': EN,
}

/** 读取当前语言（浏览器无 DOM 时回落 zh-CN，便于脚本/单测环境使用） */
export function resolveTrafficTextLocale(): string {
  if (typeof document !== 'undefined' && document.documentElement?.lang)
    return document.documentElement.lang

  return 'zh-CN'
}

export function getTrafficPeriodText(locale?: string): TrafficPeriodText {
  const key = (locale ?? resolveTrafficTextLocale()).toLowerCase()
  const base = key.split('-')[0] ?? ''
  return TABLES[key] ?? TABLES[base] ?? ZH_CN
}

/** 生成「每月 N 日 00:00（时区）重置」文案；未配置周期时返回空串 */
export function formatResetNote(day: number, timezone: string, locale?: string): string {
  if (!Number.isFinite(day) || day < 1)
    return ''

  return getTrafficPeriodText(locale).resetNote.replace('{day}', String(day)).replace('{timezone}', timezone || 'UTC')
}
