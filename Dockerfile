FROM alpine:3.21

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
