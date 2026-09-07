# TinyBot Firmware 项目规范

## 项目概述

- **项目名称**: ai-tiny-bot 陪伴机器人
- **硬件平台**: ESP32 Dev Module
- **框架**: PlatformIO + Arduino
- **语言**: C++

## 硬件配置

| 模块 | 型号 | 接口 | 引脚 |
|------|------|------|------|
| 主控 | ESP32 Dev Module | - | - |
| 屏幕 | SSD1306 128x64 OLED | I2C | SDA=GPIO21, SCL=GPIO22 |
| 麦克风 | MS4030 I2S | I2S | SCK=GPIO25, WS=GPIO33, SD=GPIO32 |
| 功放 | MAX98357 | I2S | DIN=GPIO14, BCLK=GPIO26, LRCLK=27 |
| 按键 | DevKit BOOT | GPIO | GPIO0 (LOW = pressed) |
| 电源 | TP4056 LiPo | - | 5V to ESP32 VIN |

## 目录结构

```
tiny-bot-firmware/
├── platformio.ini                  # PlatformIO 项目配置
├── src/
│   ├── main.cpp                    # 主固件入口：状态机 + 录音/放音 + cloud_client
│   ├── cloud_client.h              # 云端客户端头（API + 回调）
│   ├── cloud_client.cpp            # 云端客户端实现（HTTP provision + WS）
│   ├── step1_blink.cpp             # Step1: LED闪烁测试
│   ├── step2_oled.cpp              # Step2: OLED屏幕测试
│   └── step3_audio.cpp             # Step3: 音频模块测试
├── include/
│   └── config.h.example            # 配置文件模板（复制为 config.h 使用）
└── test/
    └── README.md                   # 测试说明
```

## 状态机

```
        ┌────────────┐  按键按下  ┌─────────┐  按键松开  ┌──────────┐
        │   IDLE     │ ─────────► │ RECORD  │ ────────► │ WAITING  │
        │ (需 READY) │            │ Listen  │            │ Think... │
        └────────────┘            └─────────┘            └─────┬────┘
              ▲                                                  │ audio
              │ done / interrupt                          ┌─────▼────┐
              └────────────────────────────────────────── │ PLAYING  │
                     (interrupt: 发 interrupt + 停播)      └──────────┘
```

详见 [../tiny-bot-cloud-agent/docs/human-docs/02-firmware-integration.md](../tiny-bot-cloud-agent/docs/human-docs/02-firmware-integration.md)。

## 云端通信协议

通过 WebSocket 连接 [tiny-bot-cloud-agent](../tiny-bot-cloud-agent/)。完整规范：

- 机器可读：[ws-protocol.schema.json](../tiny-bot-cloud-agent/docs/firmware-integration/ws-protocol.schema.json)
- HTTP API：[http-api.openapi.yaml](../tiny-bot-cloud-agent/docs/firmware-integration/http-api.openapi.yaml)
- 人类可读：[02-firmware-integration.md](../tiny-bot-cloud-agent/docs/human-docs/02-firmware-integration.md)

**音频约定**：
- 上行（mic → cloud）：PCM 16 kHz 16-bit mono，base64 编码
- 下行（cloud → spk）：PCM 16 kHz 16-bit mono，base64 编码（v1，与 `hello.ok.sample_rate` 一致）

**鉴权**：
- 启动时若 NVS 无 token → `POST /provision?device_id=…&code=…` 拿 token
- token 存 NVS（Preferences namespace=`tinybot`，key=`token`）
- 后续 WebSocket `hello` 消息携带 token（`proto` = `TB_PROTOCOL_VERSION`）
- `AUTH_FAIL` → 清 NVS → `force=1` 重配网 → 重连 WS
- `TB_USE_TLS=1` 时用 HTTPS + WSS

**心跳 / 重连**：
- 每 `TB_PING_INTERVAL_MS`（默认 15s）发 `ping`
- `TB_SERVER_IDLE_MS`（默认 30s）无服务端消息则主动重连
- 断线指数退避，上限 `TB_RECONNECT_MAX_MS`

## 关键库

| 库 | 用途 |
|---|---|
| `links2004/WebSockets` | WebSocket 客户端（支持 ws / wss） |
| `bblanchon/ArduinoJson@7` | JSON 编解码 |
| 内置 `HTTPClient` | HTTP /provision 调用 |
| 内置 `Preferences` | NVS 读写（token 持久化） |
| 内置 `mbedtls/base64`（本项目自带） | 标准 base64 编解码 |

## 编译和烧录

```bash
# 默认（main 固件）
pio run -e main --target upload
pio device monitor        # 串口监视

# 分阶段测试
make blink                # step1: LED 闪烁
make oled                 # step2: OLED
make audio                # step3: 音频
make main                 # step4: 完整云端对话
```

## 联调

```bash
# 1) 准备云端
cd ../tiny-bot-cloud-agent
make build-all
./bin/seed -device-id tinypal-01 -pairing-code ABCD-1234
./bin/tiny-bot-cloud-agent

# 2) 用 fake-device 自检云端链路
./bin/fake-device -device-id tinypal-01 -code ABCD-1234

# 3) 烧录固件
cd ../tiny-bot-firmware
make main

# 4) 按 BOOT 按键，看到 OLED 显示 "Listening..."
# 说话，松开 → OLED 显示 "Thinking..." 然后 "Speaking"，喇叭播放
```

## 开发原则

1. **分阶段验证硬件**，先测试后集成（step1 → step2 → step3 → main）
2. **每阶段测试代码独立**，便于调试
3. **配置信息**（WiFi 密码、pairing code）不提交到仓库（`config.h` 在 `.gitignore`）
4. **保持代码简洁**，注释关键逻辑
5. **不写魔数**——所有时长/采样率/端口走 `config.h`

## 每次可交付变更（ship）

与仓库根 [`AGENTS.md`](../AGENTS.md) 一致；`stop` hook 会检查：

1. 在本目录 `CHANGELOG.md` 的 `[Unreleased]` 追加一行（变更要点）
2. 仅当约定 / 命令 / 目录结构变化时更新本 `CLAUDE.md` 或根 `AGENTS.md`（不要写成第二份 changelog）
3. 英文 commit subject + body（HEREDOC），`git push -u origin HEAD`（本仓允许直接推 `main`）；禁止 force push
4. 不提交 `config.h`、WiFi 密码、pairing code、真实公网 IP/端口等敏感信息

## 已知限制 / 后续 TODO

- **VAD**：仍用按键控制起停，未做静音检测
- **TLS**：无证书 pinning（`setInsecure`）；生产建议改 CA
- **采样率**：`hello.ok.sample_rate` 仅告警，不动态改 I2S
- **多设备**：当前 NVS 名字写死，单设备够用
- **tool / status**：`status` 更新 OLED 进度；`tool` 为兜底展示，不执行设备侧动作
