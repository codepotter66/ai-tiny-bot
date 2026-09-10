# TinyBot Firmware

ESP32 firmware for the [ai-tiny-bot](../) companion robot. Talks to [tiny-bot-cloud-agent](../tiny-bot-cloud-agent/).

[中文文档](README.zh-CN.md)

## Quick start

### 1. Prepare the cloud agent

```bash
cd ../tiny-bot-cloud-agent
make build-all
./bin/seed -device-id tinypal-01 -pairing-code ABCD-1234
./bin/tiny-bot-cloud-agent
```

**What does `seed` do?**

It **pre-registers** a device in the cloud SQLite DB (`device_id` + `pairing_code`) — an “entry pass” for this ESP32.  
It does **not** bring the board online by itself. Full flow:

1. Server runs `seed` → device row exists  
2. Board boots → Wi‑Fi → `POST /provision` (same ID/code) → token  
3. Connects `ws://…/ws` → ready to talk  

Skip `seed` and you get `device not found` / HTTP 401; OLED stays on `Reconnecting...`.  
`TB_DEVICE_ID` / `TB_PAIRING_CODE` in `config.h` must match `seed`. On a remote host, run `seed` (or `make seed-remote`) on the machine that runs cloud-agent.

### 2. Configure the firmware

```bash
cp include/config.h.example include/config.h
vi include/config.h   # Wi-Fi, WS_HOST, TB_DEVICE_ID, TB_PAIRING_CODE
```

`include/config.h` holds private Wi‑Fi / host settings and is **gitignored**. Only `config.h.example` is committed.

### 3. Install PlatformIO

Install the **PlatformIO IDE** extension (CLI is usually at `~/.platformio/penv/bin/pio`).

If `pio` is not on your `PATH`:

```bash
# Option A: add to PATH (~/.zshrc)
export PATH="$HOME/.platformio/penv/bin:$PATH"

# Option B: use the Makefile (resolves that path for you)
make audio
```

### 4. Flash

```bash
make audio           # mic + amp test firmware
make main            # build + flash full firmware
make monitor         # serial monitor
```

### 5. Use it

With the cloud ready, OLED shows a **face** and bottom status `Speak anytime`. **Just talk** (hands-free energy VAD; no BOOT button to start):

1. Idle: blink / look around  
2. Speak → listening face + `Listening...`  
3. ~1.1s silence → thinking + `Thinking...` → happy + `Speaking`  
4. During thinking/speaking: speak again or press **BOOT** to barge-in (serial: `[main] barge-in`, `[cloud] >> interrupt`)  
5. After TTS (or barge-in end), back to idle  

Transcript and reply text go to the **serial port**, not the OLED.  
Tune in `config.h`: `TB_VAD_*`, `TB_VAD_BARGE_*`, `TB_SPK_GAIN_Q8`. Loudness/noise: flash and listen on device (compile-checked only in CI/dev without board).

## Layout

```
tiny-bot-firmware/
├── platformio.ini
├── src/
│   ├── main.cpp                    # state machine + record/play
│   ├── oled_face.h / oled_face.cpp # OLED eye animations
│   ├── cloud_client.h / .cpp       # HTTP provision + WebSocket
│   ├── step1_blink.cpp
│   ├── step2_oled.cpp
│   └── step3_audio.cpp
├── include/
│   └── config.h.example
└── docs/
```

## Staged tests

Follow the [step-by-step build guide](../../docs/step-by-step-build-guide.md):

| Stage | File | Purpose |
|-------|------|---------|
| Step 1 | `src/step1_blink.cpp` | LED blink |
| Step 2 | `src/step2_oled.cpp` | OLED |
| Step 3 | `src/step3_audio.cpp` | Audio |
| Final | `src/main.cpp` | Full AI chat |

## Configuration

| Field | Required | Example | Notes |
|---|---|---|---|
| `WIFI_SSID` / `WIFI_PASSWORD` | yes | `"MyHomeWiFi"` | Wi‑Fi |
| `HTTP_BASE` | yes | `"http://192.168.1.100:5678"` | Cloud HTTP base |
| `WS_HOST` / `WS_PORT` / `WS_PATH` | yes | `"192.168.1.100"` / `5678` / `"/ws"` | WebSocket |
| `TB_DEVICE_ID` | yes | `"tinypal-01"` | Must match `seed` |
| `TB_PAIRING_CODE` | yes | `"ABCD-1234"` | Cleared from NVS after first provision |
| `TB_VAD_SPEECH_THRESHOLD` | | `18` | Speech peak threshold (0–100) |
| `TB_VAD_SILENCE_MS` | | `1100` | Silence to end utterance |
| `TB_VAD_MIN_SPEECH_MS` | | `400` | Min speech before silence can end |
| `TB_VAD_REARM_DELAY_MS` | | `250` | Cool-down before listening again |
| `TB_VAD_BARGE_THRESHOLD` | | `40` | Barge-in peak threshold while speaking |
| `TB_VAD_BARGE_CHUNKS` | | `3` | Consecutive loud chunks before barge-in (~96ms) |
| `TB_SPK_GAIN_Q8` | | `220` | Digital speaker gain (Q8; 256=1.0); raise if GAIN=6dB is quiet |

## Cloud integration

- Human guide: [../tiny-bot-cloud-agent/docs/human-docs/02-firmware-integration.md](../tiny-bot-cloud-agent/docs/human-docs/02-firmware-integration.md)
- Machine-readable: [../tiny-bot-cloud-agent/docs/firmware-integration/](../tiny-bot-cloud-agent/docs/firmware-integration/)

## Known limits

- **Barge-in**: energy + BOOT during thinking/speaking; raise `TB_VAD_BARGE_THRESHOLD` if speaker loopback false-triggers (no AEC)  
- **VAD**: on-device peak energy; noisy rooms may need a higher `TB_VAD_SPEECH_THRESHOLD`  
- **Audio UX on device**: digital gain + idle I2S clock stop are implemented; verify loudness/hiss on hardware  
- **OLED**: face + status bar only; chat text on serial  
- **Multi-device**: NVS key is fixed `tinybot/token` (fine for one device)  
- **WS auth fail**: clears NVS token and re-provisions on next boot  

## Debug tips

```bash
pio device monitor

wscat -c ws://192.168.1.100:5678/ws

pio run --target erase   # or esptool.py erase_flash — force re-provision
```
