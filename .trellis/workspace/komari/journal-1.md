# Journal - komari (Part 1)

> AI development session journal
> Started: 2026-09-16

---



## Session 1: 锁定 komari 1.4.3 为自维护分叉基线
<!-- trellis-session: v=2 fp=81e26ddfd4d7e731 -->

**Date**: 2026-09-16
**Task**: 锁定 komari 1.4.3 为自维护分叉基线
**Branch**: `komari-1.4.3`

### Summary

从上游 tag 1.4.3（bf6b45ec）建立自有主干 komari-1.4.3；vendor komari-web@4a74e8a8 前端产物使后端可离线构建（上游 tag 不锁前端版本，且 //go:embed defaultTheme 使后端单独无法构建）；新增 sync-frontend.sh（pin + 补丁 + npm ci + 目录树哈希门禁）与 build-komari.sh；补丁 0001 把后台更新检查改指 zhemed/komari，补丁 0002 让 vite 构建时间遵循 SOURCE_DATE_EPOCH 以修复产物不可复现（上游把构建时刻写入 Footer 所在 chunk，导致 chunk 哈希连锁漂移）；install-komari.sh 去上游化并修复 404 覆写运行中二进制的风险；推送至 github.com/zhemed/komari（公开）。AC1-AC7 全部验证：GOPROXY=off 离线构建、/install+assets 冒烟、两次再生成哈希一致、二进制内无上游 releases 地址。

### Git Commits

| Hash | Message |
|------|---------|
| `e70304f` | chore: 将仓库固定为只维护 komari 1.4.3 的自有基线 |
| `5942199` | docs: 记录 1.4.3 分叉的构建契约与推送陷阱 |

### Status

[OK] **Completed**
