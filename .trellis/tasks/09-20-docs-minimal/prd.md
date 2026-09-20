# 文档精简到极限

用户（2026-09-20）：「太罗嗦了，反正就我们自己用自己维护，干脆精简到极限」。

## 结果

| 文件 | 之前 | 现在 |
|---|---|---|
| `README.md` | 145 → 94 行 | **36 行**（部署命令 + agent + 自检 + 一处指向） |
| `docs/MAINTAINING.md` | 877 行 | **98 行**（只留操作：版本口径/构建/发版/部署口径/数据与回滚/agent/自检/本地产物） |
| `docs/CLEANUP-2026-09-19.md` | 105 行 | **删除**（内容已被 `.build/purge-manifest-20260919.txt` 与 journal 覆盖） |
| 文档合计 | 1127 行 | **134 行**（−88%） |

README 只留：标题+一行定位、Docker 部署命令（主方案）、agent 两条命令、自检一条、
其余指向 `docs/MAINTAINING.md` 与 `.trellis/spec/`。

## 连带修复（精简不能留死引用）

- spec 里 9 处 `docs/MAINTAINING.md §x.y` 章节引用因章节重排而悬空 → 全部改成内容引用
  （如「发布一个版本」「部署口径」）或删除；复查剩余悬空引用 = 0。
- `check-repo.sh` 两处判据改为"断言内容而非编号"：
  - 第 1 项：`当前[^0-9]{0,4}<版本>`（不再绑定 `当前 **x**` 写法）
  - 第 14 项：MAINTAINING 必须含「主方案：Docker 镜像」与「compose 一律不用」（不再绑定 §3.4.1）
- 删掉 MAINTAINING 里 800 行叙事/历史/矩阵的处置说明：那些属于代码侧规范，已在
  `.trellis/spec/`（server-upgrade/build-and-pinning/incident-compose-autosync 等）里，不再重复。

## Acceptance Criteria

- [x] README ≤ 40 行且只含"用得上"的内容；compose 零命中
- [x] MAINTAINING ≤ 100 行；Docker 主方案与 compose 禁令仍在（自检第 14 项守住）
- [x] 悬空章节引用清零
- [x] `check-repo.sh --full` 14 项全绿
- [x] 未碰生产与线上发布物
