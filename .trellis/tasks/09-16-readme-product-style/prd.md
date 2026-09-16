# README 改为产品视角短文，并发布服务器镜像

## Goal

把 README 从"维护者长文"改成 [new-api-own](https://github.com/zhemed/new-api-own) 那种产品视角短文
（定位 → 特性 → 部署 → 维护），并让它里面的 Docker 一节是**可复制的真命令**——因此要发布
服务器镜像 `ghcr.io/zhemed/komari`（amd64/arm64，`:<版本>` + `:latest`）。

## Requirements

1. README 结构对齐 new-api-own：一句话定位、特性 bullet、（本 fork 取舍）、部署（环境要求 / 一键 /
   方式一 Docker 镜像 / 方式二 源码构建 / 节点 agent）、构建与维护、维护与许可。
2. README 里出现过的维护者内容（CLI 子命令、数据与备份、构建矩阵、发布流程）不能丢，
   下沉到 `docs/MAINTAINING.md`（原有 §3.1–3.4，本次补 §3.5 CLI、§3.6 数据与备份、§12 镜像）。
3. 发布服务器镜像：多架构 amd64/arm64（只有这两个静态产物），`:0.0.4` 与 `:latest`。
4. 镜像必须在 README 里写的那样**匿名可拉**（公开包）；Dockerfile 补 OCI 标签链回本仓库。
5. 只做中文单份 README（不加 `README.en.md`）。

## Acceptance Criteria

- [ ] `README.md` 信息密度接近 new-api-own（60~130 行），所有相对链接可解析。
- [ ] README 中的命令实测可用：脚本安装、`docker run` 服务器、源码构建、agent 脚本安装与
      agent `docker run`。
- [ ] `ghcr.io/zhemed/komari:{0.0.4,latest}` 匿名可拉（`DOCKER_CONFIG=<空目录> docker manifest inspect` 成功）。
- [ ] 服务器镜像跑起来能到 `/install`，日志版本号与 `scripts/version.env` 一致，数据落在挂载卷。
- [ ] `docs/MAINTAINING.md` §3.5/§3.6/§12 落地；README 与规范里不再有重复或过期的构建说明。
- [ ] `go build ./... && go vet ./... && go test ./...` 全绿；提交并推送。

## Non-Goals

- 不改 `install-komari.sh` 的交互式菜单形态（不做 `curl | bash` 真一键，因为它是 TUI 菜单）。
- 不加英文 README、不加 CI、不动 0.0.4 已发布的 release 资产。
- 不把面板里指向上游文档站的链接改掉（需要改前端并重新发版，记为遗留）。

## Notes

- 已知限制：用户级 ghcr 包的可见性无法用 API 改（实测 `PATCH /user/packages/...` 一律 404），
  只能手点，需要用户配合。
