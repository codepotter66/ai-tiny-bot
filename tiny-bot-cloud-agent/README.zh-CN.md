# tiny-bot-cloud-agent

[ai-tiny-bot](../) 陪伴机器人的云端 Agent 服务（Go）。

[English](README.md)

- WebSocket 端点 `ws://<host>:5678/ws`，与 [`../tiny-bot-firmware/`](../tiny-bot-firmware/) ESP32 固件对接
- 双向原始 PCM 16 kHz / 16 bit / 单声道（固件 I2S 直放，无需解码器）
- 支持：人设（SOUL/IDENTITY/AGENT/USER）、记忆（Markdown + 关键词/时间衰减）、技能（SKILL.md + scripts）
- 目标部署：2 vCPU / 2 GB RAM，**docker-compose 优先**（裸机 systemd 备选）
- 架构说明：[docs/human-docs/00-architecture.md](docs/human-docs/00-architecture.md)；`internal/` 包手册：[docs/human-docs/09-internal-packages.md](docs/human-docs/09-internal-packages.md)

## 快速开始

```bash
# 1. 构建
make build

# 2. 准备数据库 + 一个预置设备
#    seed = 在云端登记 device_id/pairing_code（入场许可），不是让板子立刻在线；
#    板子上电后还会走 /provision 拿 token。未 seed → provision 401 device not found。
#    本地联调：make seed
#    线上服务器（本机 SSH）：make seed-remote
make seed

# 3. 启动服务
make run &
sleep 1

# 4. 健康检查
curl http://localhost:5678/healthz
# {"ok":true,"uptime":1}

# 5. 模拟 ESP32 跑一轮
TB_FAKE_ASR_TEXT="今天天气怎么样" ./bin/fake-device -code ABCD-1234 -device-id tinypal-01
```

真实 ASR/LLM/TTS 前请 `cp .env.example .env` 并填密钥；**勿提交** `.env`。

## 协议概览

WebSocket 文本帧，JSON。详细定义见 [internal/ws/protocol.go](internal/ws/protocol.go)。

### 客户端 → 服务端

| type | 必填字段 | 含义 |
|------|----------|------|
| `hello` | `device_id`, `token` | 首次连接，鉴权 |
| `audio` | `data` (base64 PCM) | 上行音频 chunk |
| `end` | — | 一句话结束 |
| `ping` | — | keepalive |
| `interrupt` | — | 打断当前播放 |

### 服务端 → 客户端

| type | 额外字段 | 含义 |
|------|----------|------|
| `hello.ok` | `sample_rate` | 鉴权通过 |
| `stt` | `text` | ASR 结果 |
| `text` | `text` | LLM 中间 token |
| `audio` | `data` (base64 PCM), `seq` | 下行 TTS chunk |
| `tool` | `tool`, `args` | 工具调用事件 |
| `done` | — | turn 结束 |
| `error` | `error` (`CODE:message`) | 错误 |

## HTTP 端点

| 路径 | 方法 | 用途 |
|------|------|------|
| `/healthz` | GET | 存活（始终 200） |
| `/readyz` | GET | 就绪 |
| `/metrics` | GET | 简单 JSON metrics |
| `/provision` | POST | 首次配网（`?device_id=xxx&code=yyy` → token） |
| `/ws` | GET | WebSocket |
| `/demo/` | GET | 浏览器 demo（若开启） |

## 目录约定

```
tiny-bot-cloud-agent/
├── cmd/                # 入口（主服务 + seed + echo-agent + asr-test）
├── tools/fake-device/  # 假 ESP32 客户端（开发自检）
├── internal/           # 业务包
│   ├── config/  logging/  server/  ws/  auth/  devices/
│   ├── audio/  asr/  tts/  llm/  agent/  skills/  memory/  persona/  store/
├── workspace/          # 运行时数据（人设/记忆/技能）
│   ├── SOUL.md  IDENTITY.md  AGENT.md  USER.md
│   ├── memory/<device_id>/YYYY-MM-DD.md
│   └── skills/<name>/SKILL.md + scripts/
├── deploy/             # Mac → 服务器部署脚本
├── docker/             # Dockerfile
├── docker-compose.yml
└── test/               # 集成测试
```

## 开发

```bash
make build
make run
make test
make test-race
make asr-test       # 直连阿里云 ASR（读 .env + testdata PCM）
make llm-test       # 直连 MiniMax LLM
make fake-device
make lint
make tidy
```

### 阿里云 ASR 调试

在 `.env` 中配置 `TB_ALIYUN_KEY` / `TB_ALIYUN_SECRET` / `TB_ALIYUN_ASR_APP_KEY` 后：

```bash
make asr-test
# 或指定 PCM：go run ./cmd/asr-test -file path/to/audio.pcm
```

期望：CreateToken 成功 → ASR 返回识别文本。全链路联调仍用 `make run` + `fake-device`。

### 添加技能

在 `workspace/skills/<name>/SKILL.md`：

```markdown
---
name: weather
description: 查询某城市天气
parameters:
  city:
    type: string
    description: 城市名
    required: true
script: scripts/run.sh
---

# weather skill body
```

`scripts/run.sh` 读 stdin JSON，输出到 stdout。重启服务或发送 SIGHUP 后该 skill 会被 LLM 当作 tool 暴露。

## 部署

### 方式 A：docker-compose（推荐）

```bash
cp .env.example .env
vi .env                       # 填 API key；若用 make deploy 还需 DEPLOY_*

docker compose up -d
curl http://localhost:5678/healthz
docker compose kill -s HUP agent   # 热重载人设/技能
docker compose down
```

Mac 一键发版（`.env` 必填 `DEPLOY_SERVER` + `DEPLOY_SERVER_DIR`）：

```bash
make deploy
```

详见 [deploy/README.md](deploy/README.md) / [deploy/README.zh-CN.md](deploy/README.zh-CN.md)。

`docker-compose.yml` 内置：端口 5678、`.env`、`workspace/` 挂载、`agent-data` 卷、healthcheck、日志轮转、资源限制（1.5 CPU / 800M）。

### 方式 B：systemd（裸机）

```bash
sudo mkdir -p /opt/tiny-bot
sudo cp bin/tiny-bot-cloud-agent /opt/tiny-bot/
sudo cp -r workspace /opt/tiny-bot/

sudo mkdir -p /etc/tiny-bot
sudo cp .env.example /etc/tiny-bot/cloud-agent.env
sudo vi /etc/tiny-bot/cloud-agent.env

sudo cp deploy/tiny-bot-cloud-agent.service /etc/systemd/system/
sudo useradd -r -s /usr/sbin/nologin tinybot || true
sudo chown -R tinybot:tinybot /opt/tiny-bot
sudo systemctl daemon-reload
sudo systemctl enable --now tiny-bot-cloud-agent
```

## 内存 / 资源

- 静态二进制 ~15 MB
- 常驻 Go ~115 MB
- 2 GB VPS 上余量充足，可扛 burst

## 项目规范

见 [CLAUDE.md](CLAUDE.md)。变更日志：[CHANGELOG.md](CHANGELOG.md)。
