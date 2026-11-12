# 生产机升级到 0.0.26（+ 主题 1.0.5）

## Goal

用户 2026-11-12 明确授权本次生产操作：把生产 VPS（zzds.de，19.9刀）上的 Komari
从 `0.0.25` 升级到刚发布的 `0.0.26`；用户在同轮确认「顺手把生产主题也更新到 1.0.5」。

## 范围与约束

- 只动 `komari` 容器 + `/root/komari/data`；nginx / sb / new-api / picsur / 防火墙 / sshd 均不动。
- 生产凭据/私钥不进对话与仓库；全部操作经 DSH 已保存 SSH 资源 `19.9刀-VPS`（root@zzds.de）。
- 保留回滚素材（旧容器、旧镜像、升级前备份）。
- 遵循 `/root/19.9刀/.trellis/spec/ops/` 的 change-process / safety / backups 规范。

## 步骤

1. [x] 只读核对（服务/容器/版本/监听/磁盘/备份日志）
2. [x] 备份：`sqlite3 .backup` 在线备份 DB + 主题 tgz + 容器/镜像 inspect + `SHA256SUMS`
3. [x] 拉取 `ghcr.io/zhemed/komari:0.0.26`（digest `d7768daa…`）
4. [x] 容器切换：旧容器改名 `komari-rollback-0.0.25` 并停止 → 同配置重建（失败自动回滚兜底）
5. [x] 验证：版本/站点/节点/记录/日志/市场接口
6. [x] 主题 1.0.4 → 1.0.5（theme-latest 资产 sha256 校验 + 原子替换）
7. [x] 异地副本 + 运行手册/任务/journal 留痕

## 验收标准

- [x] `/api/version` = `0.0.26` / hash `41c3a22`（与 release tag 一致）
- [x] `https://vps.miio.cc/` 200；节点在线、负载/ping 数据连续
- [x] 主题 manifest 1.0.5，首页三网行双条版式（截图）
- [x] 容器 `restart=always`；日志无 error/panic/fatal
- [x] 回滚素材齐全（旧容器/旧镜像/升级前备份 + 回滚命令写入运行手册）

## 非目标

- 不清理旧镜像/旧容器（安全红线：需用户确认）。
- 不改 nginx/证书/其他服务；不升级 agent（生产节点 agent 自更新默认关闭）。
