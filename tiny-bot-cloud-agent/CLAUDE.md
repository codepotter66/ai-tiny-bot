# tiny-bot-cloud-agent 项目规范

## 项目概述

- **项目名称**: tiny-bot-cloud-agent
- **目标**: 为 ai-tiny-bot 陪伴机器人提供云端 Agent 服务
- **硬件对接**: ESP32（见 `../tiny-bot-firmware/`），通过 WebSocket 双向 PCM 音频
- **语言**: Go 1.22+
- **目标部署**: 2 vCPU / 2 GB RAM 单机（systemd 直跑，推荐）
- **二进制**: 单文件，distroless static 可选

## 与 tiny-bot-firmware 的契约

- WebSocket 端点: `ws://<host>:5678/ws`
- 上行音频: PCM 16 kHz / 16 bit / 单声道，base64 编码 JSON 消息
- 下行音频: PCM 16 kHz / 16 bit / 单声道，base64 编码 JSON 消息
- 协议详细定义: `internal/ws/protocol.go`

## 目录结构

```
tiny-bot-cloud-agent/
├── cmd/                  # 可执行入口
│   ├── tiny-bot-cloud-agent/   # 主服务
│   ├── seed/                   # 初始化用户/设备
│   ├── echo-agent/            # 文本→pipeline 自检
│   └── asr-test/              # 直连阿里云 ASR 调试（CreateToken + 识别）
├── tools/fake-device/    # 假 ESP32 客户端（开发自检用）
├── internal/             # 业务包
│   ├── config/           # YAML + env 配置
│   ├── logging/          # slog
│   ├── server/           # http.Server 路由
│   ├── ws/               # WebSocket 升级、session、协议
│   ├── auth/             # 鉴权 + 配网
│   ├── devices/          # 设备→用户路由
│   ├── audio/            # PCM 工具 + 句子聚合
│   ├── asr/              # STT 接口 + 实现
│   ├── tts/              # TTS 接口 + 实现
│   ├── llm/              # LLM 接口 + 实现 + prompt 组合
│   ├── agent/            # 核心 turn 循环
│   ├── skills/           # 技能加载/执行
│   ├── memory/           # 记忆读写/召回
│   ├── persona/          # SOUL/IDENTITY/AGENT/USER 加载
│   ├── store/            # SQLite
│   └── bus/              # 进程内 pub/sub
├── workspace/            # 运行时数据（人设/记忆/技能）
├── deploy/               # systemd unit
├── docker/               # Dockerfile
├── test/                 # 集成测试 + testdata
├── .env                  # 全部配置（gitignore）
├── .env.example          # 配置模板（提交）
├── docker-compose.yml    # 推荐部署方式
├── Dockerfile → docker/Dockerfile
├── Makefile              # build/run/test/lint/fake-device
├── go.mod
├── go.sum
├── CHANGELOG.md          # 每次变更追加
└── README.md
```

## 命名约定

- 包名: 全小写、单词 (`config`, `agent`, `store`)
- 文件名: 全小写 + 下划线分隔 (`protocol.go`, `fake_device.go`)
- 接口: 动名词 (`Transcriber`, `Synthesizer`, `Chat`)
- 实现: 单词后缀 (`Mock`, `OpenAICompat`, `Aliyun`)

## 关键约定

1. **配置单一来源**: 全部走 `.env`（一份文件），见下文
2. **依赖最少化**: 直依赖 4 个（`coder/websocket`, `modernc.org/sqlite`, `google/uuid`, `stretchr/testify`）
3. **接口 + Mock**: `asr/tts/llm` 全部接口化，v1 默认 Mock 实现，便于切换
4. **不引入向量库**: 2c2g 跑不起 embedding，记忆用关键词 + 时间衰减打分
5. **静态二进制优先**: 用 `modernc.org/sqlite`（无 CGo），distroless 镜像可行
6. **SIGHUP 热重载**: 重新读取 `.env` 与 `workspace/*.md`，不杀连接
7. **每改必验**: 改完跑 `make test`，全绿再提交
8. **每次可交付变更**: 追加 `CHANGELOG.md` 一行（变更要点）

## 配置：单一 `.env` 文件

**全部配置集中在一个 `.env` 文件**（2026-06-12 砍掉了 `config.yaml`）。优先级：

```
进程 env vars  >  ./.env 文件  >  代码内 defaults
```

| 启动方式 | 谁读 `.env` | 备注 |
|---|---|---|
| `make run` | Go 启动时 `loadDotEnv()` 加载 | 与 docker 行为一致 |
| `docker compose up` | docker 自动喂容器 | `env_file: .env` |
| `systemctl start` | systemd 读 `EnvironmentFile` | `/etc/tiny-bot/cloud-agent.env`，**完全独立于**项目里的 `.env` |

**`.env` 字段命名**：所有变量用 `TB_*` 前缀（`TB_AGENT_ADDR`、`TB_OPENAI_COMPAT_API_KEY` 等）。完整列表见 [`.env.example`](.env.example)。

**安全点**：进程里已有的 env 变量不会被 `.env` 覆盖，避免被注入的 `.env` 劫持（见 `internal/config/config.go:loadDotEnv`）。

**典型场景**：

- **改端口**：改 `.env` 的 `TB_AGENT_ADDR=:5678`
- **改 API key**：改 `.env` 的 `TB_OPENAI_COMPAT_API_KEY`
- **临时调日志**：`TB_LOG_LEVEL=debug make run`
- **本地首次跑**：`cp .env.example .env && vi .env`（填 API key），然后 `make run`

## 常用命令

```bash
make build       # 编译
make run         # 本地启动（自动加载 .env）
make test        # 跑全部单测
make test-race   # race detector
make fake-device # 用假 ESP32 跑一轮
make lint        # golangci-lint
make tidy        # go mod tidy
```

## 开发原则

1. 一次只动一个包，跑通测试再进下一个
2. 接口与实现分离：先写接口、Mock、test，再写真实实现
3. 不要为跑通而注释掉错误处理 — 找根因
4. 密钥/token 不进代码、不进 commit、不进日志
5. 大改动前先在 Plan 模式出方案
6. 改完 `CHANGELOG.md` 追加一行

## 详细方案

`/Users/kyle.he/.claude/plans/tiny-bot-cloud-agent-agent-openclaw-age-lucky-hoare.md`
