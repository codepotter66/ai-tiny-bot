# 05. 部署（国内 99 元/年 VPS 实测可用）

> 写于 2026-06-17
>
> 针对场景：阿里云轻量 / 腾讯云轻量 / 华为云 这类 99 元/年的小型 VPS，
> 特点是 docker daemon **没有默认配置任何镜像源**，直接 `docker pull` 会 i/o timeout。

## TL;DR

如果部署卡在 `failed to do request: Head "https://registry-1.docker.io/v2/...": dial tcp ...: i/o timeout`，按下面 3 步修：

```bash
# 1. 配 docker daemon 镜像源
sudo cp deploy/daemon.json.cn.example /etc/docker/daemon.json

# 2. 重启 docker
sudo systemctl restart docker

# 3. 重跑部署
cd "$DEPLOY_SERVER_DIR"
./update-tiny-bot-cloud-agent.sh
```

## 为什么会有这个问题

2024 年起 Docker Inc. 限流 + 国内镜像服务整改，所有公开匿名 Docker Hub 镜像基本都死了：

| 镜像源 | 状态 |
|---|---|
| `gcr.io/distroless/static` | ❌ gcr.io 国内 i/o timeout |
| `docker.io/library/golang` | ❌ docker.io 国内 i/o timeout |
| `registry.cn-hangzhou.aliyuncs.com/library/...` | ❌ Aliyun ACR 个人仓库要求登录 |
| `docker.m.daocloud.io/library/...` | ❌ DaoCloud 也认证化了 |
| `hub-mirror.c.163.com` / `docker.mirrors.ustc.edu.cn` / `mirror.baidubce.com` | ❌ DNS 不解析或服务下线 |

唯一靠谱的：**配置 docker daemon 的 registry-mirrors 指向能访问的镜像源**。

## 推荐配置（`deploy/daemon.json.cn.example`）

```json
{
  "registry-mirrors": [
    "https://docker.mirrors.aliyun.com",
    "https://mirror.baidubce.com",
    "https://docker.mirrors.ustc.edu.cn"
  ],
  "max-concurrent-downloads": 10,
  "log-driver": "json-file",
  "log-level": "warn"
}
```

按顺序尝试，前面的挂了自动 fallback。

### 验证镜像源是否生效

```bash
# 拉个小镜像测速
docker pull alpine:3.19

# 看是否走的是 mirror
docker info | grep -A5 "Registry Mirrors"
# 应该输出：
#  Registry Mirrors:
#   https://docker.mirrors.aliyun.com/
#   https://mirror.baidubce.com/
#   https://docker.mirrors.ustc.edu.cn/
```

## Dockerfile 设计

我们把 base image 用 **build-arg** 暴露，不硬编码镜像源：

```dockerfile
ARG GO_BASE=golang:1.22-alpine
ARG RUNTIME_BASE=alpine:3.19

FROM ${GO_BASE} AS build
...
FROM ${RUNTIME_BASE}
```

这样：
- **配了 daemon.json**：默认 base 名字直接走 mirror，0 配置
- **没配 daemon.json**：可以 `docker build --build-arg GO_BASE=<corp镜像>/library/golang:1.22-alpine ...`

## 完整部署流程

```bash
# 在服务器上（DEPLOY_SERVER_DIR 见本地 .env）
cd "$DEPLOY_SERVER_DIR"

# 0. 一次性：配 docker 镜像源
sudo cp tiny-bot-cloud-agent/deploy/daemon.json.cn.example /etc/docker/daemon.json
sudo systemctl restart docker

# 1. 上传新代码包
scp tiny-bot-cloud-agent.zip user@your.server.ip:"$DEPLOY_SERVER_DIR/"

# 2. 跑部署脚本
chmod +x update-tiny-bot-cloud-agent.sh
./update-tiny-bot-cloud-agent.sh

# 3. 验证
curl http://localhost:5678/healthz
# → {"ok":true,"uptime":N}

# 4. 运维
docker compose -f tiny-bot-cloud-agent/docker-compose.yml ps
docker compose -f tiny-bot-cloud-agent/docker-compose.yml logs -f
```

## 排错

### 问题：build 还是 timeout

```bash
# 看 daemon.json 是否真的生效
cat /etc/docker/daemon.json
docker info | grep -A5 "Registry Mirrors"

# 看 docker 日志
journalctl -u docker -n 50

# 手动测镜像源
docker pull alpine:3.19
```

### 问题：build 镜像源不全（多阶段 build）

buildkit 会为**每个 stage 单独拉 base image**。如果 `golang:1.22-alpine` 拉得到但 `alpine:3.19` 拉不到，说明 daemon.json 的 mirror 列表里某个挂了。换一个顺序。

### 问题：开发机本地 build 慢

如果在 Mac / Linux 开发机本地 build 也慢，同样改本地 `~/.docker/daemon.json`（macOS Docker Desktop 通过 UI 配）。

### 问题：用了 corp 私有 registry

如果有 corp 私有镜像源（带认证），用 build-arg：

```bash
docker compose -f tiny-bot-cloud-agent/docker-compose.yml build \
  --build-arg GO_BASE=registry.corp.internal/library/golang:1.22-alpine \
  --build-arg RUNTIME_BASE=registry.corp.internal/library/alpine:3.19
```

或者直接在 `docker-compose.yml` 里 `services.agent.build.args` 写死，省得每次传。

## 性能基准

| 阶段 | 镜像大小 | 国内首次拉取时间 |
|---|---|---|
| `golang:1.22-alpine` | ~250MB | 5-15s（走 mirror） |
| `alpine:3.19` | ~5MB | <1s |
| 我们的二进制 | ~15MB | 编译内置 |
| 最终镜像 | ~25MB | — |

build 总耗时（冷启动）：~1-2 分钟（包含两次镜像拉取 + Go 编译）。
