# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- **数字音量 `TB_SPK_GAIN_Q8`**：TTS 写入喇叭前按 Q8 增益并饱和；默认 220（约 0.86，预留削波余量）；GAIN=6dB 后仍偏小可调到 256～320
- **空闲停喇叭 I2S 时钟**：IDLE/非播放时 `i2s_stop`（无 BCLK → MAX98357 休眠），减轻空载 PWM 嘶嘶；出声前 `i2s_start` 并先写静音
- **Thinking / 播报打断（barge-in）**：WAITING/PLAYING 能量 VAD 或 BOOT 发 `interrupt` 并进入 RECORD；PLAYING 用更高阈值 `TB_VAD_BARGE_*`；忽略打断后过期 `done`
- **WebSocket `status` 进度回调**：`CloudClient` 解析下行 `type=status`（`step`/`phase`/`text`），经 `OnStatus` 回调；`main` 在 WAITING 等状态用 `text`（截断 21 字符）更新 OLED。用于展示 agent 多步 tool 进度且不触发 TTS；`tool` 仍作无 `status` 时的兜底显示。

### Fixed

- **`Makefile` 找不到 `pio`**：`PIO` 不再写死为 `pio`，会依次查找 PATH 中的 `pio` / `platformio`，以及 `~/.platformio/penv/bin/pio`；均未找到时给出安装提示。避免仅装了 PlatformIO IDE、CLI 未进 PATH 时 `make audio` / `make main` 直接失败。
- **`playBeep` 栈溢出**：`int16_t buf[4800]` 撑爆 `loopTask`，provision 失败后蜂鸣即重启；改为分块写入。

### Changed

- **测试说明补 `status` / barge-in 串口口径**：完整固件应出现 `[cloud] << status`、`[main] STATUS`；打断时应出现 `[cloud] >> interrupt` 与 `[main] barge-in`
- **`README.md` 快速开始**：补充 PlatformIO CLI 常见路径、将 CLI 加入 PATH 的写法，以及「不改 PATH 可直接 `make …`」的说明；烧录示例增加 `make audio`。
- **`step3_audio` 实时回放**：麦克风 → 喇叭 loopback（16-bit 直读）；启动提示音；`LOOPBACK_GAIN` 默认 2。
- **文档补充 `seed` 含义**：`seed` 是云端预登记设备（入场许可），不是立刻上线；未 seed 会导致 provision 401 / OLED Reconnecting。

### Removed

- **回滚 32-bit / 声道选择实验**：`MIC_PCM_SHIFT`、`RIGHT_LEFT` 能量选声道在本硬件上导致无声或大噪音，已恢复为可正常回放说话的 16-bit `ONLY_LEFT` 路径。

## [0.3.0] - 2026-07-18

### Added

- **连接健壮性**：WS 断线指数退避重连；`AUTH_FAIL` 清 token 后 `force=1` 重配网；`TAKEN_OVER` 等 5s 再连
- **心跳**：15s `ping`；30s 无服务端消息主动重连
- **打断**：WAITING/PLAYING 按 BOOT 发 `interrupt`，停 I2S 回 IDLE
- **TLS**：`TB_USE_TLS` 控制 HTTPS provision + WSS（`beginSSL`）
- **协议对齐**：`hello.proto` 用 `TB_PROTOCOL_VERSION`；校验 `hello.ok.sample_rate`；`OnTool` / `stt.lang`
- **UX**：仅 `isReady()` 可录音；断线/鉴权失败 OLED 显示 Reconnecting

### Changed

- 移除错误的 `State::RECORDING` / `isRecording()`（录音状态只在 main FSM）
- `OnSTT` 签名增加 `lang` 参数

## [0.2.0] - 2026-06-08

### Added

- **`src/cloud_client.h` / `src/cloud_client.cpp`** — 与 `tiny-bot-cloud-agent` 对接的客户端
  - `HTTP POST /provision` 自动配网（NVS 持久化 token）
  - WebSocket 连接 + `hello` 鉴权
  - `sendAudio()` / `sendEnd()` / `sendPing()`
  - 回调 `onSTT` / `onText` / `onTTS` / `onDone` / `onError`
  - 自带标准 base64 编解码（不依赖额外库）
  - 30s 心跳 ping
  - ERROR 状态 30s 自动重连

- **`src/main.cpp` 完整状态机**
  - `IDLE → RECORD → WAITING → PLAYING → IDLE`
  - BOOT 按键（`GPIO0`）按下开始录音、松开发送 `end`
  - I2S 麦克风按 `AUDIO_CHUNK_SAMPLES=512` (32ms @ 16kHz) 切片
  - I2S 喇叭直接喂 base64 解码后的 PCM
  - OLED 实时显示 STT 文本与 LLM 流式回复
  - 录音超时 `AI_MAX_AUDIO_LENGTH_MS=10000` 自动停
  - 启动后响一声 1kHz / 200ms 提示音

- **`platformio.ini`** 增补依赖
  - `links2004/WebSockets@^2.3.6`
  - `bblanchon/ArduinoJson@^7.0.4`
  - `[env:main]` 的 `src_filter` 加 `cloud_client.cpp`

- **`include/config.h.example`**
  - 新增 `HTTP_BASE` / `WS_HOST` / `WS_PORT` / `WS_PATH`
  - 新增 `TB_DEVICE_ID` / `TB_PAIRING_CODE`
  - 新增 `BTN_PIN` / `AUDIO_CHUNK_SAMPLES` / `WS_MAX_FRAME_BYTES` / `TB_PROTOCOL_VERSION`
  - `I2S_SPK_SAMPLE_RATE` 从 `24000` 改为 `16000`（与云端 TTS 输出对齐）

- **`CLAUDE.md`**
  - 状态机 ASCII 图
  - 云端对接规范引用（机器可读 + 人类可读）
  - 联调步骤（云端启动 → fake-device 自检 → 烧录固件）
  - 已知限制（打断 / VAD / 多设备）

## [0.1.0] - 2026-05-27

### Added

- `platformio.ini` - PlatformIO 项目配置，ESP32 + Arduino 框架
- `src/step1_blink.cpp` - Blink 测试固件（LED 闪烁）
- `src/step2_oled.cpp` - OLED 测试固件（SSD1306 显示）
- `src/step3_audio.cpp` - 音频测试固件（MS4030 麦克风 + MAX98357 功放）
- `src/main.cpp` - 主固件框架（预留云端对接接口）
- `include/config.h.example` - 配置文件模板
- `CLAUDE.md` - 项目规范文档
- `test/README.md` - 测试说明
- `docs/step-by-step-build-guide.md` - 分阶段接线测试指南
