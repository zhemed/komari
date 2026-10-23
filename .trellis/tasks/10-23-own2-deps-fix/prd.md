# 修复轮：内核依赖升级 + 主题构建链升级 + 部署 own.2

## 需求
1. 内核：`go mod edit -toolchain=go1.26.10`；`go get golang.org/x/net@v0.60.0 golang.org/x/text@v0.41.0` + `go mod tidy`；`govulncheck ./...` 期望清零；`go mod verify` 通过；`./build.sh` 出二进制与镜像 `komari-own:0.0.24-own.2`；
2. 主题：升级 7 个构建链包 → `bun audit`（官方源）归零、`bun run lint` 通过、`./build.sh` 出 zip（版本仍 0.0.1）；**不部署主题**；
3. 部署内核：备份 DB 与容器 inspect 快照 → 新建 `deploy/komari-v0.0.24-own.2-listen-local.sh`（逐字保留 KOMARI_LISTEN/--restart always/--network host/-v/GIN_MODE/--workdir）→ `docker save|load` → 重建容器 → 验证；
4. 打标签 `server/v0.0.24-own.2` 并推送。

## 验收标准
- `govulncheck ./...` 退出码 0（或残留项逐条说明）；`bun audit` 0 vulnerabilities；
- 部署后：`/api/version` 正常、nginx 下 `/` 与 `/admin` 双次 200、`127.0.0.1:25774`、容器 Up/Restarts=0、DB 周期增长且累计不回退、三处「本周期流量」文案与主题入口 `index-08OsQRET.js` 不变；
- 失败可回滚（0.0.24 脚本 + 镜像仍在）。

## 非目标
- 不改业务代码逻辑；不动 DB/nginx/证书/其它容器；主题不上线。
