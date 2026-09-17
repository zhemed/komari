# 执行计划：容器零配置网页升级

## Step 1 · 服务端
- [ ] `internal/upgrade`：新增 `ModeContainerReplace`；`Prepare` 容器分支：
      有 socket → docker-recreate（现状）；无 socket 且目录可写 → container-replace；
      否则 manual
- [ ] `Execute`：container-replace 复用二进制替换链路；`Result.InContainer=true`
- [ ] `internal/upgrade/selfexec.go`：`SelfExec(path)` 封装 `syscall.Exec`
      （失败返回错误，由调用方回落 exit）
- [ ] RPC：`runServerUpgrade` 在 `InContainer` 时先尝试 SelfExec，失败再 `upgradeExit(42)`
- [ ] `CurrentMode` 同步（容器无 socket → container-replace）
- 验证：`go test ./internal/upgrade/... ./web/rpc/jsonrpc/...`

## Step 2 · 前端
- [ ] 文案：container-replace → "立即升级（容器内替换）"；提示"重建容器会退回镜像版本，
      想与镜像一致请挂 docker.sock"
- [ ] i18n 5 语言；产物重建 + 哈希更新
- 验证：`npx tsc --noEmit`；`./scripts/build-frontend.sh`

## Step 3 · 文档
- [ ] README：容器升级两种路径（不挂 socket = 容器内替换；挂 socket = 重建容器）
- [ ] MAINTAINING §14.4.2 / §14.6 同步；更正"必须挂 socket"的说法（那是"与镜像一致"的充分条件，不是升级的必要条件）
- 验证：`./scripts/check-repo.sh`

## Step 4 · 发版与实测
- [ ] 版本线 → 0.0.13；提交 → 构建（服务端 2 平台 + 校验和 + agent 14）→ tag → release → 镜像
- [ ] E2E：不挂 socket 的容器（用户的原命令形态）用面板接口升级到上一版本并升回；
      容器 ID 不变、二进制与发布资产一致、数据完整
- [ ] 只读目录注入回归 → manual
- [ ] 挂 socket 容器与 systemd 形态各回归一次
