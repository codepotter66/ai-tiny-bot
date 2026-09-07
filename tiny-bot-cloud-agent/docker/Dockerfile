# ============================================================
# tiny-bot-cloud-agent - Dockerfile
# ============================================================
# 推荐部署方式：docker-compose（见 docker-compose.yml）
# 本 Dockerfile 仅作为单独构建用。镜像大小 ~25MB。
#
# 构建：
#   docker build -f docker/Dockerfile -t tiny-bot-cloud-agent:dev .
# 运行：
#   docker run --rm -p 5678:5678 \
#     -v $PWD/workspace:/opt/tiny-bot/workspace \
#     -v tiny-bot-data:/opt/tiny-bot/data \
#     --env-file .env \
#     tiny-bot-cloud-agent:dev
#
# ============================================================
# 国内环境说明
# ============================================================
# 公开的 Docker Hub 镜像大多要求认证了，所以默认 base 用最简洁的名字
# （让 docker daemon 解析到你配置的镜像源）。
#
# 配置方式二选一：
#
# 方式 A：配置 docker daemon 镜像源（推荐，一次配置永久生效）
#   sudo cp deploy/daemon.json.cn.example /etc/docker/daemon.json
#   sudo systemctl restart docker
#
# 方式 B：每次 build 时通过 build-arg 传镜像源
#   docker build \
#     --build-arg GO_BASE=mirror.baidubce.com/library/golang:1.25-alpine \
#     --build-arg RUNTIME_BASE=mirror.baidubce.com/library/alpine:3.19 \
#     -f docker/Dockerfile -t tiny-bot-cloud-agent:dev .
# ============================================================

# 全局 ARG：在 FROM 之前声明，跨多阶段共享
ARG GO_BASE=golang:1.25-alpine
ARG RUNTIME_BASE=alpine:3.19

# ---- build stage ----
FROM ${GO_BASE} AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags='-s -w' \
    -o /out/tiny-bot-cloud-agent \
    ./cmd/tiny-bot-cloud-agent
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags='-s -w' \
    -o /out/seed \
    ./cmd/seed

# ---- runtime stage ----
FROM ${RUNTIME_BASE}
RUN apk add --no-cache ca-certificates wget su-exec python3 && \
    adduser -D -u 1000 -s /sbin/nologin tinybot
WORKDIR /opt/tiny-bot
COPY --from=build /out/tiny-bot-cloud-agent /opt/tiny-bot/tiny-bot-cloud-agent
COPY --from=build /out/seed /opt/tiny-bot/seed
COPY workspace /opt/tiny-bot/workspace
COPY demo /opt/tiny-bot/demo
COPY deploy/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# 不在这里设 USER —— 让 entrypoint.sh 以 root 跑 chown，再用 su-exec 降权到 tinybot
EXPOSE 5678
ENTRYPOINT ["/entrypoint.sh"]
CMD ["/opt/tiny-bot/tiny-bot-cloud-agent"]
