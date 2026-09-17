# 执行计划：容器零配置网页升级

## Step 1 · 服务端
- [x] `internal/upgrade`：新增 `ModeContainerReplace`；`Prepare` 容器分支：
      有 socket → docker-recreate（现状）；无 socket 且目录可写 → container-replace；
      否则 manual
- [x] `Execute`：container-replace 复用二进制替换链路；`Result.InContainer=true`
- [x] `internal/upgrade/selfexec.go`：`SelfExec(path)` 封装 `syscall.Exec`
      （失败返回错误，由调用方回落 exit）
- [x] RPC：`runServerUpgrade` 在 `InContainer` 时先尝试 SelfExec，失败再 `upgradeExit(42)`
- [x] `CurrentMode` 同步（容器无 socket → container-replace）
- 验证：`go test ./internal/upgrade/... ./web/rpc/jsonrpc/...`

## Step 2 · 前端
- [x] 文案：container-replace → "立即升级（容器内替换）"；提示"重建容器会退回镜像版本，
      想与镜像一致请挂 docker.sock"
- [x] i18n 5 语言；产物重建 + 哈希更新
- 验证：`npx tsc --noEmit`；`./scripts/build-frontend.sh`

## Step 3 · 文档
- [x] README：容器升级两种路径（不挂 socket = 容器内替换；挂 socket = 重建容器）
- [x] MAINTAINING §14.4.2 / §14.6 同步；更正"必须挂 socket"的说法（那是"与镜像一致"的充分条件，不是升级的必要条件）
- 验证：`./scripts/check-repo.sh`

## Step 4 · 发版与实测
- [x] 版本线 → 0.0.13；提交 → 构建（服务端 2 平台 + 校验和 + agent 14）→ tag → release → 镜像
- [x] E2E：不挂 socket 的容器（用户的原命令形态）用面板接口升级到上一版本并升回；
      容器 ID 不变、二进制与发布资产一致、数据完整
- [x] 只读目录注入回归 → manual
- [x] 挂 socket 容器与 systemd 形态各回归一次

---

## 实测记录（2026-09-17，用户的原命令形态：**不挂 socket**）

| 场景 | 结果 |
|---|---|
| 模式探测 | ✅ `mode=container-replace`、`supported=true`（面板会显示"立即升级（容器内替换）"） |
| 升级 0.0.14 → 0.0.13 | ✅ 约 6 秒；**容器 ID 不变**、**重启次数 0 → 0**（真正的 execve 原地重执行，不依赖 restart 策略） |
| 容器内二进制 | ✅ 与 0.0.13 发布资产 **md5 一致**；备份 `/app/komari.backup.0.0.14` |
| 数据 | ✅ 累计上行单调不减；节点/指标完整 |
| 升回 0.0.14 | ✅ 约 8 秒，容器 ID 仍不变 |
| 只读无法替换 | ✅ 回落 manual（单测覆盖 `TestPrepareContainerReadOnlyFallsBackToManual`） |

## 0.0.13 的两个缺陷（本任务内发现并修于 0.0.14）

1. **面板不显示升级按钮**：`supported` 有两处内联表达式漏掉新模式 → 统一走 `upgradeSupports()`；
2. **原地重执行始终失败**：二进制替换后 `/proc/self/exe` 指向**备份文件**（inode 跟随），
   我写的"路径 == os.Executable()"校验永不成立 → 每次都回落成"退出靠 docker restart 兜住"。
   改为只校验目标文件存在且可执行后直接 exec。
   证据：修前日志有 `原地重执行失败（SelfExec 路径不一致…）`，修后 `RestartCount` 0→0。

0.0.13 的公开发布说明已追加更正段（原文保留）。
