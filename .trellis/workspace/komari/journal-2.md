# Journal - komari (Part 2)

> Continuation from `journal-1.md` (archived at ~2000 lines)
> Started: 2026-09-20

---



## Session 61: 升级弹窗按钮去重并发布 0.0.21（含生产升级）
<!-- trellis-session: v=2 fp=0b3eb5013c940731 -->

**Date**: 2026-09-20
**Task**: 升级弹窗按钮去重并发布 0.0.21（含生产升级）
**Branch**: `main`

### Summary

用户发现弹窗列表首项的「安装此版本」与底部按钮指向同版。核实确认（都来自 latestRelease），按用户选择删除列表里的按钮、只留底部，发 0.0.21 并升级生产。顺带修掉两处既有问题：MAINTAINING 版本表仍写 0.0.18（两次升级漏改）、自检第 1 项匹配写法形同虚设（并踩到 backtick 被 shell 命令替换的坑）。

### Main Changes

- AdminPanelBar.tsx 删除列表每行按钮；保留底部按钮两种形态与全部状态变量用法
- 版本 → 0.0.21；前端哈希 f4785c5b…；release 18 资产；镜像 :0.0.21/:latest
- MAINTAINING §1 显式声明当前版本；check-repo 第 1 项对准该行（backtick 已转义）
- 生产升级到 0.0.21（备份 komari.backup.20260920_031334）

### Git Commits

| Hash | Message |
|------|---------|
| `3977f5a` | fix(docs+check): MAINTAINING 显式声明当前版本，自检第 1 项对准它 [task:dedupe-upgrade-dialog-buttons] |

### Testing

- [OK] 生产 version=0.0.21 且二进制 sha256 与 release 资产一致；产物无 install_version 调用点；面板 200；agent online；日志 0 错误
- [OK] check-repo.sh --full 全绿；release digest 与本地产物逐一一致

### Status

[OK] **Completed**

### Next Steps

- 可选：清掉语言包里已无用的 upgrade.install_version 键；清理历史备份与恢复分支


## Session 62: 修 README 过期版本号（0.0.18→0.0.21）+ 自检纳入文档版本声明
<!-- trellis-session: v=2 fp=e7e12a356b144cf2 -->

**Date**: 2026-09-26
**Task**: 修 README 过期版本号（0.0.18→0.0.21）+ 自检纳入文档版本声明
**Branch**: `main`

### Summary

用户问 README 里的「当前 0.0.18」是否最新——核查真源全是 0.0.21，README 是陈旧文案（三次升版漏同步）。根因是自检第 1 项只查 MAINTAINING、从不查 README。已修 README，并把两处文档版本声明都纳入自检，带判别性验证。

### Main Changes

- README：「（当前 0.0.18）」→「（当前 0.0.21）」
- check-repo 第 1 项：MAINTAINING + README 双查，任一与 version.env 不一致即报红

### Git Commits

| Hash | Message |
|------|---------|
| `dd653ce` | chore(task): archive 09-26-fix-readme-stale-version |

### Testing

- [OK] 判别性验证：README 改回 0.0.18 → 报红并指出不一致；还原 → 通过
- [OK] check-repo.sh --full 全绿

### Status

[OK] **Completed**

### Next Steps

- 可选：语言包里 upgrade.install_version 已无用键；清理历史备份与恢复分支
