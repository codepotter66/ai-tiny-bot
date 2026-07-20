# ai-tiny-bot

免提陪伴机器人：ESP32 通过 WebSocket 与 Go 云端 Agent 通信（原始 PCM 16 kHz 单声道）。直接说话即可——板端能量 VAD，云端跑 ASR → LLM → TTS。

[English](README.md)

## 仓库结构

| 路径 | 作用 |
|------|------|
| [`tiny-bot-firmware/`](tiny-bot-firmware/) | ESP32 固件（PlatformIO）：Wi‑Fi、配网、WebSocket、OLED 表情、I2S 麦/喇叭 |
| [`tiny-bot-cloud-agent/`](tiny-bot-cloud-agent/) | 云端 Agent（Go）：WebSocket 协议、人设/记忆/技能、ASR/LLM/TTS |
| [`docs/`](docs/) | 硬件设计、搭建指南、到货检查等 |

## 整体链路

```
ESP32  --POST /provision-->  cloud-agent  （配对码 → token）
ESP32  <--WebSocket /ws---->  cloud-agent  （上行 PCM / 下行 STT+文本+PCM）
```

1. 在服务器上 `seed` 登记设备（`device_id` + `pairing_code`）。
2. 烧录固件，ID / Wi‑Fi / 云端地址与 seed 一致。
3. 上电 → provision → 说话；OLED 显示表情状态，对话文本走串口。

## 快速开始

### 云端 Agent（本地）

```bash
cd tiny-bot-cloud-agent
cp .env.example .env   # 填密钥；切勿提交 .env
make build-all
make seed              # 部署后可用 make seed-remote
make run
curl http://localhost:5678/healthz
```

部署（`.env` 中必须设置 `DEPLOY_SERVER` 与 `DEPLOY_SERVER_DIR`）：

```bash
cd tiny-bot-cloud-agent
make deploy
```

详见：[tiny-bot-cloud-agent/README.zh-CN.md](tiny-bot-cloud-agent/README.zh-CN.md)（[English](tiny-bot-cloud-agent/README.md)）· [deploy/README.zh-CN.md](tiny-bot-cloud-agent/deploy/README.zh-CN.md)

### 固件

```bash
cd tiny-bot-firmware
cp include/config.h.example include/config.h   # 私有配置，已 gitignore
# 编辑 Wi-Fi、HTTP_BASE / WS_HOST、TB_DEVICE_ID、TB_PAIRING_CODE
make main
make monitor
```

详见：[tiny-bot-firmware/README.zh-CN.md](tiny-bot-firmware/README.zh-CN.md)（[English](tiny-bot-firmware/README.md)）

### 浏览器 Demo

Agent 启动后访问：`http://localhost:5678/demo/`  
手机需 HTTPS（自签或隧道）——见 [demo/README.md](tiny-bot-cloud-agent/demo/README.md)。

## 密钥与本地配置

请勿提交：

- `tiny-bot-cloud-agent/.env` — API Key、`DEPLOY_SERVER`、`DEPLOY_SERVER_DIR`、`TB_PUBLIC_IP`
- `tiny-bot-firmware/include/config.h` — Wi‑Fi 与服务器地址

模板：`.env.example`、`include/config.h.example`。根目录 [`.gitignore`](.gitignore) 已忽略环境文件、构建产物与压缩包。

## 文档

- 搭建指南：[docs/step-by-step-build-guide.md](docs/step-by-step-build-guide.md)
- 硬件设计：[docs/hardware-design.md](docs/hardware-design.md)
- 云端架构：[tiny-bot-cloud-agent/docs/human-docs/00-architecture.md](tiny-bot-cloud-agent/docs/human-docs/00-architecture.md)
- 固件对接：[tiny-bot-cloud-agent/docs/human-docs/02-firmware-integration.md](tiny-bot-cloud-agent/docs/human-docs/02-firmware-integration.md)

## 许可

以各子项目声明为准；未声明则保留作者权利。
