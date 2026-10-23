# 部署

> 原则：**先备份再替换**；主题为静态产物（秒级、无需重启），服务端需要重建容器。

仓库内**不写任何站点专属值**（主机名/IP/域名），目标机一律用参数或环境变量传入。

## 一、主题部署（静态，无需重启）

```bash
./deploy/deploy-theme.sh \
  --zip ops-dist/theme/ink-build-<sha>.zip \
  --host <user@host>            # 或设置 KOMARI_HOST
# 预演（只打印计划，不触碰远端）：
./deploy/deploy-theme.sh --zip ... --host ... --dry-run
```

脚本行为：上传 → **远端 sha256 校验** → 备份现主题（`tar.gz`，含条目校验）→ 清 `dist/` 后解包 →
打印回滚命令。**不重启任何服务**；替换后刷新页面即生效（hashed 资源自动失效缓存）。

可选参数：`--theme-name`（默认 ink）、`--remote-theme-root`、`--remote-backup-dir`。

## 二、服务端部署（需要重建容器）

```bash
./build.sh --skip-theme                                  # 产出本地镜像
docker save komari-own:<版本>-own.1 | ssh <host> docker load
ssh <host> 'bash -s' < deploy/komari-v0.0.24-own.1-listen-local.sh
```

启动脚本约定了三件事：`--restart always`、`--network host`、`KOMARI_LISTEN=127.0.0.1:25774`（可用环境变量覆盖）。
> 收紧到 loopback 的原因：面板只经 nginx 反代暴露，避免端口直接对外。

## 三、部署后核对清单

1. `curl -s http://127.0.0.1:<port>/api/version` 版本正确；
2. 经 nginx 的站点 `/` 与 `/admin` 均 200（建议测两次）；
3. `ss -lntp | grep <port>` 只监听 `127.0.0.1`；
4. 容器 `Restarts=0`、`StartedAt` 与部署前一致（主题部署不应导致重启）；
5. 数据面：流量/DB 指标继续增长、累计不回退（只读方式查库）。
