# 部署 Quickref

> Mac → 服务器一键部署，**整个流程不用登录服务器**。

[English](README.md)

## 最常用：发版

```bash
make deploy
```

→ 本地 build → save tar → scp 到服务器 → 远端触发部署（**默认 HTTPS 自签 443**）→ 健康检查

**workspace**：容器内可写（存记忆）；二次发版时**线上已有文件优先**，本地包只追加线上没有的路径（如新 skill），不会冲掉线上记忆/手改人设。

**在线编辑**：`https://<服务器IP>/demo/workspace.html`（需在 `.env` 配置 `TB_WORKSPACE_EDITOR_TOKEN`，页面填同一 token）。

部署完成后手机访问：**`https://<服务器IP>/demo/`**（首次信任证书；需安全组开放 443）

## 运维（不用登录服务器）

| 命令 | 干嘛 |
|---|---|
| `make deploy` | **发版**（build + scp + 远端触发 + 健康检查） |
| `make seed-remote` | **远端登记设备**（SSH 进 agent 容器跑 seed；可传 `DEVICE_ID` / `PAIRING_CODE`） |
| `make seed` | 本地 DB 登记设备（仅本机联调） |
| `make deploy-health` | 远端 /healthz（本地 curl，不登录） |
| `make deploy-status` | 远端 `docker compose ps`（容器状态） |
| `make deploy-logs` | 远端 `docker compose logs -f`（持续 tail） |
| `make deploy-restart` | 远端重建容器（重读 `.env`） |
| `make deploy-stop` | 远端停服 |

```bash
make seed-remote
make seed-remote DEVICE_ID=tinypal-esp32-01 PAIRING_CODE=ABCD-1234
```

登记后复位 ESP32；固件 `config.h` 的 `TB_DEVICE_ID` / `TB_PAIRING_CODE` 须与上述参数一致。

## 配置文件

`.env` 中必须配置部署目标（仓库内无硬编码默认）：

```bash
DEPLOY_SERVER=user@your.server.ip           # 必填：SSH 目标
DEPLOY_SERVER_DIR=/path/to/deploy/root      # 必填：远端部署根目录
TB_PUBLIC_IP=your.server.ip                 # 自签 HTTPS profile 时必填
```

## 完整流程

```
Mac: docker save 镜像 → tar
    ↓ scp
    ↓ scp update-tiny-bot-cloud-agent.sh（每次覆盖）
    ↓ ssh
服务器: update-tiny-bot-cloud-agent.sh --from-tar
      → 解 files tar（workspace：线上优先 + 仅追加缺失）
      → docker load
      → docker compose down
      → docker compose up -d
      → sleep 20
      → curl /healthz
      → 退出码
    ↓ exit 0/1
Mac: 把退出码透传，>0 打印远端日志
```

## 首次部署前置（一次性）

1. 服务器上 `daemon.json` 已配镜像源（国内小 VPS 常用 `deploy/daemon.json.cn.example`）
2. SSH 密钥已配：`ssh-copy-id user@your.server.ip`（与 `.env` 的 `DEPLOY_SERVER` 一致）
3. 本地 `.env` 已建并填好密钥与 `DEPLOY_SERVER` / `DEPLOY_SERVER_DIR` / `TB_PUBLIC_IP`

## 出问题

| 症状 | 第一步 |
|---|---|
| `make deploy` 卡在 passphrase | `ssh-copy-id user@your.server.ip` |
| 健康检查失败 | `make deploy-logs` 看远端日志 |
| 服务器完全不可达 | `make deploy-status` 看容器 |
| 手机 demo 麦克风报错 | 不能用 `http://IP`，见下方「手机 Demo 需要 HTTPS」 |
| 想强制清掉重来 | `make deploy-stop` 后再 `make deploy` |

## 手机 Demo 需要 HTTPS（无域名也行）

浏览器 `getUserMedia` 仅在 **HTTPS** 或 **localhost** 可用。公网 `http://IP:5678/demo/` 在手机上会禁用麦克风。

### 方案 A：Cloudflare 临时隧道

```bash
cd "$DEPLOY_SERVER_DIR/tiny-bot-cloud-agent"
docker compose --profile tunnel up -d
docker compose logs cloudflared
```

日志里会出现 `https://xxxx.trycloudflare.com`，手机打开 **`https://xxxx.trycloudflare.com/demo/`** 即可。临时 URL 重启后会变。

### 方案 B：自签 HTTPS（固定 `https://IP/demo/`）

1. 云安全组开放 **443**
2. `docker compose --profile https-selfsigned up -d`
3. 手机访问 `https://<服务器IP>/demo/`，首次信任证书

### 方案 C：有域名时

`.env` 设 `TB_PUBLIC_DOMAIN`，`docker compose --profile https up -d`。

详见 [`demo/README.md`](../demo/README.md) / [`demo/README.zh-CN.md`](../demo/README.zh-CN.md)。

## 详细文档

[../docs/human-docs/05-deployment.md](../docs/human-docs/05-deployment.md) — 详细部署指南（含 daemon.json、网络排查、Dockerfile 原理）
