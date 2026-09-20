# Komari

自用服务器监控：单个 Go 二进制 + 内嵌面板，节点装轻量 agent 上报。版本线 `0.0.x`（当前 0.0.18）。

## 部署

```bash
docker run -d --name komari --restart always \
  --network host \
  -v ./data:/app/data \
  ghcr.io/zhemed/komari:latest
```

数据在 `./data`；面板 `http://<主机>:25774`（首次进安装向导）。不用 host 网络就把
`--network host` 换成 `-p 25774:25774`。**升级**：面板「有新版本」点一下即可（容器内替换，零配置）。

## 节点 agent

```bash
wget -qO- https://raw.githubusercontent.com/zhemed/komari/main/install-agent.sh | sudo bash -s -- \
  -e <面板地址> -t <节点Token>

docker run -d --name komari-agent --restart always \
  ghcr.io/zhemed/komari-agent:latest -e <面板地址> -t <节点Token>
```

面板「节点 → 添加」会直接给出带密钥的现成命令。

## 自检

```bash
./scripts/check-repo.sh          # 发版前用 --full
```

其余（构建、发版、部署口径、回滚、维护规范）：[docs/MAINTAINING.md](./docs/MAINTAINING.md)
与 `.trellis/spec/`。
