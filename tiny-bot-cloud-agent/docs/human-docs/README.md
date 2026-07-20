# human-docs

Human-oriented docs (why / how / how to change). Separate from code comments and machine-readable specs.

[中文文档](README.zh-CN.md)

Filenames use kebab-case.

## Index

| File | Topic | Status |
|---|---|---|
| [00-architecture.md](00-architecture.md) | Architecture, data flow, persona/memory/skills | ✅ |
| [01-database.md](01-database.md) | Persistence | ✅ |
| [02-firmware-integration.md](02-firmware-integration.md) | Firmware integration (pairs with [../firmware-integration/](../firmware-integration/)) | ✅ |
| [05-deployment.md](05-deployment.md) | Small VPS deploy: daemon mirror + Dockerfile | ✅ |
| [09-internal-packages.md](09-internal-packages.md) | `internal/` Go packages | ✅ |
| [03-protocol.md](03-protocol.md) | WS / HTTP field detail | 📝 planned |
| [04-persona-memory.md](04-persona-memory.md) | Persona + memory | 📝 planned |
| [05-skills.md](05-skills.md) | Skills | 📝 planned |
| [06-deployment.md](06-deployment.md) | Deploy (compose / systemd) | 📝 planned |
| [07-providers.md](07-providers.md) | LLM / ASR / TTS providers | 📝 planned |
| [08-observability.md](08-observability.md) | Logs / metrics / triage | 📝 planned |

## Machine-readable docs

| Path | Purpose |
|---|---|
| [../firmware-integration/ws-protocol.schema.json](../firmware-integration/ws-protocol.schema.json) | WebSocket JSON Schema |
| [../firmware-integration/http-api.openapi.yaml](../firmware-integration/http-api.openapi.yaml) | HTTP OpenAPI 3.0 |
| [../firmware-integration/README.md](../firmware-integration/README.md) | Machine-doc index |

When you finish a planned doc, flip 📝 → ✅ and add a date.
