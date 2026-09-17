# Trellis 闸门指南（强制规则怎么落地、怎么自证）

> 用户 2026-09-17 定调：**调用 Trellis 不是口头承诺，而是强制规则**。
> 本文写"规则是什么、机器怎么拦、我该怎么自证没跳过"。规则原文见 `AGENTS.md`「强制规则」，
> 维护者视角见 `docs/MAINTAINING.md` §2.2。

---

## 1. 规则（最严范围）

**任何会话工作——包括只读调查——开工第一步必须先建 Trellis 任务。**
"只读调查"指：看代码、查日志、读数据库定位原因、跑诊断命令，全都算。

唯一例外：用户明确说"这次跳过 Trellis"。没有第二条例外；不要自己判断"这次很小所以不用"。

标准开场（3 条命令，之后才允许读代码/改文件）：

```bash
python3 .trellis/scripts/task.py create "<标题>" --slug <slug> -d "<描述>"
# 复杂任务补 prd.md / implement.md，然后：
python3 .trellis/scripts/task.py validate <MM-DD-slug>
python3 .trellis/scripts/task.py start <MM-DD-slug>
```

收尾：`task.py finish` → `task.py archive <name> --skip-branch-validation` → 写 journal。

## 2. 机器怎么拦（三层）

| 层 | 文件 | 触发点 | 违规后果 |
|---|---|---|---|
| ① 当场拦 | `.githooks/pre-commit` | `git commit` 且暂存区含非 `.trellis/` 改动、却无进行中任务 | 提交被拒，打印建任务与提交命令 |
| ① 当场拦 | `.githooks/commit-msg` | 同上条件、消息里没有 `[task:<slug>]` 或 slug 不存在 | 提交被拒，打印正确格式与现有任务 |
| ② 事后审计 | `scripts/check-trellis-gate.sh`（`check-repo.sh` 第 8 项） | `./scripts/check-repo.sh [--full]` | ✗ 列出违规提交，检查失败（发版前必跑） |
| ③ 远程兜底 | `.github/workflows/trellis-gate.yml` | push 到 main / 开 PR | GitHub 上 workflow 红叉 |

新克隆必须跑一次 `./scripts/install-git-hooks.sh`（`core.hooksPath` 是本地配置）。

## 3. 消息格式

```
<type>(<scope>): <一句话说明> [task:<slug>]
```

slug 是 `.trellis/tasks/<MM-DD>-<slug>` 去掉日期前缀的部分。例：

```
feat(upgrade): 容器内替换后原地重执行 [task:container-inplace-self-upgrade]
```

## 4. 刻意放行

- **纯 `.trellis/` 改动**：journal、任务归档、`implement.jsonl`、闸门自身 —— 流程产物不该被拦；
- **merge 提交**：被合并的提交各自有审计，合并动作不产生新改动。

## 5. 绕过与代价（诚实写）

`git commit --no-verify` 是 git 内建能力，**封不死**。闸门的设计不是"禁止绕过"，而是
**绕过必留痕**：第 ② / ③ 层审计逐个提交核对，任何漏锚点的提交都会在下一次 `check-repo`
或下一次 push 时暴露，并阻断发版。**不许把 `--no-verify` 当常规手段。**

## 6. 自证清单（每个任务收尾前自问）

- [ ] 任务是在**动手之前**建的（不是事后补的）——看 `task.json` 的 `createdAt` 与首个改动提交的时间关系；
- [ ] 每个改动提交都带 `[task:<slug>]`（`git log --format='%h %s' <起点>..HEAD` 自查）；
- [ ] `./scripts/check-trellis-gate.sh` 通过；
- [ ] 任务已 `finish` + `archive`，journal 有本次会话记录；
- [ ] 如果为了排查"只读调查"而没建任务——那是违规，别在 journal 里粉饰成"顺手看的"。

## 7. 已知边界（不许假装全自动）

机器只能强制"**提交时**必须有任务"。只读调查不产生提交，因此"只读也要建任务"这条
**没有机械保障**，靠本指南 + `AGENTS.md` + journal 留痕。写文档或向用户汇报时，
必须照实说，不要声称闸门覆盖了一切。
