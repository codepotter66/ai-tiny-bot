# human-docs

给人看的文档。跟代码注释（给机器看）不同，这里解释「为什么这样设计」「怎么用」「怎么改」。

[English](README.md)

按主题分文件。命名用 kebab-case。

## 目录

| 文件 | 主题 | 状态 |
|---|---|---|
| [00-architecture.md](00-architecture.md) | 整体架构：模块职责、数据流、人设/记忆/技能、扩展指南 | ✅ |
| [01-database.md](01-database.md) | 持久化：用什么 DB、存什么、怎么改 | ✅ |
| [02-firmware-integration.md](02-firmware-integration.md) | 固件对接指南（与 [../firmware-integration/](../firmware-integration/) 配对） | ✅ |
| [05-deployment.md](05-deployment.md) | 国内小 VPS 部署：docker daemon mirror + Dockerfile build-arg | ✅ |
| [09-internal-packages.md](09-internal-packages.md) | `internal/` 各 Go 包职责、接口、依赖与改代码指引 | ✅ |
| [03-protocol.md](03-protocol.md) | WebSocket / HTTP 协议字段详解 | 📝 计划中 |
| [04-persona-memory.md](04-persona-memory.md) | 人设 + 记忆机制 | 📝 计划中 |
| [05-skills.md](05-skills.md) | 技能系统 | 📝 计划中 |
| [06-deployment.md](06-deployment.md) | 部署（docker-compose / systemd） | 📝 计划中 |
| [07-providers.md](07-providers.md) | LLM / ASR / TTS provider 接入 | 📝 计划中 |
| [08-observability.md](08-observability.md) | 日志 / 监控 / 排障 | 📝 计划中 |

## 机器可读文档（给 AI / 自动化用）

| 路径 | 用途 |
|---|---|
| [../firmware-integration/ws-protocol.schema.json](../firmware-integration/ws-protocol.schema.json) | WebSocket 消息 JSON Schema |
| [../firmware-integration/http-api.openapi.yaml](../firmware-integration/http-api.openapi.yaml) | HTTP 端点 OpenAPI 3.0 |
| [../firmware-integration/README.md](../firmware-integration/README.md) | 机器可读文档索引 |

写新文档时，把上表的状态从 📝 改成 ✅ 并补上日期。
