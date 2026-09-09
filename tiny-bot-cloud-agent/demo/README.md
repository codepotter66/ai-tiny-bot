# Local demo frontend

Single-page HTML debugger: open in a browser and talk to the agent.

[中文文档](README.zh-CN.md)

## Usage

### 1. Start cloud-agent

```bash
cd ../   # tiny-bot-cloud-agent root
make run
```

Or exercise the protocol with fake-device:

```bash
./bin/fake-device -device-id tinypal-demo -code DEMO-1234
```

Keep the browser Device ID as `tinypal-demo`. Do not provision with the speaker's firmware Device ID (`force=1` would rotate that token and drop the hardware WebSocket). On the server, `make seed-demo-remote` registers the demo device without restarting the agent.

### 2. Open the demo

**Recommended (same origin as the agent):**

```bash
make run
# open http://localhost:5678/demo/
```

On a remote deploy, phones need **HTTPS** (see below). Desktop `localhost` can stay on HTTP.

**Phone, no domain (tunnel):**

```bash
cd "$DEPLOY_SERVER_DIR/tiny-bot-cloud-agent"
docker compose --profile tunnel up -d
docker compose logs cloudflared   # copy https://xxx.trycloudflare.com
# phone: https://xxx.trycloudflare.com/demo/
```

**Phone, self-signed HTTPS:**

```bash
# open security group 443
docker compose --profile https-selfsigned up -d
# phone: https://<server-ip>/demo/ (trust cert once)
```

**With a domain:**

1. Set `TB_PUBLIC_DOMAIN=demo.example.com` in `.env`
2. `docker compose --profile https up -d`
3. `https://demo.example.com/demo/`

**Standalone local HTTP (dev):**

```bash
make demo
# http://localhost:8888 — set Server URL to http://localhost:5678

open demo/index.html   # Safari may block mic
```

### 3. In the browser

1. Leave Server URL empty on same-origin; when opening the file alone, set `http://localhost:5678`
2. **Provision** → `POST /provision`, token in localStorage
3. **Connect WebSocket** → `hello`
4. Hold **Push to talk** (on phone, expand settings for provision UI)
5. Release → PCM chunks → `end`
6. Hear reply; STT / LLM stream in the chat pane

### 4. Real LLM

In `.env`:

```bash
TB_PROVIDER_LLM=openai_compat
TB_OPENAI_COMPAT_BASE_URL=https://api.minimaxi.com
TB_OPENAI_COMPAT_API_KEY=sk-cp-xxx
TB_OPENAI_COMPAT_MODEL=MiniMax-M3
```

Restart the agent and try the demo again.

## Features

| Capability | Implementation |
|---|---|
| HTTP provision | `POST /provision?device_id=…&code=…` |
| WebSocket | `hello` / `hello.ok` auth |
| Capture | `MediaRecorder` → decode → resample 16 kHz → Int16 → base64 |
| Audio chunks | ~4 KB base64 / frame |
| `end` | On stop recording |
| Inbound | `stt` / `text` / `audio` / `done` |
| UI text | STT = user; streaming LLM = assistant |
| Playback | Int16 → Float32 → AudioContext, sequential |
| Log | Scroll log of WS + system events |

## Debug tips

- **F12** → Network / WS for raw frames  
- Token preview (first 16 chars) — same prefix ⇒ token unchanged  
- Bottom log — protocol errors land here  
- Clear log / clear chat — does not drop WS  

## Limits

- Chrome / Edge / Safari 16+  
- Mic needs a **secure context**: `https://` or `http://localhost` (not bare LAN/public HTTP or `file://`)  
- Mic permission required  
- One browser session at a time  
