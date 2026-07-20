# tiny-bot-cloud-agent

Cloud agent for the [ai-tiny-bot](../) companion robot (Go).

[中文文档](README.zh-CN.md)

- WebSocket `ws://<host>:5678/ws` for [`../tiny-bot-firmware/`](../tiny-bot-firmware/)
- Bidirectional raw PCM 16 kHz / 16-bit / mono (I2S playback on device; no codec)
- Persona (SOUL/IDENTITY/AGENT/USER), memory (Markdown + keyword/time decay), skills (`SKILL.md` + scripts)
- Target: 2 vCPU / 2 GB RAM; **docker compose preferred** (bare-metal systemd optional)
- Architecture: [docs/human-docs/00-architecture.md](docs/human-docs/00-architecture.md) · packages: [docs/human-docs/09-internal-packages.md](docs/human-docs/09-internal-packages.md)

## Quick start

```bash
# 1. Build
make build

# 2. DB + one pre-registered device
#    seed = register device_id/pairing_code (entry pass), not “online now”;
#    board still POSTs /provision for a token. No seed → 401 device not found.
#    Local: make seed
#    Remote (SSH from this machine): make seed-remote
make seed

# 3. Run
make run &
sleep 1

# 4. Health
curl http://localhost:5678/healthz
# {"ok":true,"uptime":1}

# 5. Fake ESP32 one turn
TB_FAKE_ASR_TEXT="how is the weather today" ./bin/fake-device -code ABCD-1234 -device-id tinypal-01
```

Copy `.env.example` → `.env` and fill secrets before real ASR/LLM/TTS. Never commit `.env`.

## Protocol overview

WebSocket text frames, JSON. See [internal/ws/protocol.go](internal/ws/protocol.go).

### Client → server

| type | Required | Meaning |
|------|----------|---------|
| `hello` | `device_id`, `token` | Auth |
| `audio` | `data` (base64 PCM) | Uplink chunk |
| `end` | — | End of utterance |
| `ping` | — | Keepalive |
| `interrupt` | — | Barge-in |

### Server → client

| type | Extra | Meaning |
|------|-------|---------|
| `hello.ok` | `sample_rate` | Auth OK |
| `stt` | `text` | ASR |
| `text` | `text` | LLM token |
| `audio` | `data`, `seq` | TTS chunk |
| `tool` | `tool`, `args` | Tool event |
| `done` | — | Turn end |
| `error` | `error` (`CODE:message`) | Error |

## HTTP endpoints

| Path | Method | Purpose |
|------|--------|---------|
| `/healthz` | GET | Liveness |
| `/readyz` | GET | Readiness |
| `/metrics` | GET | Simple JSON metrics |
| `/provision` | POST | Pairing (`?device_id=&code=` → token) |
| `/ws` | GET | WebSocket |
| `/demo/` | GET | Browser demo (if enabled) |

## Layout

```
tiny-bot-cloud-agent/
├── cmd/                # main, seed, echo-agent, asr-test, …
├── tools/fake-device/  # fake ESP32 client
├── internal/           # config, ws, auth, asr, tts, llm, agent, …
├── workspace/          # persona / memory / skills
├── deploy/             # Mac→server deploy scripts
├── docker/             # Dockerfiles
├── docker-compose.yml
└── test/
```

## Development

```bash
make build
make run
make test
make test-race
make asr-test       # Aliyun ASR smoke (needs .env + testdata PCM)
make llm-test       # MiniMax LLM smoke
make fake-device
make lint
make tidy
```

### Aliyun ASR smoke

Set `TB_ALIYUN_KEY` / `TB_ALIYUN_SECRET` / `TB_ALIYUN_ASR_APP_KEY` in `.env`, then:

```bash
make asr-test
# or: go run ./cmd/asr-test -file path/to/audio.pcm
```

Expect CreateToken OK → transcript. Full path: `make run` + `fake-device`.

### Add a skill

`workspace/skills/<name>/SKILL.md`:

```markdown
---
name: weather
description: Look up weather for a city
parameters:
  city:
    type: string
    description: City name
    required: true
script: scripts/run.sh
---

# weather skill body
```

`scripts/run.sh` reads JSON on stdin, writes result to stdout. Restart or SIGHUP to reload.

## Deploy

### A. docker compose (recommended)

```bash
cp .env.example .env
vi .env                 # keys + DEPLOY_* if using make deploy

docker compose up -d
curl http://localhost:5678/healthz
docker compose kill -s HUP agent   # reload persona/skills
docker compose down
```

One-shot from Mac (requires `DEPLOY_SERVER` + `DEPLOY_SERVER_DIR` in `.env`):

```bash
make deploy
```

See [deploy/README.md](deploy/README.md).

### B. systemd (bare metal)

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

## Memory / resources

- Static binary ~15 MB  
- Idle Go ~115 MB  
- Comfortable on 2 GB VPS with headroom for bursts  

## Conventions

See [CLAUDE.md](CLAUDE.md). Changelog: [CHANGELOG.md](CHANGELOG.md).
