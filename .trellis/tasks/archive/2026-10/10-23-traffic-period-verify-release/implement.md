# 执行清单

1. [ ] 无 tzdata 验证（需授权）：在**无 zoneinfo 的容器**里跑 `go test ./database/clients/ -run TestPeriodWindowTimezoneDatabaseAvailable`
       （或等价：临时改名宿主 zoneinfo —— 属系统改动，须单独授权；优先用容器方式）。
2. [ ] 跨边界观察（需授权）：本地实例 + 临时库，`traffic_reset_day` 设为"今天/明天"，观察 `net_period_up/down` 归零、`net_total_*` 不回退、auditlog 有重置记录。
3. [ ] 决定发版：若要发 → 版本号 +1、构建双架构 + agent 14 平台、18 资产、镜像 `:latest`、生产升级 + 验收 + 回滚点。
4. [ ] 收尾：finish → archive → journal。

## 回滚点
- 代码：`git revert 323f907`（新列被旧二进制忽略，**不删列**）；
- 生产：上一个 release 资产或 `komari.backup.<时间戳>`。
