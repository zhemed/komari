# Workspace Index - komari

> Journal tracking for AI development sessions.

---

## Current Status

<!-- @@@auto:current-status -->
- **Active File**: `journal-1.md`
- **Total Sessions**: 53
- **Last Active**: 2026-09-20
<!-- @@@/auto:current-status -->

---

## Active Documents

<!-- @@@auto:active-documents -->
| File | Lines | Status |
|------|-------|--------|
| `journal-1.md` | ~1825 | Active |
<!-- @@@/auto:active-documents -->

---

## Session History

<!-- @@@auto:session-history -->
| # | Date | Title | Commits | Branch |
|---|------|-------|---------|--------|
| 53 | 2026-09-20 | 构建并发布 0.0.19（release 18 资产 + 双镜像 :latest 迁移） | `b4ba1e8` | `main` |
| 52 | 2026-09-20 | 删除面板里提到 compose 的提示文案并重建前端产物 | `abd30cf` | `main` |
| 51 | 2026-09-20 | 文档精简到极限（README 36 行 / MAINTAINING 98 行） | `c2a55bf` | `main` |
| 50 | 2026-09-20 | 修：README 删减未落盘（被自己的验证动作覆盖） | `b6a0c68` | `main` |
| 49 | 2026-09-20 | 纠正部署口径：Docker 镜像为主方案，README 删减，compose 剔除 | `9fb8a86` | `main` |
| 48 | 2026-09-19 | 定稿唯一部署路径：install-komari.sh（用户验收后） | `d43ce46` | `main` |
| 47 | 2026-09-19 | 按用户决定保留 0.0.18，并用仓库部署命令把生产升上去 | `4b24cc2` | `main` |
| 46 | 2026-09-19 | compose 事故复盘：把最严重的失误写死成规范 | `613c143` | `main` |
| 45 | 2026-09-19 | 全面整理：垃圾清零 + 一致性对齐 + 自检加自测 | `f65d004` | `main` |
| 44 | 2026-09-19 | 回答：0.0.18 修复了什么（只读核对） | - | `main` |
| 43 | 2026-09-19 | 全面审查 + 全面回滚（compose 弧线 12 提交/19 文件 → 0.0.17；生产二进制回 0.0.17） | `fb66759` | `main` |
| 42 | 2026-09-19 | 仓库侧回滚 compose 主推地位（systemd 回到方式一） | `69e5d14` | `main` |
| 41 | 2026-09-19 | 回滚部署：compose → systemd（0.0.18 二进制 + 数据同步） | `c555fb0` | `main` |
| 40 | 2026-09-19 | 回滚 Dockerfile 基础镜像钉版（回到 alpine:3.21 tag） | `bd92c64` | `main` |
| 39 | 2026-09-19 | README 恢复「源码构建」段落 | `50fb61f` | `main` |
| 38 | 2026-09-18 | install-compose 备份名唯一化（修同一秒连跑互相覆盖） | `f9e8520` | `main` |
| 37 | 2026-09-18 | install-compose 备份治理：摘要提示 + 保留最近 3 份（历史名备份不动） | `86130f8` | `main` |
| 36 | 2026-09-18 | install-compose.sh 三处改进（干跑跳过预检 / --force 不并存 / 迁移提示补重建） | `7eae40e` | `main` |
| 35 | 2026-09-18 | compose 文件名核实并切到首选名 compose.yaml | `f263484` | `main` |
| 34 | 2026-09-18 | 发布 0.0.18 + 生产升级（B 策略真正生效） | `4479580` | `main` |
| 33 | 2026-09-18 | README 大面积删减（175 → 53 行） | `5d3bf9b` | `main` |
| 32 | 2026-09-18 | 重启策略评估 + 一条命令部署（install-compose.sh） | `d907c6e` | `main` |
| 31 | 2026-09-18 | compose tag 自动同步 + host 网络下识别自身容器（0.0.18 待发） | `dd0486a` | `main` |
| 30 | 2026-09-18 | 生产切换到 compose（/opt/docker/komari，0.0.17，升级策略 B） | `6ed7fbd` | `main` |
| 29 | 2026-09-18 | 定稿 Compose 部署形态并入仓（升级策略 B + 实测 compose 交互三条） | `644169c` | `main` |
| 28 | 2026-09-18 | 日志上限取值：实测日志速率并给出 max-size/max-file 建议 | - | `main` |
| 27 | 2026-09-18 | Docker Compose 部署方案审查（真实镜像逐条验证） | - | `main` |
| 26 | 2026-09-18 | 去掉安装/改密口令的强度与长度校验（发布 0.0.17） | `3698480` | `main` |
| 25 | 2026-09-17 | 仓库体检：清残留 + 瘦身本地产物 + 钉死镜像基线与测试门控 + 前端死代码 | `fc1b597`, `dc61273` | `main` |
| 24 | 2026-09-17 | 强制规则推广到全局（~/.dsh/AGENTS.md）：有 .trellis/ 的项目全部适用 | `30e5c2c` | `main` |
| 23 | 2026-09-17 | Trellis 强制闸门：提交时拦截 + 事后审计 + GitHub CI 兜底 | `922e1a9`, `afa8fb9` | `main` |
| 22 | 2026-09-17 | 文档一致性：容器部署命令去掉 socket 挂载，统一零配置升级口径 | `a5c53fa` | `main` |
| 21 | 2026-09-17 | 修"点升级后卡住、必须手动刷新"（0.0.16） | `585f02d` | `main` |
| 20 | 2026-09-17 | 去掉升级弹窗里的 Github 按钮（0.0.15） | `b6bc4d7` | `main` |
| 19 | 2026-09-17 | 容器零配置网页升级（0.0.13 引入，0.0.14 修两个缺陷） | `247aa9f`, `5b478ee` | `main` |
| 18 | 2026-09-17 | 容器一键升级：docker socket + helper 重建容器（0.0.11 引入，0.0.12 修缺陷） | `c663d96`, `c434af7` | `main` |
| 17 | 2026-09-17 | 容器形态暴露可复制的升级命令（发布 0.0.10） | `f24b743`, `b976210` | `main` |
| 16 | 2026-09-17 | 面板一键升级服务器（0.0.8 功能 + 0.0.9 修 E2E 抓到的缺陷） | `c671364`, `6421ec9`, `8dea6a6` | `main` |
| 15 | 2026-09-17 | 补建 Trellis 任务 + 更正未验证因果结论（流程违规整改） | `245bb91`, `ec3deb1` | `main` |
| 14 | 2026-09-17 | 定位并修复流量图点值被除以采样条数（发布 0.0.7） | `48dca2e`, `fc7d6e8`, `2109504` | `main` |
| 13 | 2026-09-17 | 发布 0.0.6（流量跨重启）+ 本机部署改用 release 资产验证 | `7cbd25d`, `5b5f5c5` | `main` |
| 12 | 2026-09-17 | Session 11: 流量历史跨重启保存（服务端持久累计）+ 修 v2 上报缺 uptime 的增量清零 | `10e5605`, `df4f08f` | `main` |
| 11 | 2026-09-16 | Session 10: 全面维护——说明口径换成我们自己的版本 + 新增仓库自检脚本 | `8cc30ef`, `1ca69c4`, `f1186a2` | `main` |
| 10 | 2026-09-16 | Session 9: 前端与 agent 源码全部 vendor 进仓库（完全自有） | `bfb96ad`, `ea28dc0`, `7759a77` | `main` |
| 9 | 2026-09-16 | Session 8: agent 从 1.5.10 退回 1.4.3 同期血统（发 0.0.5），motd 告警事件复盘 | `3ab871e`, `7b61d17` | `main` |
| 8 | 2026-09-16 | Session 7 收尾: 镜像公开验证通过（README 的 docker run 已可匿名使用） | `fc7cf35`, `dbc0410` | `main` |
| 7 | 2026-09-16 | Session 7: README 改成产品视角短文 + 发布服务器镜像 ghcr.io/zhemed/komari | `fc7cf35` | `main` |
| 6 | 2026-09-16 | Session 6: agent 自有发行线（0.0.4）——单仓库发布 agent、默认关自更新、自建镜像 | `2ba82cc`, `1a2288d`, `5d047d8`, `4e88360` | `main` |
| 5 | 2026-09-16 | 彻底移除通知系统与内嵌 JS 运行时，发布 0.0.3 | `b397115` | `main` |
| 4 | 2026-09-16 | 解除内网地址限制、移除 HTTPS 横幅并发布 0.0.2 | `241dddb` | `main` |
| 3 | 2026-09-16 | 重写历史为自有基线并发布 0.0.1 | `45f9f1c`, `e3a104f` | `main` |
| 2 | 2026-09-16 | 0.0.1 自有基线 + 彻底移除插件系统 | `18305a9` | `komari-1.4.3` |
| 1 | 2026-09-16 | 锁定 komari 1.4.3 为自维护分叉基线 | `e70304f`, `5942199` | `komari-1.4.3` |
<!-- @@@/auto:session-history -->

---

## Notes

- Sessions are appended to journal files
- New journal file created when current exceeds 2000 lines
- Use `add_session.py` to record sessions