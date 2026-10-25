# Komari ink 主题：三网卡片改按地区分行（延迟+丢包+历史条）本地小样

## Goal

用户 2026-10-25 定调（主仓任务 10-25-komari-theme-region-format）：ink 主题首页节点卡片把
「三网」行（只显示延迟数字）与「延迟/丢包」两块聚合卡，**完全替换**为按 ping 任务分行的布局：
每行 = 任务名 + 延迟 ms + 丢包% + 该任务自己的历史点阵。本轮只做**本地小样**：
改源码 + type-check + build + 本地截图审阅；**不部署、不改服务器**。

## Requirements

1. 修改权威源码 `/root/ssh/komari-src/themes/ink/src` 中 3 个文件：
   - `composables/useNodePingStats.ts`：`NodePingTaskLatency` 增加 `loss`/`lossApproximate`/每任务 `history`；
     metric 路径从 `stat.loss` 取每任务丢包、按 `task_id` 过滤 records 生成每任务 20 桶历史；
     legacy 路径把已算出的每任务丢包挂回；`CACHE_VERSION` 9→10 并更新缓存校验。
   - `composables/useNodePingDisplay.ts`：新增按任务行数据（label/延迟文本/丢包文本/每任务 bars），
     复用 `getLatencyToneClass`、DataTooltip 与现有文案；保留原导出不破坏其它调用方。
   - `components/NodeCard.vue`：用行列表替换 `:461-483` 三网行与 `:485-571` 两块卡；整行可点击
     `emit('pingClick')`；离线 blur/opacity 保留；最多 3 行；无数据时保留一个加载/占位行。
2. 行标签来自后台 ping 任务 `name`，不硬编码地区名；不新增文案。
3. 边界：任务 0/1/2/3 个、无样本、100% 丢包、`loss_approximate`、legacy 回退、旧缓存、移动端宽度。

## Constraints

- 只改上述 3 个文件；不引入依赖；不改测试 fixture/snapshot（截图由 Lead 在审阅后生成）。
- 不联网、不 ssh、不读工作区外路径、不部署、不改服务器。
- 不 git commit/push、不调用 task.py（Trellis 记账由 Lead 负责）。
- 不自造文案（历史硬规矩）。

## Acceptance Criteria

- [ ] `bun run type-check` 与 `bun run build-only` 均通过。
- [ ] `git -C /root/ssh/komari-src status --porcelain` 仅显示这 3 个源码文件（+构建产物忽略项）。
- [ ] 行布局、点击弹窗、离线态、空态、色觉友好色阶均保留。
- [ ] 不硬编码地区名、不新增文案、无后端改动。

## Notes

- 主仓任务：`.trellis/tasks/10-25-komari-theme-region-format/`（评估 + 用户授权记录）。
- 线上已核对（只读）：3 个 ping 任务、名称格式「<地区><运营商>」、weight 有序、30s tcp、主题 0.0.1。
