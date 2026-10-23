# 回滚

## 主题回滚（秒级、无停机）

deploy 脚本会在部署时打印具体的回滚命令，形如：

```bash
ssh <host> "rm -rf /root/komari/data/theme/ink \
  && tar xzf /root/backups/theme-ink-<ver>_<时间戳>.tgz -C /root/komari/data/theme"
```

刷新页面即回到旧版（静态资源）。

## 服务端回滚

1. 用上一版启动脚本重建容器（镜像仍在本地）：`deploy/komari-v<上一版>-listen-local.sh`；
2. 若数据异常：停容器 → 备份现 `data/komari.db` → 用每日备份/手工备份恢复 → 再启动；
3. 回滚后同样执行 DEPLOY.md 的核对清单。

## 备份策略

- 主题：每次部署自动备份到远端 `/root/backups/`（保留人工清理，不在自动轮转内）；
- 数据：每日备份（DB 7 份 / picsur dump 3 份等，见运维侧说明）；
- 建议：主题/服务端各保留最近 2~3 个可回滚版本。
