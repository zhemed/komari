# 主题独立维护加固：防后台覆盖 + 视觉快照更新（komari-src 侧）

## Goal

主仓任务 `10-25-theme-maintenance-closeout` 的 komari-src 落地部分：
① 按调查结论对 ink 主题做**防覆盖加固**（管理面板「主题→更新」与「主题市场 Update」两条路径，
   防止把自维护的 0.0.2 覆盖回上游 0.6.x）；
② 更新因新版式失效的 **3 张 Playwright 视觉基线快照**并验证；
③ 提交（含配套 manifest/文档改动），配合 Lead 完成 push 与收尾。
**不改服务端、不重建容器、不重新部署线上主题。**

## Requirements

1. 加固方案须由只读调查结论支撑（`UpdateTheme` 对 manifest `url` 各形态的行为、
   主题市场版本比较与覆盖路径、可用的禁用/隐藏开关）；**不破坏**日常部署脚本与「导入主题」。
2. 加固优先选择「面板更新安全失败 / 按钮不可用 / 市场不提示可更新」中最小且可验证的一种或组合；
   若需调整版本号策略（例如跳出 0.0.x 线），先说明影响并由 Lead 确认。
3. 更新 `themes/ink/tests/visual/snapshots/chromium/` 下受版式影响的快照；
   在本地用可用浏览器跑通 `test:visual`（记录 Playwright 版本与浏览器方案）。
4. 提交需带 `[task:10-25-theme-independent-hardening]`；改动范围限主题元数据/测试快照/必要文档。
5. 不联网安装新浏览器（除非 Lead 另行授权）；不 push（push 由 Lead 执行）。

## Constraints

- 不改服务端 Go 代码（除非调查证明必须且 Lead 同意）。
- 不写站点专属值；不修改 git 持久配置。
- 不启动/不动唯一服务器。

## Acceptance Criteria

- [ ] 两条覆盖路径均有可验证的加固结果 + 证据（代码/配置/版本判定说明）。
- [ ] 3 张视觉快照已更新，`test:visual` 本地通过（记录方案）。
- [ ] 改动已提交、工作树干净；任务归档留痕。
- [ ] 未部署、未重建服务端。

## Notes

- 版本线：主题自有 0.0.x（`ops-docs/VERSION.md`），服务端 `0.0.24+own.4`。
- 前序：`theme-002-release`（0.0.2 发布）。
