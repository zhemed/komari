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
# **不要**给这两个 ARG 默认值：stage 内带默认值的 ARG 会**覆盖** buildx 按目标平台注入的值，
# 于是多架构构建的每一腿都 COPY 同一份 amd64 产物（2026-10-23 实测：arm64 腿里装的是 x86-64
# 二进制；`docker manifest inspect` 与匿名可拉**查不出**这类载荷错误，见 docs/MAINTAINING.md §12）。
# 现代 `docker build` 本身就是 BuildKit、会自动注入 TARGETOS/TARGETARCH；只有 `DOCKER_BUILDKIT=0`
# 的旧式构建才需要显式 `--build-arg TARGETOS=linux --build-arg TARGETARCH=<arch>`。
ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache ca-certificates curl tzdata

COPY --chmod=755 komari-${TARGETOS}-${TARGETARCH} /app/komari

ENV GIN_MODE=release
ENV KOMARI_LISTEN=0.0.0.0:25774

EXPOSE 25774

CMD ["/app/komari", "server"]
