#!/usr/bin/env node
/**
 * 面板口径的**渲染层**回归：用真实组件产出 DOM，断言文案与数值真的渲染出来（零新增依赖）
 *
 * 与 `period-parity.mjs`（纯逻辑断言）互补：那份证明"算得对"，这份证明"显示得对"——
 * 渲染的是 `frontend/src/components/Node.tsx` 的**真组件**（不是复制品），
 * 因此能抓住"逻辑对了但 UI 没接上 / 文案键缺失显示原始 key / 出现 NaN"这类回归。
 *
 * 实现要点（为什么零新增依赖）：
 *   - `Node.tsx` 是 TSX，Node 的类型擦除不转换 JSX ⇒ 用 **esbuild**（vite 的既有依赖，
 *     `frontend/node_modules/esbuild`）把它连同本仓库的 `react` / `react-dom` / `i18next`
 *     打成一个自包含 ESM 到临时目录，再由本脚本 import 执行；
 *   - 语言包用仓库里的 5 个 JSON（en / zh_CN / zh_TW / ja_JP / id_ID）；
 *   - 不装任何测试框架、不写仓库内文件（临时产物写在 os.tmpdir()，跑完删除）。
 *
 * 用法：
 *   cd frontend && npm run test:period-dom
 *   cd frontend && node script/period-panel-dom.mjs
 *
 * 退出码：0 = 全绿；1 = 存在断言失败；3 = 环境不满足（缺 node_modules/esbuild，先 `npm ci`）。
 */

import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { fileURLToPath, pathToFileURL } from "node:url";

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const FRONTEND_DIR = path.resolve(SCRIPT_DIR, "..");
const ENTRY_FILE = path.join(SCRIPT_DIR, "period-panel-dom-entry.tsx");

const GiB = 1024 ** 3;
const TiB = 1024 ** 4;

// ---------------------------------------------------------------------------
// 极简断言框架
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

/** DOM → 纯文本：去标签、解几个常见实体、收敛空白（断言只看人眼能看到的文字）。 */
function domText(html) {
  return String(html)
    .replace(/<[^>]*>/g, " ")
    .replace(/&#x27;/g, "'")
    .replace(/&quot;/g, '"')
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/\s+/g, " ")
    .trim();
}

// ---------------------------------------------------------------------------
// 打包渲染入口（esbuild 是 vite 的既有依赖；缺它就说明没装依赖）
// ---------------------------------------------------------------------------

async function loadEsbuild() {
  try {
    return await import("esbuild");
  } catch (error) {
    console.error("[period-panel-dom] 无法加载 esbuild（vite 的既有依赖）");
    console.error("  先安装前端依赖：cd frontend && npm ci");
    console.error(`  原始错误：${error?.message ?? error}`);
    process.exit(3);
  }
}

const esbuild = await loadEsbuild();
const outDir = fs.mkdtempSync(path.join(os.tmpdir(), "period-panel-dom-"));
const outFile = path.join(outDir, "render-bundle.mjs");

let renderScenarios;
try {
  await esbuild.build({
    entryPoints: [ENTRY_FILE],
    outfile: outFile,
    bundle: true,
    format: "esm",
    platform: "node",
    target: "node20",
    jsx: "automatic",
    loader: { ".css": "empty" },
    alias: { "@": path.join(FRONTEND_DIR, "src") },
    logLevel: "silent",
    // react-dom/server 是 CJS，内部会 `require("util")` 等 Node 内建模块；
    // 打成一个自包含 ESM 时 esbuild 的 `__require` 会抛 "Dynamic require ... is not supported"，
    // 因此在顶部注入一个真正的 require（esbuild 官方 issue 里的标准做法）。
    banner: {
      js: 'import { createRequire as __periodCreateRequire } from "node:module"; const require = __periodCreateRequire(import.meta.url);',
    },
  });
  ({ renderScenarios } = await import(pathToFileURL(outFile).href));
} finally {
  fs.rmSync(outDir, { recursive: true, force: true });
}

// ---------------------------------------------------------------------------
// 夹具：与 period-parity.mjs 同一批口径场景
// ---------------------------------------------------------------------------

const baseNode = {
  uuid: "11111111-2222-3333-4444-555555555555",
  name: "demo-node",
  cpu_name: "CPU",
  virtualization: "kvm",
  arch: "amd64",
  cpu_cores: 4,
  os: "linux",
  kernel_version: "6.1.0",
  gpu_name: "",
  region: "CN",
  mem_total: 8 * GiB,
  swap_total: 0,
  disk_total: 100 * GiB,
  version: "0.0.24",
  weight: 0,
  price: 0,
  tags: "",
  billing_cycle: 0,
  currency: "CNY",
  group: "",
  traffic_limit: 0,
  traffic_limit_type: "sum",
  traffic_reset_day: 0,
  traffic_reset_timezone: "",
  expired_at: "",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-10-23T08:00:00Z",
  ipv4: "10.0.0.1",
  ipv6: "",
};

const liveRecord = (network) => ({
  cpu: { usage: 12.5 },
  ram: { used: 2 * GiB },
  swap: { used: 0 },
  load: { load1: 0.1, load5: 0.2, load15: 0.3 },
  disk: { used: 10 * GiB },
  network: { up: 1024, down: 2048, ...network },
  connections: { tcp: 3, udp: 1 },
  uptime: 3600,
  process: 100,
  message: "",
  updated_at: "2026-10-23T08:00:00Z",
});

const periodNode = {
  ...baseNode,
  traffic_limit: TiB,
  traffic_limit_type: "sum",
  traffic_reset_day: 22,
  traffic_reset_timezone: "Asia/Shanghai",
};

const scenarios = [
  {
    name: "存量节点：周期已配置 + period_started_at 为空",
    node: periodNode,
    live: liveRecord({
      totalUp: 544.75 * GiB,
      totalDown: 0,
      periodUp: 0,
      periodDown: 0,
      periodStartedAt: null,
    }),
    checks(text) {
      check("显示「本周期流量」（口径为周期）", text.includes("本周期流量"));
      check(
        "显示「本周期尚未开始」且进度条 0.0%",
        text.includes("本周期尚未开始") && text.includes("0.0%"),
      );
      check(
        "显示「累计（历史）」并保留累计值 544.75 GB",
        text.includes("累计（历史）") && text.includes("544.75 GB"),
      );
      check(
        "重置说明含节点配置的时区名与重置日",
        text.includes("每月 22 日 00:00（Asia/Shanghai）重置"),
      );
      check("不再出现旧的「总流量」标签", !text.includes("总流量"));
    },
  },
  {
    name: "未配置周期（traffic_reset_day = 0）⇒ 与升级前一致",
    node: { ...periodNode, traffic_reset_day: 0, traffic_reset_timezone: "" },
    live: liveRecord({
      totalUp: 544.75 * GiB,
      totalDown: 0,
      periodUp: 0,
      periodDown: 0,
      periodStartedAt: null,
    }),
    checks(text) {
      check("标签仍是「总流量」", text.includes("总流量"));
      check("百分比 = 53.2%（与升级前同一公式）", text.includes("53.2%"));
      check(
        "不出现本周期文案 / 重置说明 / 口径标注",
        !text.includes("本周期") &&
          !text.includes("重置") &&
          !text.includes("口径已切换"),
      );
    },
  },
  {
    name: "存量超额：累计 600GiB ≥ 限额 500GiB，周期 0.02GiB",
    node: { ...baseNode, traffic_limit: 500 * GiB, traffic_limit_type: "sum", traffic_reset_day: 22, traffic_reset_timezone: "UTC" },
    live: liveRecord({
      totalUp: 600 * GiB,
      totalDown: 0,
      periodUp: 0.02 * GiB,
      periodDown: 0,
      periodStartedAt: "2026-10-22T00:00:00Z",
    }),
    checks(text) {
      check("显示「口径已切换」标注", text.includes("口径已切换"));
      check(
        "百分比按周期值（0.0%，不是累计的 120%）",
        text.includes("0.0%") && !text.includes("120.0%"),
      );
      check(
        "累计（历史）仍显示 600.00 GB",
        text.includes("累计（历史）") && text.includes("600.00 GB"),
      );
      check("空时区 ⇒ 重置说明用 UTC", text.includes("（UTC）重置"));
      check("本周期数值来自服务端周期字段", text.includes("20.48 MB"));
    },
  },
  {
    name: "周期已开始：本周期 25GiB / 限额 100GiB",
    node: { ...baseNode, traffic_limit: 100 * GiB, traffic_limit_type: "sum", traffic_reset_day: 1, traffic_reset_timezone: "Asia/Tokyo" },
    live: liveRecord({
      totalUp: 80 * GiB,
      totalDown: 10 * GiB,
      periodUp: 20 * GiB,
      periodDown: 5 * GiB,
      periodStartedAt: "2026-10-01T00:00:00Z",
    }),
    checks(text) {
      check("百分比 = 25.0%（按周期值）", text.includes("25.0%"));
      check(
        "周期上行/下行 = 20.00 GB / 5.00 GB",
        text.includes("20.00 GB") && text.includes("5.00 GB"),
      );
      check(
        "累计（历史）= 80.00 GB",
        text.includes("累计（历史）") && text.includes("80.00 GB"),
      );
      check(
        "累计未超额 ⇒ 不出现「口径已切换」",
        !text.includes("口径已切换"),
      );
      check(
        "重置说明用该节点的时区 Asia/Tokyo",
        text.includes("每月 1 日 00:00（Asia/Tokyo）重置"),
      );
    },
  },
  {
    name: "起点是垃圾串（防御性）",
    node: periodNode,
    live: liveRecord({
      totalUp: 544.75 * GiB,
      totalDown: 0,
      periodUp: 0,
      periodDown: 0,
      periodStartedAt: "not-a-date",
    }),
    checks(text) {
      check(
        "按「本周期尚未开始」处理",
        text.includes("本周期尚未开始") && text.includes("0.0%"),
      );
      check("累计（历史）仍展示", text.includes("累计（历史）"));
    },
  },
  {
    name: "英文语言包（locale = en）",
    locale: "en",
    node: periodNode,
    live: liveRecord({
      totalUp: 544.75 * GiB,
      totalDown: 0,
      periodUp: 0,
      periodDown: 0,
      periodStartedAt: null,
    }),
    checks(text) {
      check("英文「Cycle Traffic」", text.includes("Cycle Traffic"));
      check("英文「Lifetime (history)」", text.includes("Lifetime (history)"));
      check(
        "英文「This cycle hasn't started yet」",
        text.includes("This cycle hasn't started yet"),
      );
      check(
        "英文重置说明含时区名",
        text.includes("Resets on day 22") && text.includes("Asia/Shanghai"),
      );
    },
  },
];

// 5 个语言包：每个都要能渲染出该语言的本周期标签（缺键会退化成原始 key）
const localeExpectations = [
  ["en", "Cycle Traffic"],
  ["zh_CN", "本周期流量"],
  ["zh_TW", "本週期流量"],
  ["ja_JP", "今期の通信量"],
  ["id_ID", "Trafik Siklus"],
];

for (const [locale, expectedLabel] of localeExpectations) {
  scenarios.push({
    name: `语言包 ${locale}：本周期标签渲染为该语言文案`,
    locale,
    node: periodNode,
    live: liveRecord({
      totalUp: 1 * GiB,
      totalDown: 0,
      periodUp: 0,
      periodDown: 0,
      periodStartedAt: null,
    }),
    checks(text) {
      check(`出现「${expectedLabel}」`, text.includes(expectedLabel));
      check(
        "没有退化成原始 key（nodeCard.*）",
        !/nodeCard\./.test(text),
      );
    },
  });
}

// ---------------------------------------------------------------------------
// 渲染 + 断言
// ---------------------------------------------------------------------------

section("渲染真实 Node 组件（SSR）并断言 DOM 文案");

const rendered = await renderScenarios(scenarios);
const byName = new Map(rendered.map((item) => [item.name, domText(item.html)]));

for (const scenario of scenarios) {
  section(`场景：${scenario.name}`);
  const text = byName.get(scenario.name);
  check("场景已渲染出内容", Boolean(text && text.length > 0));
  if (!text) continue;
  scenario.checks(text);

  // 全局护栏：不得出现 NaN / Infinity / undefined / 负数值
  check(
    "DOM 文本不含 NaN",
    !/NaN/.test(text),
    /NaN/.test(text) ? text : "",
  );
  check("DOM 文本不含 Infinity", !/Infinity/.test(text));
  check("DOM 文本不含 undefined", !/undefined/.test(text));
  check(
    "DOM 文本不含负数值",
    !/(^|[^\w])-\d/.test(text),
    (text.match(/(^|[^\w])-\d\S*/) ?? [""])[0],
  );
}

// 打印一个场景的可见文本，便于失败时肉眼核对（成功时也有据可查）
section("样例 DOM 文本（第一个场景）");
console.log(`  ${byName.get(scenarios[0].name)}`);

// ---------------------------------------------------------------------------
// 汇总
// ---------------------------------------------------------------------------

const total = passed + failures.length;
console.log(`\n${"=".repeat(60)}`);
if (failures.length === 0) {
  console.log(`全绿：${passed}/${total} 项通过（渲染层口径断言）`);
  process.exit(0);
}
console.log(`失败：${failures.length}/${total} 项未通过，通过 ${passed} 项`);
for (const label of failures) console.log(`  - ${label}`);
process.exit(1);
