# ai-tiny-bot

Hands-free companion robot: an ESP32 device talks to a Go cloud agent over WebSocket (raw PCM 16 kHz mono). Speak anytime — the board uses energy VAD; the cloud runs ASR → LLM → TTS.

[中文文档](README.zh-CN.md)

## Repository layout

| Path | Role |
|------|------|
| [`tiny-bot-firmware/`](tiny-bot-firmware/) | ESP32 firmware (PlatformIO): Wi‑Fi, provision, WebSocket, OLED face, I2S mic/speaker |
| [`tiny-bot-cloud-agent/`](tiny-bot-cloud-agent/) | Cloud agent (Go): WebSocket protocol, persona/memory/skills, ASR/LLM/TTS providers |
| [`docs/`](docs/) | Hardware design, build guides, arrival checks |

## How it fits together

```
ESP32  --POST /provision-->  cloud-agent  (pairing code → token)
ESP32  <--WebSocket /ws---->  cloud-agent  (PCM up / STT+text+PCM down)
```

1. Seed a device on the server (`device_id` + `pairing_code`).
2. Flash firmware with matching IDs, Wi‑Fi, and cloud host.
3. Power on → provision → speak; OLED shows face states, not chat text.

## Quick start

### Cloud agent (local)

```bash
cd tiny-bot-cloud-agent
cp .env.example .env   # fill API keys; never commit .env
make build-all
make seed              # or: make seed-remote after deploy
make run
curl http://localhost:5678/healthz
```

Deploy (requires `DEPLOY_SERVER` and `DEPLOY_SERVER_DIR` in `.env`):

```bash
cd tiny-bot-cloud-agent
make deploy
```

Details: [tiny-bot-cloud-agent/README.md](tiny-bot-cloud-agent/README.md) ([中文](tiny-bot-cloud-agent/README.zh-CN.md)) · [deploy/README.md](tiny-bot-cloud-agent/deploy/README.md)

### Firmware

```bash
cd tiny-bot-firmware
cp include/config.h.example include/config.h   # private; gitignored
# edit Wi-Fi, HTTP_BASE / WS_HOST, TB_DEVICE_ID, TB_PAIRING_CODE
make main
make monitor
```

Details: [tiny-bot-firmware/README.md](tiny-bot-firmware/README.md) ([中文](tiny-bot-firmware/README.zh-CN.md))

### Browser demo

With the agent running: `http://localhost:5678/demo/`  
Mobile needs HTTPS (self-signed or tunnel) — see [demo/README.md](tiny-bot-cloud-agent/demo/README.md).

## Secrets and local config

Do **not** commit:

- `tiny-bot-cloud-agent/.env` — API keys, `DEPLOY_SERVER`, `DEPLOY_SERVER_DIR`, `TB_PUBLIC_IP`
- `tiny-bot-firmware/include/config.h` — Wi‑Fi and host

Templates: `.env.example`, `include/config.h.example`. Root [`.gitignore`](.gitignore) covers env files, build artifacts, and archives.

## Docs

- Build walkthrough: [docs/step-by-step-build-guide.md](docs/step-by-step-build-guide.md)
- Hardware: [docs/hardware-design.md](docs/hardware-design.md)
- Cloud architecture: [tiny-bot-cloud-agent/docs/human-docs/00-architecture.md](tiny-bot-cloud-agent/docs/human-docs/00-architecture.md)
- Firmware ↔ cloud protocol: [tiny-bot-cloud-agent/docs/human-docs/02-firmware-integration.md](tiny-bot-cloud-agent/docs/human-docs/02-firmware-integration.md)

## License

See individual subprojects if a license file is present; otherwise all rights reserved by the authors unless stated otherwise.
