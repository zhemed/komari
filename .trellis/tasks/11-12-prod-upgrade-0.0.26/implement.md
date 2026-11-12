# 实施记录：生产机升级 0.0.26 + 主题 1.0.5

## 生产环境

- 主机：`zzds.de`（19.9刀 VPS，KVM，Ubuntu 22.04），经 DSH SSH 资源 `19.9刀-VPS` 操作
- 服务：Docker 容器 `komari`（host 网络，`127.0.0.1:25774`，数据 `/root/komari/data`），nginx 反代 `vps.miio.cc`
- 操作前：`0.0.25` / hash `942bb4a`，镜像 `ghcr.io/zhemed/komari:0.0.25`（digest `7ef9f4ca…`）

## 执行（2026-11-12 10:41–10:45 UTC）

| 步骤 | 结果 |
|---|---|
| 只读核对 | 服务全 active；容器 up 6 天；磁盘 12G/49G；最近备份 `komari-20261111-201701` |
| 升级前备份 | `/root/backups/preupgrade-komari-0.0.26-20261112T104138Z/`：`komari.db`（sqlite3 在线备份）、`theme-ink-1.0.4.tgz`、容器/镜像 inspect、`SHA256SUMS`（全部校验 OK） |
| 拉镜像 | `ghcr.io/zhemed/komari:0.0.26` digest `d7768daa…`（linux/amd64） |
| 容器切换 | 旧容器改名 `komari-rollback-0.0.25` → 停止 → 同配置重建 `komari`（`--network host -v /root/komari/data:/app/data -e GIN_MODE=release -e KOMARI_LISTEN=127.0.0.1:25774 --restart always --workdir /app`），带失败自动回滚兜底 |
| 程序自动备份 | `data/backup/upgrade-20261112-104204.zip`（9.9MB，0.0.25→0.0.26） |
| 主题更新 | 下载 theme-latest `ink-theme.zip`（sha256 `439cb768…` 校验通过，包内 1.0.5/761 文件）→ 原子替换 `data/theme/ink`，旧目录移入备份目录 |
| 异地副本 | 备份目录下载到 `/root/19.9刀/backups/preupgrade-komari-0.0.26-20261112T104138Z/`（gitignore，SHA256SUMS 复核 OK） |

## 验证

| 项 | 结果 |
|---|---|
| 版本 | `/api/version` = `0.0.26` / hash `41c3a22f…`（与 release tag 一致） |
| 站点 | `https://vps.miio.cc/`、`/admin`、`/api/version` 均 200 |
| 节点 | `RAKsmart` 在线；agent 日志：WebSocket 1006/502 → `attempt: 1` → `connected using v2 protocol` |
| 数据 | 负载记录 61 点/小时、ping 记录 183 点（task_id 1），连续无缺口 |
| 日志 | `docker logs --since 5m | grep -ciE 'error|panic|fatal'` = 0 |
| 主题 | manifest `1.0.5`；首页 `assets/index-ou270oNa.js`；三网行双条版式（截图确认，无「丢包」前缀） |
| 容器 | `restart=always`，镜像 `:0.0.26`；旧容器 `komari-rollback-0.0.25`（Exited 0）保留 |
| 市场接口 | `/api/admin/theme/market/catalog` 返回 SPA 兜底（路由已删） |
| 未动项 | nginx / sb / new-api / picsur / 防火墙 / sshd / 证书 均未变更 |

## 回滚（已写入运行手册变更记录）

```bash
# 容器
docker rm -f komari && docker rename komari-rollback-0.0.25 komari && docker start komari
# 主题
rm -rf /root/komari/data/theme/ink && mkdir -p /root/komari/data/theme/ink
tar xzf /root/backups/preupgrade-komari-0.0.26-20261112T104138Z/theme-ink-1.0.4.tgz -C /root/komari/data/theme
```

## 遗留

- 旧容器 `komari-rollback-0.0.25` 与旧镜像（0.0.23/0.0.24/0.0.25 等）占磁盘，清理需用户确认。
- 生产主题 1.0.5 与 0.0.26 内置主题版本一致，后续可走面板「主题→更新」。
