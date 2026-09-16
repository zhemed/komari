# Backend Development Guidelines

> Best practices for backend development in this project.

---

## Overview

This directory contains guidelines for backend development. Fill in each file with your project's specific conventions.

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Build & Pinning](./build-and-pinning.md) | 本 fork 的构建/vendor/pin 契约与验证命令 | **Filled** |
| [Directory Structure](./directory-structure.md) | Module organization and file layout | To fill |
| [Database Guidelines](./database-guidelines.md) | ORM patterns, queries, migrations | To fill |
| [Error Handling](./error-handling.md) | Error types, handling strategies | To fill |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns | To fill |
| [Logging Guidelines](./logging-guidelines.md) | Structured logging, log levels | To fill |

---

## Pre-Development Checklist

改动以下任一位置前，先读 [Build & Pinning](./build-and-pinning.md)：

- [`scripts/`](../../../scripts/) — 构建与前端再生成脚本
- [`web/public/`](../../../web/public/)（含 `web/public/.gitignore` 的 vendor 例外）
- [`install-komari.sh`](../../../install-komari.sh)
- `Dockerfile` / `.github/workflows/`

其它后端改动按常规流程，本仓库尚未填充的规范文件不要臆造内容。

---

## How to Fill These Guidelines

For each guideline file:

1. Document your project's **actual conventions** (not ideals)
2. Include **code examples** from your codebase
3. List **forbidden patterns** and why
4. Add **common mistakes** your team has made

The goal is to help AI assistants and new team members understand how YOUR project works.

---

**Language**: All documentation should be written in **English**.
