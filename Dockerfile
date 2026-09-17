# 基础镜像钉到 digest（2026-09-17，仓库体检）：tag 会随上游滚动，同一份源码在不同时间
# 构建出的镜像不可复现。升基础镜像时改这里并重跑 scripts/build-server-image.sh 验证；
# 代价是上游 3.21.x 的安全更新不再自动进来（见 docs/MAINTAINING.md §7）。
FROM alpine:3.21@sha256:48b0309ca019d89d40f670aa1bc06e426dc0931948452e8491e3d65087abc07d

# OCI 标签：ghcr 包页面会链回本仓库，也便于溯源（见 docs/MAINTAINING.md §12）
LABEL org.opencontainers.image.source="https://github.com/zhemed/komari" \
      org.opencontainers.image.description="Komari Monitor 服务器（自维护 0.0.x 版本线）" \
      org.opencontainers.image.licenses="MIT"

WORKDIR /app

# 需要**静态链接**的二进制（见 docs/MAINTAINING.md：KOMARI_STATIC=1 构建）。
# 默认值让普通 `docker build` 也能工作，不必依赖 buildx 注入平台参数。
ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN apk add --no-cache ca-certificates curl tzdata

COPY --chmod=755 komari-${TARGETOS}-${TARGETARCH} /app/komari

ENV GIN_MODE=release
ENV KOMARI_LISTEN=0.0.0.0:25774

EXPOSE 25774

CMD ["/app/komari", "server"]
