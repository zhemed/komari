# agent 自有版本线：单仓库发布 agent 资产、默认关自更新、自建镜像

## Goal

把 komari-agent 纳入我们自己的发行线，但**不新增自有仓库**：agent 二进制作为
`zhemed/komari` release 的资产发布，安装脚本、镜像、前端安装命令全部指向我们。
终点是发 `0.0.4`（第一个同时含服务器 + agent 资产的 release），并在本机用我们的
安装脚本装一个 agent 连本地服务器做端到端验证。

## 背景（现状事实，已核实）

- 上游 agent `update/update.go:22-23` 是包级 `var CurrentVersion` / `var Repo`，
  `-X` 可覆盖，实测二进制里已是 `Repo=zhemed/komari`（源码零改动也能改目标）。
- agent **默认开自动更新**，每 6 小时查一次，命中就替换自己并 `os.Exit(42)`
  （靠 systemd `Restart=always` 拉起）。上游 `1.5.10` 是当前最新。
- 服务器**没有任何自更新代码**，也没有 agent 版本闸门（`web/agent/connections.go:44`
  只记协议版本）。
- 坑：自更新库用**后缀**匹配资产（`selfupdate/detect.go:69-75`），
  `komari-linux-amd64` 同样命中后缀 `linux-amd64`。若同一 release 混装两种资产，
  agent 会把自己刷成服务器二进制 → 节点报废。必须加资产过滤。

## Requirements

1. **单仓库**：不 fork agent，仍只有 `zhemed/komari` 一个自有仓库。
2. agent 源码 pin 到具体 commit（不改用分支/tag 漂移），补丁系列落仓库，可重放。
3. agent 自更新目标为本仓库，且**只能**命中 `komari-agent-*` 资产（防误刷服务器二进制）。
4. agent **默认关闭自动更新**；显式开启的方式要写进文档与安装脚本，并保留旧参数兼容。
5. `install-agent.sh` / `install-agent.ps1` 从我们仓库提供，参数与前端传参兼容，
   下载我们的资产。
6. 前端安装命令与 docker 两条命令改指我们（前端补丁系列新增 0006）。
7. 自建 `ghcr.io/zhemed/komari-agent`（含 `:latest`），随 release 推送。
8. 发 `0.0.4`：同一 tag 下同时含 `komari-linux-<arch>` 与 `komari-agent-<os>-<arch>`。
9. 本机用我们的安装脚本装一个 agent，注册到本地服务器，验证上报与版本显示。

## Acceptance Criteria

- [ ] `scripts/build-agent.sh` 在本地纯 Go 环境（`CGO_ENABLED=0`，无需 zig）产出 14 个
      `komari-agent-<os>-<arch>`，命名与上游一致。
- [ ] 构建产物中 `Repo=zhemed/komari`、`CurrentVersion=<我们的版本>`；不带
      `--enable-auto-update` 时**不发**更新请求（日志可证）。
- [ ] 资产过滤生效：用"两种资产 + 可辨认假二进制"的临时 release 实测，
      agent 自更新抓到的是 `komari-agent-*` 而不是 `komari-linux-amd64`（验完删除临时 release/tag）。
- [ ] `install-agent.sh` 在本机能装起 agent 并注册到 `http://127.0.0.1:25774`，
      节点列表可见且版本号显示为我们的版本。
- [ ] 前端补丁 0006 重放后，仓库内不再有 `komari-monitor/komari-agent` 与
      `ghcr.io/komari-monitor/komari-agent` 的安装命令文案，且 `FRONTEND_TREE_SHA256` 更新并复现一致。
- [ ] 镜像 `ghcr.io/zhemed/komari-agent:0.0.4` 可拉取运行并成功上报。
- [ ] `go build ./... && go vet ./... && go test ./...` 全绿；`docs/MAINTAINING.md`、
      `README.md`、`.trellis/spec/backend/build-and-pinning.md` 更新到与代码一致。

## Non-Goals

- 不 fork / 不镜像上游 agent 源码树到我们仓库（只 pin + 补丁）。
- 不改 agent 的监控/上报/远程控制行为（除自更新目标与默认值外不扩大改动）。
- 不新增 CI，发布仍是手动流程（沿用 `docs/MAINTAINING.md` §3.4）。
- 不做服务器侧的 agent 版本闸门（另立任务）。
- 不能改变已装在别处的上游 agent（它们仍指向上游），只能引导重装；这点写进文档。

## Notes

- 本地部署当前**只装了服务器、没有 agent**（`pgrep komari-agent` 无结果）。
- 上游 agent 的"远程控制（web ssh / 文件）默认开"是公开被滥用点；本次只做文档提示，
  不改默认值（用户明确不要削弱功能）。
