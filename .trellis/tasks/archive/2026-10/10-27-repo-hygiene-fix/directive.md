# 用户指令（directive）

## 用户当轮原话（2026-10-27）

> 授权第五项未做与风险，开始维护

## 上下文

上一轮「Komari 远程 SSH 移除」收尾报告「五、未做与风险」第 5 项：

> `komari-src` 的 `scripts/check-repo.sh` 仍余 2 项既有失败（非本任务引入）：
> `themes/ink/.env` 入库、`core.hooksPath` 指向全局 hooks + 上游快照历史 177 个提交无任务锚点。

用户授权处理该项，要求开始维护。

## 授权范围

- 修仓库卫生：`.env` 移出索引、安装本仓 git hooks。
- 治理 Trellis 历史审计：为外部导入历史设置 grandfather 基线（不重写历史、不关闸门）。
- 在 `komari-src` 提交维护变更（带任务锚点）、跑 `check-repo.sh` 验证。
- 不授权：产品功能改动、线上部署、GitHub 推送/发版、重写历史。
