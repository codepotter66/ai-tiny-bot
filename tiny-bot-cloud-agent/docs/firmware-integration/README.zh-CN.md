# firmware-integration

给 [tiny-bot-firmware](../../../tiny-bot-firmware/)（ESP32）对接用的规范。

[English](README.md)

| 文件 | 用途 | 谁看 |
|---|---|---|
| [ws-protocol.schema.json](ws-protocol.schema.json) | WebSocket 消息的 **JSON Schema** | AI / 自动化测试 / 协议校验器 |
| [http-api.openapi.yaml](http-api.openapi.yaml) | HTTP 端点的 **OpenAPI 3.0** 规范 | AI / 客户端代码生成 |
| 人类可读版 | 同 [docs/human-docs/02-firmware-integration.md](../human-docs/02-firmware-integration.md) | 嵌入式开发者 |

## 用法

### AI / 自动化

- 协议生成代码：把 `ws-protocol.schema.json` 喂给 JSON Schema 代码生成器（如 `quicktype`）
- HTTP 客户端代码生成：`http-api.openapi.yaml` → `openapi-generator`
- 契约测试：用 `ajv` / `jsonschema` 在 CI 里校验消息合规

### 嵌入式开发者

直接看 [docs/human-docs/02-firmware-integration.md](../human-docs/02-firmware-integration.md)——逐步讲解流程、消息、坑点、示例代码片段。

## 版本

| Spec 版本 | 对应 Go 代码 | 备注 |
|---|---|---|
| 1.0.0 | `internal/ws/protocol.go` (v1) | 当前 |

任何不兼容改动都升 major 版本号，并在 [CHANGELOG.md](../../CHANGELOG.md) 记录。
