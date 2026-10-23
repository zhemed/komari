import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import i18next from "i18next";
import { initReactI18next } from "react-i18next";
import NodeCard from "@/components/Node";
import en from "@/i18n/locales/en.json";
import zhCN from "@/i18n/locales/zh_CN.json";
import zhTW from "@/i18n/locales/zh_TW.json";
import jaJP from "@/i18n/locales/ja_JP.json";
import idID from "@/i18n/locales/id_ID.json";

/**
 * period-panel-dom.mjs 的渲染入口（**不是**运行时组件，只服务于渲染层断言）。
 *
 * 为什么要单独一个入口：`Node.tsx` 是 TSX，Node 的类型擦除不会转换 JSX，
 * 所以必须先用 esbuild（vite 的既有依赖，不新增包）把它打成一个 ESM 模块，
 * 再由 `period-panel-dom.mjs` 在内存里调用本文件导出的 `renderScenarios()`。
 *
 * 这里刻意 `import` 真实的 `@/components/Node`——断言的对象是**真组件**，
 * 不是复制出来的近似实现。
 */

let ready = null;

/** 初始化 i18next（用仓库里的 5 个语言包，不触碰浏览器 API）。 */
export function ensureI18n() {
  if (!ready) {
    ready = i18next.use(initReactI18next).init({
      lng: "zh_CN",
      fallbackLng: "en",
      resources: {
        en: { translation: en },
        zh_CN: { translation: zhCN },
        zh_TW: { translation: zhTW },
        ja_JP: { translation: jaJP },
        id_ID: { translation: idID },
      },
      interpolation: { escapeValue: false },
    });
  }
  return ready;
}

/**
 * 逐个场景渲染真实节点卡片，返回 { name, html }[]。
 * 场景数据由调用方（period-panel-dom.mjs）给出，便于把断言与夹具放在一起。
 */
export async function renderScenarios(scenarios) {
  await ensureI18n();
  const rendered = [];
  for (const scenario of scenarios) {
    if (scenario.locale) await i18next.changeLanguage(scenario.locale);
    const html = renderToStaticMarkup(
      <MemoryRouter>
        <NodeCard
          basic={scenario.node}
          live={scenario.live}
          online={scenario.online ?? true}
          isMobile={scenario.isMobile ?? false}
          showIpTagsInCard={false}
        />
      </MemoryRouter>,
    );
    rendered.push({ name: scenario.name, html });
  }
  return rendered;
}
