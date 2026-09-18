# Compose 部署定稿入仓（升级走 B 方案）

## Goal

用户拍板两件事：① compose 部署的升级策略选 **B**（挂 `/var/run/docker.sock`，面板拉镜像 + helper
重建容器，版本与镜像始终一致）；② 把定稿 compose（含日志上限）**写进仓库**。

## Requirements

- **R1** 写文档前先实测"重建容器后 compose 是否仍认得该容器"与"改文件会不会把版本拉回"，
  结论必须带实测证据。
- **R2** README 的 Docker 段改成 compose 优先：钉 tag、host 网络、卷、socket（B 策略）、
  日志上限、curl healthcheck，并保留不用 compose 的最小命令。
- **R3** MAINTAINING 新增"部署约定（Compose）"章节：目录约定、逐项取值依据、B 策略、
  compose 交互三条实测、操作规程。
- **R4** 只写文档，不改代码；`check-repo.sh` 全绿。
- **R5** 提交带 `[task:compose-mode-b-docs]`，流程走完并归档。

## Acceptance Criteria

- [x] 实测：`docker compose ps` 在面板式重建后仍识别容器；改文件才回退；回滚点被 compose 清掉
- [ ] README 含可直接复制的 compose（与实测配置一致）
- [ ] MAINTAINING §15 含三条实测行为与操作规程
- [ ] `./scripts/check-repo.sh` 通过
- [ ] journal + 归档

## Out of Scope

- 不在用户宿主机实际部署（`/opt/docker/komari` 需另行授权）；
- 不改 compose 之外的部署形态文档（不挂 socket 的形态已在 §14 记录）。
