# 全面整理：一致性对齐 + 垃圾清零

用户 2026-09-19 指令：「现在全面整理我们的项目必须保证一致性，垃圾全部移除，我要看到效果」。

## 清点结果（只读，2026-09-19 05:1x）

工作区 1.2G，其中被 gitignore 的本地产物/缓存：

| 路径 | 大小 | 文件数 | 性质 | 处置 |
|---|---|---|---|---|
| `.build/` | 565M | 19596 | 本地构建缓存 | 拆开看（见下） |
| `.build/tools/` | 449M | — | zig 0.16.0 工具链（仅静态发版用，本仓库不装系统 zig） | 删（可重下，URL 已记录） |
| `.build/server-image/` `.build/agent-image/` | 102M | 6 | 镜像构建上下文，镜像内二进制可由 `scripts/build-*.sh` 重建 | 删 |
| `.build/check-agent/` | 13M | 2 | 门禁用临时 agent 二进制 | 删 |
| `.build/shots/` | 2.0M | 15 | 0.0.2–0.0.14 的手工验证截图（历史证据，非构建输入） | 删 |
| `.build/*.md` | ~60K | 17 | 历次发布说明 | **保留**（发版流程要用的唯一副本） |
| `.build/traffic-samples.csv` `.build/traffic_sampler.py` | 16K | 2 | 流量实测脚本/样本 | 待定（见下） |
| `.build/rel-server.sha256` | 4K | 1 | 发版校验中间产物 | 删 |
| `frontend/node_modules/` | 320M | 28627 | npm 依赖 | 删（`npm ci` 可还原，package-lock 已入库） |
| `frontend/dist/` | 6.5M | 428 | 前端构建输出 | 删（`scripts/build-frontend.sh` 可重建） |
| `frontend/bundle-analysis.html` | 1.3M | 1 | 一次性打包分析产物 | 删 |
| `dist/` | 236M | 19 | 0.0.18 发布资产副本（与已发布 release **sha256 完全相同**） | 删 |
| `bin/komari` | 26M | 1 | 陈旧二进制：内嵌 hash `2ec98e8`（0.0.18 弧线提交）、自称 0.0.18 | 删 |
| `utils/geoip/data/GeoLite2-Country.mmdb` | 8.2M | 1 | 手动下载的 GeoIP 库，源码无下载脚本 | 保留（重下麻烦） |
| `data/` | 132K | 2 | 本地开发数据库 | 保留（本地开发在用） |
| `.trellis/scripts/common/__pycache__/` | 228K | 19 | Python 字节码 | 删 |
| `.trellis/.runtime/` | 20K | 3 | 会话运行时状态 | 保留（运行时用） |

## 判定「垃圾」的四条硬标准（每条都要能举证）

1. **陈旧**：内嵌的提交/版本与当前仓库状态矛盾 → `bin/komari`
2. **可重建**：有脚本或已记录的下载路径可原样还原 → node_modules / frontend dist / `.build` 缓存
3. **已发布副本**：sha256 与线上 release 资产完全相同的本地副本 → `dist/`
4. **无来源**：字节码缓存等运行时副产物 → `__pycache__`

## 一致性核对（已做，结论：需要修的只有 1 处）

- 版本字面量：`scripts/version.env`、`install-komari.sh:REPO_TAG`、`install-agent.sh`、`install-agent.ps1` 全部 = `0.0.17` ✅
- 文档版本表：`docs/MAINTAINING.md:21` = 0.0.17 ✅
- `git diff 3698480..HEAD -- . ':(exclude).trellis'` = 0（回滚彻底）✅
- 追踪文件里对已回滚功能的引用（install-compose / compose.yaml / tag 自动同步）：**0 处** ✅
- 内网地址/主机名/凭据泄露扫描：`install*.sh`、`README.md`、`docs/`、`scripts/`、`.trellis/spec/` 全部为 0 ✅
- 大文件：最大追踪文件 740K（主题产物），无异常 ✅
- 文档里「缺失」的文件引用只有两条，且**正文已注明「该目录已从本仓库移除」** → 是刻意记录，不算悬空 ✅
- `./scripts/check-repo.sh` 全绿（含 Trellis 闸门、前端产物哈希）✅
- **唯一不一致**：`bin/komari` 自称 0.0.18 + 线上 `0.0.18` tag/release 仍存在，而分支已回到 0.0.17（发布物处置需用户点头，本轮只报告）

## Acceptance Criteria

- [x] 上述「删除」列全部移除，工作区体积显著下降（**1.2G → 43M**，不含 .git；文件数 50211 → 1541）
- [x] 保留项原样保留（发布说明、GeoLite2、本地 data、journal、发布标签）
- [x] 删除清单可查：`.build/purge-manifest-20260919.txt` 含体积、关键 sha256 与逐条还原命令
- [x] `./scripts/check-repo.sh --full` 12 项全绿；`go build`/`go vet`/`go test` 全绿
- [x] 生产 `/opt/komari/komari` 与线上 tag 不受影响（本轮没碰；生产二进制 sha256 `af56db79…` 与已发布 0.0.17 资产一致）

## 交付物

- 清理报告（给人看）：`docs/CLEANUP-2026-09-19.md`
- 清理清单（可复跑）：`.build/purge-manifest-20260919.txt`
- 修好的检查器：`scripts/check-repo.sh` 第 2 项（`git check-ignore` 豁免）+ 新增第 12 项
- 新增自检自测：`scripts/check-repo-selftest.sh`（7 条断言，3 条判别性）
- CI：`.github/workflows/trellis-gate.yml` 新增 `repo-consistency` 任务
- 文档：`docs/MAINTAINING.md` §2.1.1（自测的理由与断言表）、§3.5（本地产物与清理对照表）

## 意外发现（已修）

清理后自检报 4 条失败，根因是第 2 项把 `gitignore` 覆盖的目录当文档错误——
即"干净克隆必然假失败"。已修并加自测守住（详见报告 §三）。

## 未做（等用户决定）

线上 0.0.18 release/镜像的处置、`/opt/docker/komari` 生产路径清理、弧线前删除产物的重建。

