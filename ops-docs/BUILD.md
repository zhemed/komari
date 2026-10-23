# 构建

## 前置

- Go（内核构建；`go version` ≥ go.mod 要求）
- Bun（主题构建，`bun --version` ≥ 1.2）
- Docker（构建本地服务端镜像）
- 首次构建主题需要网络（`bun install` 拉 npm 依赖）

## 一键构建

```bash
./build.sh                 # 服务端二进制+镜像 + 主题 zip
./build.sh --skip-install  # 跳过 bun install（依赖已装）
./build.sh --skip-server   # 只构建主题
./build.sh --skip-theme    # 只构建服务端
```

## 产物

| 产物 | 路径 | 说明 |
|---|---|---|
| 服务端二进制 | `bin/komari` | 动态链接（本机形态） |
| 服务端镜像 | `komari-own:<上游版本>-<own后缀>` | 基于 `deploy/Dockerfile.local`（debian-slim） |
| 主题包 | `ops-dist/theme/ink-build-<git短sha>.zip` | 含 `komari-theme.json` + `preview.png` + `dist/` |
| 主题包校验 | `ops-dist/theme/ink-build-<sha>.zip.sha256` | 部署脚本会校验它 |
| 构建摘要 | `ops-dist/BUILD-INFO.txt` | 版本、git sha、产物 sha256 |

## 关于可复现性（重要）

- **主题构建不可复现**：产物含构建时间，chunk 文件名/内容 hash 每次构建都会变。
  ⇒ **部署时必须使用同一份 zip**（配套 `.sha256`），不要"重新构建后混用"。
- 服务端二进制含 git 版本信息；同 commit 重复构建一般字节一致，但不做保证。

## 与官方镜像形态的差异

官方发版镜像基于 alpine（要求 zig 静态链接）。本仓库默认走 **glibc 基础镜像**（`deploy/Dockerfile.local`），
体积略大、行为一致。若需要与官方一致的静态形态：

```bash
cd server && KOMARI_STATIC=1 ./scripts/build-komari.sh   # 需 zig（未装则不静默降级，会明确报错）
```
