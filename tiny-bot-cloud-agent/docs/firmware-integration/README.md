# firmware-integration

Specs for [tiny-bot-firmware](../../../tiny-bot-firmware/) (ESP32) ↔ this agent.

[中文文档](README.zh-CN.md)

| File | Purpose | Audience |
|---|---|---|
| [ws-protocol.schema.json](ws-protocol.schema.json) | WebSocket **JSON Schema** | AI / tests / validators |
| [http-api.openapi.yaml](http-api.openapi.yaml) | HTTP **OpenAPI 3.0** | AI / client codegen |
| Human guide | [docs/human-docs/02-firmware-integration.md](../human-docs/02-firmware-integration.md) | Embedded developers |

## Usage

### AI / automation

- Codegen structs from `ws-protocol.schema.json` (e.g. `quicktype`)
- HTTP clients from `http-api.openapi.yaml` (`openapi-generator`)
- Contract tests with `ajv` / `jsonschema` in CI

### Embedded developers

Read [02-firmware-integration.md](../human-docs/02-firmware-integration.md) for flows, messages, pitfalls, and snippets.

## Version

| Spec | Go code | Notes |
|---|---|---|
| 1.0.0 | `internal/ws/protocol.go` (v1) | current |

Incompatible changes bump major and land in [CHANGELOG.md](../../CHANGELOG.md).
