# 执行计划

## Step 1 — agent vendor
1. 从 `.build/agent-src`（pin+补丁后的工作区）复制跟踪文件到 `agent/`，排除 `.git`、`.github/`、
   `install.sh`、`install.ps1`。
2. 重写 `scripts/build-agent.sh`：去掉 clone/checkout/apply/源码树哈希/安装脚本回放比对，
   保留 14 平台矩阵、`-trimpath -buildvcs=false`、ldflags 注入、过滤与自更新目标断言、版本一致性断言、SHA256SUMS。
3. 删 `scripts/patches-agent/`，`scripts/agent-pin.env` 改成溯源说明（保留 `KOMARI_AGENT_UPDATE_REPO`、
   上游 commit 注释、`KOMARI_AGENT_GO_VERSION`）。

## Step 2 — agent 等价性验证
```bash
# 老路径（去掉 buildvcs 以与新路径可比）
cd .build/agent-src && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false \
  -ldflags "-X github.com/komari-monitor/komari-agent/update.CurrentVersion=0.0.5 \
            -X github.com/komari-monitor/komari-agent/update.Repo=zhemed/komari" -o /tmp/agent-old .
# 新路径
./scripts/build-agent.sh --only linux/amd64
sha256sum /tmp/agent-old dist/agent/komari-agent-linux-amd64
```
期望：两个哈希相同。

## Step 3 — 前端 vendor
1. 从 `.build/komari-web`（pin+补丁后的工作区）复制跟踪文件到 `frontend/`，排除 `.github/`。
2. 新写 `scripts/build-frontend.sh`（取代 `sync-frontend.sh`）：本地 npm ci + build + 注入 + 两处校验；
   删除 `scripts/sync-frontend.sh` 与 `scripts/patches/`。
3. `.gitignore` 补前端构建产物与 node_modules 规则。

## Step 4 — 前端等价性验证
```bash
./scripts/build-frontend.sh          # 期望哈希仍为 af0bd793… 并通过校验
grep -rn "git clone\|git fetch" scripts/   # 期望只剩注释
GOPROXY=off GOFLAGS=-mod=mod ./scripts/build-komari.sh
```

## Step 5 — 文档与收尾
- `docs/MAINTAINING.md`：§1 固定点表（源码来源改为"已 vendor"）、§3.2 前端构建、
  §11 agent（去 pin/补丁叙述，改为"源码在 agent/"，保留历史教训 §11.5/§11.6）、§4 解耦点表。
- `.trellis/spec/backend/build-and-pinning.md`：§1.2/§1.3/§1.6 全部按新事实改写。
- `README.md`：取舍里补"前后端与 agent 源码都在本仓库内，构建不克隆上游"。
- 提交、推送；不新发版本（产物未变）。
