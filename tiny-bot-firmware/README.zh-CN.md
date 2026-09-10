# TinyBot Firmware

ESP32 固件，用于 [ai-tiny-bot](../) 陪伴机器人。与 [tiny-bot-cloud-agent](../tiny-bot-cloud-agent/) 对接。

[English](README.md)

## 快速开始

### 1. 准备云端

```bash
cd ../tiny-bot-cloud-agent
make build-all
./bin/seed -device-id tinypal-01 -pairing-code ABCD-1234
./bin/tiny-bot-cloud-agent
```

**`seed` 是做什么的？**

它在云端 SQLite 里**预登记一台设备**（写入 `device_id` + `pairing_code`），相当于给这台 ESP32「办入场证」。  
它**不会**让板子立刻在线，只是登记许可。完整流程是：

1. 服务器执行 `seed` → 数据库有这台设备  
2. 板子上电 → 连 WiFi → `POST /provision`（用相同的 ID/配对码）→ 拿到 token  
3. 再连 `ws://…/ws` → 可以对话  

若跳过 `seed`，固件会出现 `device not found` / HTTP 401，OLED 一直显示 `Reconnecting...`。  
`config.h` 里的 `TB_DEVICE_ID` / `TB_PAIRING_CODE` 必须与 `seed` 参数一致。线上机器也要在**那台跑 cloud-agent 的机器上**执行一次 `seed`（或本机 `make seed-remote`）。

### 2. 配 ESP32 固件

```bash
cp include/config.h.example include/config.h
vi include/config.h   # 填 WiFi 密码 + WS_HOST + TB_DEVICE_ID + TB_PAIRING_CODE
```

`include/config.h` 含 WiFi / 服务器等私有配置，已被 `.gitignore` 忽略，勿提交；仓库只保留 `config.h.example` 模板。

### 3. 安装 PlatformIO

VS Code / Cursor 扩展市场搜索 **PlatformIO IDE**（安装后 CLI 一般在 `~/.platformio/penv/bin/pio`）。

若终端里直接敲 `pio` 提示找不到，可二选一：

```bash
# 方式 A：把 CLI 加进 PATH（写入 ~/.zshrc）
export PATH="$HOME/.platformio/penv/bin:$PATH"

# 方式 B：不用改 PATH，直接用 Makefile（已会自动找上述路径）
make audio
```

### 4. 烧录

```bash
make audio           # 音频测试固件（麦克风 + 功放）
make main            # 编译 + 烧录完整固件
make monitor         # 打开串口监视
```

### 5. 用起来

云端就绪后 OLED 显示**大眼睛表情** + 底栏 `Speak anytime`，**直接说话即可**（免提能量 VAD；开始录音不必按 BOOT）：

1. 空闲：眨眼、左右看  
2. 说话 → 聆听表情 + `Listening...`  
3. 说完安静约 1.1s → 思考表情 + `Thinking...` → 开心表情 + `Speaking`  
4. Thinking / 播报中再说话或按 **BOOT** 可打断（串口：`[main] barge-in`、`[cloud] >> interrupt`）  
5. 播完（或打断结束）回到空闲  

转写与回复文本只打在**串口**，不再刷满屏字。  
可在 `config.h` 调：`TB_VAD_*`、`TB_VAD_BARGE_*`、`TB_SPK_GAIN_Q8`。响度/底噪需上板听感确认（无板时仅保证编译通过）。

## 项目结构

```
tiny-bot-firmware/
├── platformio.ini                  # PlatformIO 项目配置
├── src/
│   ├── main.cpp                    # 主固件：状态机 + 录音/放音
│   ├── oled_face.h / oled_face.cpp # OLED 表情眼睛动画
│   ├── cloud_client.h              # 云端客户端头
│   ├── cloud_client.cpp            # 云端客户端实现（HTTP provision + WebSocket）
│   ├── step1_blink.cpp             # Step1 测试
│   ├── step2_oled.cpp              # Step2 测试
│   └── step3_audio.cpp             # Step3 测试
├── include/
│   └── config.h.example            # 配置模板（复制为 config.h）
└── docs/
```

## 分阶段测试

按 [Step-by-Step Build Guide](../../docs/step-by-step-build-guide.md) 顺序：

| 阶段 | 文件 | 内容 |
|------|------|------|
| Step 1 | `src/step1_blink.cpp` | LED 闪烁测试 |
| Step 2 | `src/step2_oled.cpp` | OLED 屏幕测试 |
| Step 3 | `src/step3_audio.cpp` | 音频模块测试 |
| Final | `src/main.cpp` | 完整 AI 对话 |

## 配置

| 字段 | 必填 | 示例 | 说明 |
|---|---|---|---|
| `WIFI_SSID` / `WIFI_PASSWORD` | ✅ | `"MyHomeWiFi"` | WiFi 凭据 |
| `HTTP_BASE` | ✅ | `"http://192.168.1.100:5678"` | 云端 HTTP 基础 URL |
| `WS_HOST` / `WS_PORT` / `WS_PATH` | ✅ | `"192.168.1.100"` / `5678` / `"/ws"` | WebSocket 端点 |
| `TB_DEVICE_ID` | ✅ | `"tinypal-01"` | 设备 ID（与 `seed` 一致） |
| `TB_PAIRING_CODE` | ✅ | `"ABCD-1234"` | 配对码（首次启动后 NVS 自动清掉） |
| `TB_VAD_SPEECH_THRESHOLD` | | `18` | 开始说话峰值阈值（0–100） |
| `TB_VAD_SILENCE_MS` | | `1100` | 静音多久结束本轮 |
| `TB_VAD_MIN_SPEECH_MS` | | `400` | 最短说话时长才允许因静音结束 |
| `TB_VAD_REARM_DELAY_MS` | | `250` | 播完后再听的冷却 |
| `TB_VAD_BARGE_THRESHOLD` | | `40` | 播报中打断峰值阈值 |
| `TB_VAD_BARGE_CHUNKS` | | `3` | 连续超阈 chunk 数才打断（约 96ms） |
| `TB_SPK_GAIN_Q8` | | `220` | 喇叭数字音量（Q8；256=1.0）；GAIN=6dB 偏小时可调高 |

`config.h` 在 `.gitignore` 里，不会提交。

## 云端对接

完整规范：

- 人类可读：[../tiny-bot-cloud-agent/docs/human-docs/02-firmware-integration.md](../tiny-bot-cloud-agent/docs/human-docs/02-firmware-integration.md)
- 机器可读：[../tiny-bot-cloud-agent/docs/firmware-integration/](../tiny-bot-cloud-agent/docs/firmware-integration/)

## 已知限制

- **打断**：Thinking / 播报可用能量或 BOOT 打断；喇叭回灌误触发时调高 `TB_VAD_BARGE_THRESHOLD`（无 AEC）
- **VAD**：板端峰值能量检测，嘈杂环境可能需调高 `TB_VAD_SPEECH_THRESHOLD`
- **听感**：数字音量与空闲停 I2S 时钟已实现；底噪/响度需上板确认
- **OLED**：表情脸 + 底栏状态；对话正文只在串口
- **多设备**：NVS key 写死 `tinybot/token`，单设备够用
- **WS 鉴权失败**：会自动清 NVS 里的 token，等下次重启重新 `provision`

## 调试技巧

```bash
# 串口监视
pio device monitor

# 抓 WebSocket 流量
wscat -c ws://192.168.1.100:5678/ws

# 强制重新配网（清 NVS）
pio run --target erase  # 或用 esptool.py erase_flash
```
