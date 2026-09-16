# agent 版本漂到 1.5：退回 1.4.3 同期并发 0.0.5

## Goal

用户在本机登录时看到 `/etc/motd` 里出现
`[Komari] Remote control is enabled on this device ... 127.0.0.1:25774 can execute commands ...`
怀疑我们拉到了上游 1.5。查证并修正版本漂移，让三条线（服务器/前端/agent）回到同一 1.4.3 血统。

## 查证结论

- 那条告警**不是入侵痕迹**：是 agent 自己写的（`cmd/warn_linux.go` 往 `/etc/motd` 注入），
  触发条件是"远程控制开着"（上游默认开），说的是 WebSSH/远程执行本身的能力。
- **版本确实漂了**：0.0.4 的 agent pin 是上游 agent 的 tag `1.5.10`（2026-09-15），
  比服务器/前端的 1.4.3（2026-08-13）晚一个月；motd 注入是 `79d8d45 增强安全提醒`（2026-09-14）新加的。

## Requirements

1. agent pin 退回 `1186aafb`（2026-08-07，1.4.3 之前最后一个 agent 提交）。
2. 安装脚本补丁按旧版重做，`install-agent.sh`/`.ps1` 重新 vendor（默认目录 /opt/komari-agent、
   默认装 pin 版本、无 snapshot 通道、修 Git Bash 下 Windows 资产缺 .exe）。
3. 发 `0.0.5`：服务器 + 14 个 agent 资产 + 镜像，tag == 资产 == 镜像源码。
4. 加构建期守卫：pin 日期不得晚于服务器基线日期（防同类错误再犯）。
5. 本机清掉 `/etc/motd` 告警并重装 agent，验证不再被写回、身份不重复注册。

## Acceptance Criteria

- [x] 新 agent 二进制里没有 motd 注入代码（`strings | grep -c "Remote control is enabled"` = 0）
- [x] `build-agent.sh` 三道门禁通过（pin 日期 / 安装脚本逐字节 / 资产过滤与自更新目标）；
      反例（上限调到 pin 之前）以退出码 1 拦下
- [x] 0.0.5 发布，17 个资产；镜像 `:0.0.5` 与 `:latest` 已推
- [x] 本机服务器升到 0.0.5（二进制 sha256 与 release 资产一致），agent 重装为 0.0.5，
      `/etc/motd` 干净、无 401、身份复用同一 uuid、面板节点版本显示 0.0.5
- [x] 文档：MAINTAINING §11.5（为什么不跟 1.5）、§11.6（那条告警到底是什么）、
      §7（升级后 WAL 偶发 unlink 现象）；README 补"agent 也停在 1.4.3 同期"

## Notes

- 退回的代价（已核对）：丢 3 个检测修复（AMD GPU sysfs / Android FUSE / macOS nullfs）与
  安装脚本两处改进（无 bash 环境、snapshot 通道）；文件访问、终端重连、文件上传那批
  我们这条血统根本调不到，不构成损失，且 root 二进制更小。
- 顺带修正 README 里错写的"文件管理"（我们这条血统没有该功能：服务器无文件 RPC、前端无该页面）。
