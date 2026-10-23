# 面板口径收尾：渲染层证据入库 + 可选门禁接入

## 背景

面板/限额口径已切到服务端周期值并随 0.0.24 上线；逻辑层回归已入库
（`frontend/script/period-parity.mjs`，`npm run test:period`，69/69）。仍有三个可选收尾项。

## 待决项

1. **渲染层证据入库**：当前"「本周期尚未开始」「累计（历史）」「口径已切换」真的渲染出来、且无 NaN"
   的 SSR/DOM 断言仍在临时目录（`/tmp/period-panel-parity/`），**不在仓库**。
   可行方案：`frontend/script/period-panel-dom.mjs`（`react-dom/server` + 既有依赖，零新增包，约 150 行）。
2. **门禁接入**：是否把 `npm run test:period` 接进 `scripts/check-repo.sh`（或 pre-commit）。
   需考虑：Node 版本不足（< 22.18）时的跳过策略（脚本退出码 2 表示环境不满足）、CI 里是否装 Node、
   以及**判别性验证**（故意改坏一条断言 ⇒ 门禁必须报红）。
3. **文案/配色微调**：按用户实际观感调整（如需）。

## Acceptance Criteria

- [ ] 每项有明确结论（做/不做 + 理由）；
- [ ] 若做 1：`node frontend/script/period-panel-dom.mjs` 退出码 0、打印断言计数；故意改坏一条 ⇒ 非零退出（红→绿证据）；
- [ ] 若做 2：`check-repo.sh` 全绿；判别性验证（改坏断言 ⇒ 报红）；Node 缺失/过旧时行为明确（跳过并提示，不静默通过）；
- [ ] 不引入新依赖、不改 `frontend/src/**` 实现逻辑。

## Constraints

- 提交带 `[task:period-panel-hardening]`；文档与提交脱敏；
- 不碰生产；发版另行决定。
