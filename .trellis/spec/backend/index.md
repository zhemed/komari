# Backend Development Guidelines

> 本仓库后端的**实际**开发约定。规范描述的是代码现状，不是理想状态。

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Build & Pinning](./build-and-pinning.md) | 构建/vendor/pin 契约与验证命令 | **Filled** |
| [Directory Structure](./directory-structure.md) | 目录分层、放置与命名约定 | **Filled** |
| [Database Guidelines](./database-guidelines.md) | GORM 模型、AutoMigrate、配置存储、SQLite 调优 | **Filled** |
| [Error Handling](./error-handling.md) | RPC/REST 错误返回、panic/recover 边界 | **Filled** |
| [Logging Guidelines](./logging-guidelines.md) | `utils/log` API、模块名约定、级别使用 | **Filled** |
| [Quality Guidelines](./quality-guidelines.md) | 测试写法、提交前必跑命令、已知遗留缺陷 | **Filled** |

---

## Pre-Development Checklist

改动以下任一位置前，先读 [Build & Pinning](./build-and-pinning.md)：

- [`scripts/`](../../../scripts/) — 构建与前端再生成脚本
- [`web/public/`](../../../web/public/)（含 `web/public/.gitignore` 的 vendor 例外）
- [`install-komari.sh`](../../../install-komari.sh)
- `Dockerfile`

其它后端改动，按对应规范文件执行；`.trellis/spec/guides/` 下是跨包思考指引。

---

## 维护这些规范

1. 只写**代码实际怎么做**（带 `file:line` 锚点），不写理想状态；锚点会随代码漂移，改动相关代码时顺手更新。
2. 发现规范与代码不一致时，以代码为准并修正规范——除非该不一致本身就是已记录的遗留缺陷。
3. 新学到的可复用结论（尤其是踩过的坑）写进对应文件，而不是只留在对话里。

---

**语言**：本目录规范使用中文，与 `build-and-pinning.md` 保持一致。
