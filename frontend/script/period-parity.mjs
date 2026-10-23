#!/usr/bin/env node
/**
 * 前端口径回归：本周期 / 累计 判定与百分比（零依赖、可复跑）
 *
 * 背景：面板展示与限额判定已从「累计值」切到「服务端周期值」
 * （见 frontend/src/utils/trafficPeriod.ts 与提交 [task:period-panel-frontend]）。
 * 该文件把这次切换的两条硬约束固化成可复跑断言：
 *
 *   1. 未配置周期（traffic_reset_day = 0 / 字段缺失，含老服务端 payload）的节点，
 *      百分比必须与**改动前的旧公式**在同一组输入下逐位相同（零回归）；
 *   2. 周期模式下已用取服务端周期值；period_started_at 为 null / 空串 / 垃圾串 /
 *      记录缺失 / 限额 0 / NaN 与负数污染时，不得出现 NaN / Infinity / 负值。
 *
 * 为什么能零依赖直接跑：
 *   - `frontend/src/utils/trafficPeriod.ts` 没有任何 import（纯函数 + 类型）；
 *   - Node >= 22.18（本机 v24）默认开启类型擦除，可 `import` 该 .ts 文件；
 *   - 因此不需要 vitest / jest / ts-node，也不新增任何依赖。
 *
 * 参照实现（旧公式）来源：`frontend/src/components/Node.tsx` 在被切换前的最后一个提交
 * （见 LEGACY_REFERENCE）：其中的 `getTrafficPercentage` 被原样内联为
 * `legacyGetTrafficPercentage`；若当前仓库里能取到那个提交，脚本会把它从 git 里再取一次、
 * 交给 Node 擦除类型后与内联副本做**去空白等值**比对（来源自证，取不到就跳过并打印说明）。
 *
 * 用法：
 *   cd frontend && npm run test:period        # 推荐
 *   cd frontend && node script/period-parity.mjs
 *
 * 退出码：0 = 全部通过；1 = 存在失败项（CI / 提交前可直接用）；2 = 环境不满足（Node 过旧）。
 */

import { execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { fileURLToPath, pathToFileURL } from "node:url";

/**
 * 动态 import 被验证的实现：Node < 22.18 没有默认类型擦除，会直接抛错——
 * 这里给出可读提示并以退出码 2 结束（区别于断言失败的退出码 1）。
 */
async function loadTrafficPeriodModule() {
  try {
    return await import("../src/utils/trafficPeriod.ts");
  } catch (error) {
    console.error("[period-parity] 无法 import ../src/utils/trafficPeriod.ts");
    console.error(
      `  需要 Node >= 22.18（默认开启类型擦除）；当前 ${process.version}`,
    );
    console.error(`  原始错误：${error?.message ?? error}`);
    process.exit(2);
  }
}

const {
  getTrafficPercentage,
  hasTrafficPeriod,
  isPeriodStarted,
  resolveTrafficUsage,
  trafficUsedBytesByType,
} = await loadTrafficPeriodModule();

// ---------------------------------------------------------------------------
// 配置
// ---------------------------------------------------------------------------

/** 被切换前的最后一个提交：`f672d9f^`（面板口径切到周期值的那次提交之前）。 */
const LEGACY_REFERENCE = Object.freeze({
  commit: "526b466",
  path: "frontend/src/components/Node.tsx",
  lines: "361-382",
  description: "改动前的 getTrafficPercentage（累计口径百分比）",
});

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const FRONTEND_DIR = path.resolve(SCRIPT_DIR, "..");
const REPO_ROOT = path.resolve(FRONTEND_DIR, "..");

const GiB = 1024 ** 3;
const TiB = 1024 ** 4;

// ---------------------------------------------------------------------------
// 极简断言框架（不引入任何测试框架）
// ---------------------------------------------------------------------------

let passed = 0;
const failures = [];

function section(title) {
  console.log(`\n== ${title} ==`);
}

function check(label, ok, detail = "") {
  if (ok) {
    passed += 1;
    console.log(`  PASS  ${label}${detail ? `  | ${detail}` : ""}`);
  } else {
    failures.push(label);
    console.log(`  FAIL  ${label}${detail ? `  | ${detail}` : ""}`);
  }
}

const isFiniteNonNegative = (value) =>
  typeof value === "number" && Number.isFinite(value) && value >= 0;

// ---------------------------------------------------------------------------
// 参照实现：改动前的 getTrafficPercentage（原样内联，勿改）
//
// 取自 LEGACY_REFERENCE 指向的提交；类型注解按 JS 写法省略，函数体逐行保留，
// 脚本会用 git 里的原文（经 Node 类型擦除）与它做去空白等值比对（见 §0.2）。
// ---------------------------------------------------------------------------

function legacyGetTrafficPercentage(totalUp, totalDown, limit, type) {
  if (limit === 0) return 0;
  switch (type) {
    case "max":
      return (Math.max(totalUp, totalDown) / limit) * 100;
    case "min":
      return (Math.min(totalUp, totalDown) / limit) * 100;
    case "sum":
      return ((totalUp + totalDown) / limit) * 100;
    case "up":
      return (totalUp / limit) * 100;
    case "down":
      return (totalDown / limit) * 100;
    default:
      return 0;
  }
}

/** 改动前面板调用旧公式时的实参：`basic.traffic_limit_type ?? "sum"`。 */
const legacyEffectiveType = (type) => type ?? "sum";

/** 去空白 + 去尾部多余逗号 + 忽略函数名（内联副本刻意改名为 legacyGetTrafficPercentage）。 */
const normalizeSource = (source) =>
  String(source)
    .trim()
    .replace(/^function\s+[A-Za-z0-9_$]+/, "function")
    .replace(/\s+/g, "")
    .replace(/,\)/g, ")");

// ---------------------------------------------------------------------------
// §0 环境与来源自证
// ---------------------------------------------------------------------------

section("§0 环境与参照实现来源");

console.log(`  Node ${process.version}（类型擦除直接 import src/utils/trafficPeriod.ts）`);
console.log(
  `  旧公式来源：${LEGACY_REFERENCE.path}@${LEGACY_REFERENCE.commit}:${LEGACY_REFERENCE.lines}`,
);

check(
  "已加载被验证的实现（resolveTrafficUsage 是函数）",
  typeof resolveTrafficUsage === "function",
);

/** 从 git 取旧函数原文，交给 Node 擦除类型后返回其 { toString, sha256 }；取不到返回 null。 */
async function loadLegacyFromGit() {
  let raw;
  try {
    raw = execFileSync(
      "git",
      ["show", `${LEGACY_REFERENCE.commit}:${LEGACY_REFERENCE.path}`],
      { cwd: REPO_ROOT, encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] },
    );
  } catch {
    return null;
  }
  const start = raw.indexOf("function getTrafficPercentage(");
  if (start < 0) return null;
  const end = raw.indexOf("\n}", start);
  if (end < 0) return null;
  const source = raw.slice(start, end + 2);

  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "period-parity-"));
  const tmpFile = path.join(tmpDir, "legacy-reference.ts");
  try {
    fs.writeFileSync(tmpFile, `${source}\nexport { getTrafficPercentage };\n`, "utf8");
    const mod = await import(pathToFileURL(tmpFile).href);
    const hash = await import("node:crypto").then(({ createHash }) =>
      createHash("sha256").update(source).digest("hex"),
    );
    return { source, hash, toString: mod.getTrafficPercentage.toString() };
  } catch {
    return null;
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

const legacyFromGit = await loadLegacyFromGit();
if (legacyFromGit) {
  check(
    "内联的旧公式与 git 里的原文等值（去空白、去类型注解后）",
    normalizeSource(legacyFromGit.toString) ===
      normalizeSource(legacyGetTrafficPercentage.toString()),
    `sha256(旧函数原文)=${legacyFromGit.hash.slice(0, 16)}…`,
  );
} else {
  console.log(
    `  SKIP  来源自证：当前仓库取不到 ${LEGACY_REFERENCE.commit}（浅克隆 / 已 squash？）——` +
      "逐位对照仍会执行，但“内联副本 = 原文”这一步未验证",
  );
}

// ---------------------------------------------------------------------------
// §1 未配置周期：与旧公式逐位相同（零回归）
// ---------------------------------------------------------------------------

section("§1 未配置周期（traffic_reset_day = 0 / 缺失）⇒ 与旧公式逐位相同");

/** 未配置周期的节点：[标签, totalUp, totalDown, limit, traffic_limit_type] */
const legacyCases = [
  ["实测节点：累计 544.75 GiB / 限额 1 TiB", 544.75 * GiB, 0, TiB, "sum"],
  ["双向均低于限额", 10 * GiB, 5 * GiB, 100 * GiB, "sum"],
  ["刚好等于限额", 50 * GiB, 50 * GiB, 100 * GiB, "sum"],
  ["超出限额", 120 * GiB, 30 * GiB, 100 * GiB, "sum"],
  ["累计为 0", 0, 0, 100 * GiB, "sum"],
  ["限额为 0（未配置限额）", 10 * GiB, 5 * GiB, 0, "sum"],
  ["类型 max", 120 * GiB, 30 * GiB, 100 * GiB, "max"],
  ["类型 min", 120 * GiB, 30 * GiB, 100 * GiB, "min"],
  ["类型 up", 120 * GiB, 30 * GiB, 100 * GiB, "up"],
  ["类型 down", 120 * GiB, 30 * GiB, 100 * GiB, "down"],
  ["类型缺失（旧代码 ?? \"sum\"）", 7 * GiB, 3 * GiB, 100 * GiB, undefined],
  ["类型空串（旧代码 default ⇒ 0%）", 120 * GiB, 30 * GiB, 100 * GiB, ""],
  ["类型未知 MAX（旧代码 default ⇒ 0%）", 120 * GiB, 30 * GiB, 100 * GiB, "MAX"],
  ["类型未知 bogus（旧代码 default ⇒ 0%）", 120 * GiB, 30 * GiB, 100 * GiB, "bogus"],
  ["小数值 sum", 3 * GiB + 12345678, 7 * GiB + 87654321, 50 * GiB, "sum"],
  ["小数值 max", 3 * GiB + 12345678, 7 * GiB + 87654321, 50 * GiB, "max"],
  ["小数值 min", 3 * GiB + 12345678, 7 * GiB + 87654321, 50 * GiB, "min"],
  ["小数值 up", 3 * GiB + 12345678, 7 * GiB + 87654321, 50 * GiB, "up"],
  ["小数值 down", 3 * GiB + 12345678, 7 * GiB + 87654321, 50 * GiB, "down"],
];

const legacyLive = (up, down) => ({
  totalUp: up,
  totalDown: down,
  periodUp: 0,
  periodDown: 0,
  periodStartedAt: null,
});

let legacyIdentical = 0;
for (const [label, up, down, limit, type] of legacyCases) {
  const node = {
    traffic_limit: limit,
    traffic_limit_type: type,
    traffic_reset_day: 0,
  };
  const before = legacyGetTrafficPercentage(up, down, limit, legacyEffectiveType(type));
  const after = resolveTrafficUsage(node, legacyLive(up, down)).percentage;
  const same = before === after;
  if (same) legacyIdentical += 1;
  check(
    `逐位一致：${label}`,
    same,
    `before=${before} after=${after}${same ? "" : "  ← 不一致"}`,
  );
}
console.log(`  → ${legacyIdentical}/${legacyCases.length} 组输入逐位一致`);

// 老服务端：payload 里没有 traffic_reset_day 字段（等价于未配置周期）
{
  const node = { traffic_limit: 100 * GiB, traffic_limit_type: "max" };
  const before = legacyGetTrafficPercentage(120 * GiB, 30 * GiB, 100 * GiB, "max");
  const usage = resolveTrafficUsage(node, legacyLive(120 * GiB, 30 * GiB));
  check(
    "老服务端（payload 无 traffic_reset_day）仍走累计口径",
    usage.periodMode === false && usage.percentage === before,
    `periodMode=${usage.periodMode} before=${before} after=${usage.percentage}`,
  );
}

// 未配置限额模式（traffic_limit = 0）在两种实现下都不除零
{
  const usage = resolveTrafficUsage(
    { traffic_limit: 0, traffic_limit_type: "sum", traffic_reset_day: 0 },
    legacyLive(10 * GiB, 5 * GiB),
  );
  check(
    "未配置限额 ⇒ 0%（不除零），且与旧公式一致",
    usage.percentage === 0 &&
      usage.percentage === legacyGetTrafficPercentage(10 * GiB, 5 * GiB, 0, "sum"),
    `percentage=${usage.percentage}`,
  );
}

// ---------------------------------------------------------------------------
// §2 周期模式：已用取周期值、百分比按周期值
// ---------------------------------------------------------------------------

section("§2 周期模式（traffic_reset_day = 22，Asia/Shanghai）");

{
  const node = {
    traffic_limit: 500 * GiB,
    traffic_limit_type: "sum",
    traffic_reset_day: 22,
    traffic_reset_timezone: "Asia/Shanghai",
  };
  const live = {
    totalUp: 600 * GiB,
    totalDown: 0,
    periodUp: 0.02 * GiB,
    periodDown: 0,
    periodStartedAt: "2026-10-22T16:00:00Z",
  };
  const usage = resolveTrafficUsage(node, live);
  check(
    "周期模式启用且周期已开始",
    usage.periodMode === true && usage.periodStarted === true,
  );
  check(
    "已用取服务端周期值（不是累计）",
    usage.usedUp === 0.02 * GiB && usage.usedBytes === 0.02 * GiB,
    `usedUp=${usage.usedUp}`,
  );
  check(
    "百分比按周期值",
    Math.abs(usage.percentage - (0.02 * GiB / (500 * GiB)) * 100) < 1e-9,
    `percentage=${usage.percentage}`,
  );
  check(
    "累计（历史）对照值保留",
    usage.legacyUsedBytes === 600 * GiB && usage.totalUp === 600 * GiB,
    `legacyUsedBytes=${usage.legacyUsedBytes}`,
  );
  check(
    "重置说明参数来自节点配置（日 + 时区名）",
    usage.resetDay === 22 && usage.resetTimezone === "Asia/Shanghai",
    `${usage.resetDay}/${usage.resetTimezone}`,
  );
  check(
    "空时区按后端约定回落 UTC（不读浏览器时区）",
    resolveTrafficUsage(
      { traffic_reset_day: 1, traffic_reset_timezone: "" },
      {},
    ).resetTimezone === "UTC",
  );
}

// 周期模式 + 各限额类型：已用组合方式与旧公式同构
{
  const periodLive = {
    totalUp: 9 * GiB,
    totalDown: 4 * GiB,
    periodUp: 30 * GiB,
    periodDown: 20 * GiB,
    periodStartedAt: "2026-10-22T16:00:00Z",
  };
  const node = (type) => ({
    traffic_limit: 100 * GiB,
    traffic_limit_type: type,
    traffic_reset_day: 22,
    traffic_reset_timezone: "UTC",
  });
  const expected = {
    sum: 50 * GiB,
    max: 30 * GiB,
    min: 20 * GiB,
    up: 30 * GiB,
    down: 20 * GiB,
  };
  for (const [type, want] of Object.entries(expected)) {
    const usage = resolveTrafficUsage(node(type), periodLive);
    check(
      `周期模式 + 类型 ${type}：已用按周期值组合`,
      usage.usedBytes === want &&
        usage.percentage === (want / (100 * GiB)) * 100,
      `usedBytes=${usage.usedBytes} percentage=${usage.percentage}`,
    );
  }
}

// ---------------------------------------------------------------------------
// §3 存量节点与异常输入：不 NaN / 不除零 / 不为负
// ---------------------------------------------------------------------------

section("§3 period_started_at 为空 / 记录缺失 / 污染输入 ⇒ 数值有限且非负");

const periodNode = {
  traffic_limit: 100 * GiB,
  traffic_limit_type: "sum",
  traffic_reset_day: 5,
  traffic_reset_timezone: "UTC",
};

const guardCases = [
  ["起点为 null", periodNode, { totalUp: 544.75 * GiB, totalDown: 0, periodUp: 0, periodDown: 0, periodStartedAt: null }],
  ["起点为空串", periodNode, { totalUp: 544.75 * GiB, totalDown: 0, periodUp: 0, periodDown: 0, periodStartedAt: "" }],
  ["起点为空白串", periodNode, { totalUp: 544.75 * GiB, totalDown: 0, periodUp: 0, periodDown: 0, periodStartedAt: "   " }],
  ["起点是垃圾串", periodNode, { totalUp: 1, totalDown: 2, periodUp: 3, periodDown: 4, periodStartedAt: "not-a-date" }],
  ["周期值字段整体缺失", periodNode, { totalUp: 544.75 * GiB, totalDown: 0 }],
  ["实时记录整体缺失", periodNode, undefined],
  ["限额缺失", { traffic_reset_day: 5, traffic_reset_timezone: "UTC" }, { totalUp: 1, totalDown: 2, periodStartedAt: null }],
  ["NaN / 负数污染", periodNode, { totalUp: -5, totalDown: Number.NaN, periodUp: Number.NaN, periodDown: -1, periodStartedAt: null }],
  ["Infinity 污染", periodNode, { totalUp: Number.POSITIVE_INFINITY, totalDown: -Infinity, periodUp: Number.POSITIVE_INFINITY, periodDown: 0, periodStartedAt: "2026-10-22T16:00:00Z" }],
  ["周期已开始但周期值缺失", periodNode, { totalUp: 500 * GiB, totalDown: 0, periodStartedAt: "2026-10-22T16:00:00Z" }],
];

for (const [label, node, live] of guardCases) {
  const usage = resolveTrafficUsage(node, live);
  const numbers = [
    usage.percentage,
    usage.usedUp,
    usage.usedDown,
    usage.usedBytes,
    usage.totalUp,
    usage.totalDown,
    usage.legacyUsedBytes,
  ];
  check(
    `${label}：所有数值有限且非负`,
    numbers.every(isFiniteNonNegative),
    `percentage=${usage.percentage} used=${usage.usedUp}/${usage.usedDown} total=${usage.totalUp}/${usage.totalDown}`,
  );
}

// 「本周期尚未开始」的判定：null / 空串 / 垃圾串 / 记录缺失 都必须为 false
for (const [label, live] of [
  ["null", { periodStartedAt: null }],
  ["空串", { periodStartedAt: "" }],
  ["垃圾串", { periodStartedAt: "not-a-date" }],
  ["记录缺失", undefined],
]) {
  const usage = resolveTrafficUsage(periodNode, live);
  check(
    `起点 ${label} ⇒ periodStarted=false（UI 显示「本周期尚未开始」）且按 0`,
    usage.periodStarted === false && usage.usedBytes === 0 && usage.percentage === 0,
    `periodStarted=${usage.periodStarted} usedBytes=${usage.usedBytes} percentage=${usage.percentage}`,
  );
}
check(
  "isPeriodStarted 与判定一致（合法 RFC3339 ⇒ true）",
  isPeriodStarted("2026-10-22T16:00:00Z") === true &&
    isPeriodStarted(null) === false &&
    isPeriodStarted("") === false,
);
check(
  "周期已配置但起点为空 ⇒ periodMode=true / periodStarted=false（区别于“未配置周期”）",
  resolveTrafficUsage(periodNode, { periodStartedAt: null }).periodMode === true &&
    hasTrafficPeriod(periodNode) === true &&
    hasTrafficPeriod({ traffic_reset_day: 0 }) === false &&
    hasTrafficPeriod({}) === false,
);

// 除零与类型护栏
check(
  "限额 0 ⇒ 0%（不除零）",
  getTrafficPercentage(10 * GiB, 10 * GiB, 0, "sum") === 0 &&
    resolveTrafficUsage(
      { traffic_limit: 0, traffic_limit_type: "sum", traffic_reset_day: 5 },
      { periodUp: 10, periodDown: 10, periodStartedAt: "2026-10-22T16:00:00Z" },
    ).percentage === 0,
);
for (const type of ["sum", "max", "min", "up", "down"]) {
  check(
    `trafficUsedBytesByType(${type}) 组合正确`,
    trafficUsedBytesByType(30, 20, type) ===
      ({ sum: 50, max: 30, min: 20, up: 30, down: 20 })[type],
  );
}
check(
  "未知类型 ⇒ 已用 0、百分比 0（与旧公式 default 分支一致）",
  trafficUsedBytesByType(30, 20, "bogus") === 0 &&
    getTrafficPercentage(30, 20, 100, "bogus") === 0,
);

// ---------------------------------------------------------------------------
// §4 存量超额节点：以周期为准 + 口径已切换标记
// ---------------------------------------------------------------------------

section("§4 存量超额（累计≥限额、周期<限额）⇒ basisSwitched = true");

{
  const node = {
    traffic_limit: 500 * GiB,
    traffic_limit_type: "sum",
    traffic_reset_day: 22,
    traffic_reset_timezone: "UTC",
  };
  const switched = resolveTrafficUsage(node, {
    totalUp: 600 * GiB,
    totalDown: 0,
    periodUp: 0.02 * GiB,
    periodDown: 0,
    periodStartedAt: "2026-10-22T00:00:00Z",
  });
  check(
    "累计超额 + 周期未超额 ⇒ 标注「口径已切换」",
    switched.basisSwitched === true &&
      switched.legacyUsedBytes >= 500 * GiB &&
      switched.usedBytes < 500 * GiB,
    `legacyUsedBytes=${switched.legacyUsedBytes} usedBytes=${switched.usedBytes}`,
  );

  const notOver = resolveTrafficUsage(node, {
    totalUp: 80 * GiB,
    totalDown: 10 * GiB,
    periodUp: 20 * GiB,
    periodDown: 5 * GiB,
    periodStartedAt: "2026-10-22T00:00:00Z",
  });
  check(
    "累计未超额 ⇒ 不标注",
    notOver.basisSwitched === false,
    `legacyUsedBytes=${notOver.legacyUsedBytes}`,
  );

  const bothOver = resolveTrafficUsage(node, {
    totalUp: 600 * GiB,
    totalDown: 0,
    periodUp: 600 * GiB,
    periodDown: 0,
    periodStartedAt: "2026-10-22T00:00:00Z",
  });
  check(
    "周期也已超额 ⇒ 不标注（避免噪声）",
    bothOver.basisSwitched === false,
    `usedBytes=${bothOver.usedBytes}`,
  );

  const notStartedOver = resolveTrafficUsage(
    { ...node, traffic_limit_type: "sum" },
    { totalUp: 600 * GiB, totalDown: 0, periodStartedAt: null },
  );
  check(
    "周期未开始的存量超额节点也标注（以周期为准，本周期 0）",
    notStartedOver.basisSwitched === true && notStartedOver.usedBytes === 0,
    `usedBytes=${notStartedOver.usedBytes}`,
  );

  const noLimit = resolveTrafficUsage(
    { ...node, traffic_limit: 0 },
    { totalUp: 600 * GiB, totalDown: 0, periodStartedAt: "2026-10-22T00:00:00Z" },
  );
  check(
    "未配置限额 ⇒ 不标注（不构成超额判定）",
    noLimit.basisSwitched === false,
  );
}

// ---------------------------------------------------------------------------
// §5 单一来源与文案可用性（口径切换的回归闸门）
// ---------------------------------------------------------------------------

section("§5 单一来源 / 语言包键（防止口径被改回、防止显示原始 key）");

{
  const nodeSource = fs.readFileSync(
    path.join(FRONTEND_DIR, "src/components/Node.tsx"),
    "utf8",
  );
  check(
    "Node.tsx 不再自带 getTrafficPercentage（口径只在 utils/trafficPeriod.ts）",
    !/function\s+getTrafficPercentage\(/.test(nodeSource),
  );
  check(
    "Node.tsx 使用 resolveTrafficUsage（单一来源）",
    nodeSource.includes("resolveTrafficUsage"),
  );

  const requiredKeys = [
    "periodTraffic",
    "periodUpload",
    "periodDownload",
    "periodNotStarted",
    "periodResetNote",
    "cumulativeHistory",
    "limitLabel",
    "basisSwitched",
    "basisSwitchedTip",
  ];
  for (const locale of ["en", "zh_CN", "zh_TW", "ja_JP", "id_ID"]) {
    const file = path.join(FRONTEND_DIR, `src/i18n/locales/${locale}.json`);
    let nodeCardKeys = [];
    try {
      nodeCardKeys = Object.keys(JSON.parse(fs.readFileSync(file, "utf8")).nodeCard ?? {});
    } catch {
      nodeCardKeys = [];
    }
    const missing = requiredKeys.filter((key) => !nodeCardKeys.includes(key));
    check(
      `语言包 ${locale}：本周期相关 ${requiredKeys.length} 键齐全`,
      missing.length === 0,
      missing.length ? `缺 ${missing.join(",")}` : "",
    );
  }
}

// ---------------------------------------------------------------------------
// 汇总
// ---------------------------------------------------------------------------

const total = passed + failures.length;
console.log(`\n${"=".repeat(60)}`);
if (failures.length === 0) {
  console.log(`全绿：${passed}/${total} 项通过（口径回归脚本）`);
  process.exit(0);
}
console.log(`失败：${failures.length}/${total} 项未通过，通过 ${passed} 项`);
for (const label of failures) console.log(`  - ${label}`);
process.exit(1);
