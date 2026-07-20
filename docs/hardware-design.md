# TinyBot — 低成本陪伴机器人硬件方案

> 预算：¥60-80 | 零焊接 | 全部淘宝现成模块 | 拳头大小

---

## 目录

1. [总体架构](#1-总体架构)
2. [BOM 清单与淘宝选购指南](#2-bom-清单与淘宝选购指南)
3. [硬件接线图](#3-硬件接线图)
4. [软件架构](#4-软件架构)
5. [固件端代码资料与库](#5-固件端代码资料与库)
6. [固件端关键代码骨架](#6-固件端关键代码骨架)
7. [云端 Go 后端骨架](#7-云端-go-后端骨架)
8. [组装流程](#8-组装流程)

---

## 1. 总体架构

```
┌─────────────────────────────────────────────────────┐
│                   TinyBot 物理层                       │
│                                                       │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐          │
│  │ INMP441  │   │ESP32     │   │ MAX98357 │          │
│  │ I2S 麦克风│◄──┤ DevKit V1├──►│ I2S 功放  │── 喇叭   │
│  └──────────┘   │          │   └──────────┘          │
│                  │ WiFi+BLE │                          │
│  ┌──────────┐   │ 双核240M │   ┌──────────┐          │
│  │ ST7789   │◄──┤          ├──►│ 舵机(可选)│          │
│  │ 1.3" TFT │   └──────────┘   └──────────┘          │
│  │ 表情+文字 │                                         │
│  └──────────┘   ┌──────────┐                          │
│                  │ TP4056   │                          │
│                  │ 锂电池充电│                          │
│                  └────┬─────┘                          │
│                  ┌────┴─────┐                          │
│                  │ 500mAh   │                          │
│                  │ 锂电池    │                          │
│                  └──────────┘                          │
└──────────────────────┬──────────────────────────────┘
                       │ WiFi WebSocket
                       ▼
┌─────────────────────────────────────────────────────┐
│               云端 Go 后端（你的服务器）                 │
│                                                       │
│  WebSocket  ◄──►  Whisper STT  ◄──►  LLM 对话         │
│  Server                                   │           │
│                                            ▼           │
│                                    Edge TTS / 火山引擎  │
│                                            │           │
│                                            ▼           │
│                          WebSocket ──► 推回音频块        │
└─────────────────────────────────────────────────────┘
```

**设计原则**：ESP32 本地只做三件事——收音、画表情、播放 TTS。所有 AI 重活（语音识别、LLM 推理、语音合成）全部在云端完成。这样 ESP32 的性能足够，不需要 S3/P4 等高端芯片。

---

## 2. BOM 清单与淘宝选购指南

### 2.1 必须项（零焊接全套 ¥56-81）

| 序号 | 部件 | 型号 | 数量 | 约价(¥) | 淘宝搜索关键词 | 选型说明 |
|:---:|------|------|:---:|--------|-------------|---------|
| 1 | 主控板 | ESP32-DevKit V1 CH340C Type-C | 1 | 12-16 | `ESP32 开发板 CH340C Type-C` | 买 Type-C 口版本，Micro-USB 已淘汰；确认带排针（已焊好），否则需要自己焊 |
| 2 | 麦克风 | INMP441 I2S MEMS | 1 | 5-8 | `INMP441 麦克风模块 I2S` | 选带 PCB 底板+排针的模块版，顶部有个小黑孔就是麦克风元件 |
| 3 | 功放 | MAX98357 I2S D 类放大器 | 1 | 4-6 | `MAX98357 I2S 功放模块` | 注意模块上芯片丝印为 MAX98357A；买带排针版本 |
| 4 | 喇叭 | 3W 4Ω 小喇叭（40mm）| 1 | 3-5 | `4R3W 喇叭 40mm` | 内磁型体积更小；40mm 直径对拳头大小合适，声音足够对话用 |
| 5 | 屏幕 | 1.3" ST7789 TFT 240×240 IPS | 1 | 10-15 | `1.3寸 ST7789 240x240 SPI 模块` | 买"模块"不要买"裸屏"——模块带 PCB 转接板和排针，插面包板即用；IPS 视角好 |
| 6 | 充电板 | TP4056 Type-C 锂电池充电 | 1 | 1.5-3 | `TP4056 Type-C 充电模块` | 买 Type-C 口，自带过充/过放保护 |
| 7 | 电池 | 503040 聚合物锂电池 500mAh | 1 | 6-10 | `503040 聚合物锂电池 500mAh` | 503040 尺寸 50×30×4mm 很薄；确认带 JST 1.25mm 2P 插头 |
| 8 | 面包板 | 400 孔面包板 | 1 | 4-6 | `400孔面包板` | 中间带凹槽的标准款 |
| 9 | 杜邦线 | 公母+母母+公公 各 10 根 | 1 套 | 5-8 | `杜邦线套装 三种各10根` | 20cm 长度合适，太短不好走线 |
| 10 | USB 线 | USB-A to Type-C 数据线 | 1 | 3-5 | 随便买 | 用于刷固件和调试 |

### 2.2 可选项（按需追加）

| 序号 | 部件 | 型号 | 数量 | 约价(¥) | 淘宝搜索关键词 | 说明 |
|:---:|------|------|:---:|--------|-------------|------|
| 11 | 舵机 | SG90 微型舵机 | 1-2 | 4-8 | `SG90 舵机` | 点头/摇头用，接 GPIO 即可 |
| 12 | 测距 | VL53L0X TOF 激光测距 | 1 | 10-15 | `VL53L0X 激光测距模块` | 避障用，I2C 接口，精度比超声波高 |
| 13 | 外壳 | 3D 打印外壳 | 1 | 10-20 | 淘宝搜 `3D打印代打` | 自己画 STL 或找现成的 ESP32 robot enclosure 模型 |

### 2.3 低配版 BOM（¥45-60）

如果预算有限，屏幕换成 SSD1306 0.96" OLED 蓝/白单色屏（¥8-12），麦克风选 ¥3-4 的兼容模块。其余不变。总价可压到 ¥45-60。

> 搜索关键词：`SSD1306 0.96寸 OLED I2C 模块`。注意买 I2C 接口的（4 针），不要买 SPI 的（7 针），I2C 接线简单。

### 2.4 购买渠道对比

| 渠道 | 优势 | 劣势 | 推荐度 |
|------|------|------|:---:|
| 淘宝 | 品类全、价格低、退换方便 | 品质参差 | ★★★★★ |
| 拼多多 | 部分品类更便宜 | 电子元件没有淘宝全 | ★★★ |
| 闲鱼 | 二手更便宜 | 品质无保障、无退换 | 不推荐新手 |
| AliExpress | 面向海外 | 国内没必要 | ★ |

### 2.5 避免踩坑

- **不要买 Micro-USB 口的 ESP32**：2026 年还在用 Micro-USB 的通常是老货/库存，优先选 Type-C
- **不要买英文版 ESP32 DevKitC**：40-50 元，跟 12 元的功能完全一样
- **屏幕确认是模块不是裸屏**：模块带转接 PCB + 排针，裸屏是 FPC 排线需要焊接转接板
- **INMP441 确认版本**：淘宝上 ¥3-4 的可能是兼容替代品（MS3625 等），功能和引脚兼容 INMP441，差别不大
- **电池确认带保护板**：聚合物锂电池必须带保护板（过充/过放/短路），绝大部分成品电芯默认带

---

## 3. 硬件接线图

### 3.1 面包板布局（俯视图）

```
     ┌────────────────────────────────────┐
     │           面包板 400孔               │
     │                                     │
     │  + ─── 3.3V 正极轨 (红线)           │
     │  - ─── GND  负极轨 (蓝线)           │
     │                                     │
     │  ┌───────────────────────┐          │
     │  │   ESP32 DevKit V1     │ (跨凹槽) │
     │  │   USB口朝外           │          │
     │  │                       │          │
     │  │ GPIO21 ██─────────────┼── SDA ───┤──► ST7789 SDA / SCL
     │  │ GPIO22 ██─────────────┼── SCL ───┤
     │  │ GPIO18 ██─────────────┼── SCK ───┤──► ST7789 SCL (SPI CLK)
     │  │ GPIO23 ██─────────────┼── MOSI ──┤──► ST7789 SDA (SPI MOSI)
     │  │ GPIO2  ██─────────────┼── DC  ───┤──► ST7789 DC
     │  │ GPIO4  ██─────────────┼── RST ───┤──► ST7789 RST
     │  │                       │          │
     │  │ GPIO32 ██─────────────┼── SD  ───┤──► INMP441 SD
     │  │ GPIO33 ██─────────────┼── WS  ───┤──► INMP441 WS
     │  │ GPIO25 ██─────────────┼── SCK ───┤──► INMP441 SCK
     │  │                       │          │
     │  │ GPIO26 ██─────────────┼── BCLK ──┤──► MAX98357 BCLK
     │  │ GPIO27 ██─────────────┼── LRCLK ─┤──► MAX98357 LRCLK
     │  │ GPIO14 ██─────────────┼── DIN  ──┤──► MAX98357 DIN
     │  │                       │          │
     │  │ VIN  ██───────────────┼── 5V ────┤──► 电池正极
     │  │ 3.3V ██───────────────┼── 3.3V ──┤──► 正极轨
     │  │ GND  ██───────────────┼── GND ───┤──► 负极轨
     │  └───────────────────────┘          │
     │                                     │
     │  ┌─ST7789────┐  ┌─INMP441──┐       │
     │  │ 240x240   │  │ I2S MEMS │       │
     │  └───────────┘  └──────────┘       │
     │  ┌─MAX98357──┐  ┌─TP4056───┐       │
     │  │ I2S Amp   │  │ 充电板   │       │
     │  │ ┌─SPK OUT │  │ [USB-C]  │       │
     │  │ │ ->喇叭  │  │ BAT→电池 │       │
     │  └─┴─────────┘  └──────────┘       │
     └────────────────────────────────────┘
```

### 3.2 引脚速查表

```
ESP32 引脚       目标模块         目标引脚         说明
──────────────────────────────────────────────────────
3.3V            所有模块的 VCC    VCC             共阳极轨
GND             所有模块的 GND    GND             共阴极轨
VIN (5V)        MAX98357 VIN     VIN             音频功放需要高于3.3V
                面包板 5V 轨     5V              经板载 LDO 转 3.3V

# I2C 总线 (ST7789 也可以用 SPI，此处 I2C 保留给以后加传感器)
GPIO21          ST7789(SPI 共用)  —               I2C SDA（预留）
GPIO22          ST7789(SPI 共用)  —               I2C SCL（预留）

# SPI 总线 (ST7789 屏幕)
GPIO18          ST7789           SCL (SCK)        SPI 时钟
GPIO23          ST7789           SDA (MOSI)       SPI 数据输出
GPIO2           ST7789           DC               Data/Command
GPIO4           ST7789           RST              Reset（可不接，设 -1）

# I2S 麦克风
GPIO32          INMP441          SD               I2S 数据输入
GPIO33          INMP441          WS (LRC)         左右声道选择
GPIO25          INMP441          SCK              I2S 时钟
GND             INMP441          L/R              L/R 接地 = 左声道

# I2S 功放
GPIO26          MAX98357         BCLK             I2S 位时钟
GPIO27          MAX98357         LRCLK (WS)       I2S 左右声道时钟
GPIO14          MAX98357         DIN              I2S 数据输出
MAX98357 SPK+   喇叭             正极             接喇叭正极
MAX98357 SPK-   喇叭             负极             接喇叭负极

# 充电
TP4056 BAT+     电池             正极 (红线)      确认极性！
TP4056 BAT-     电池             负极 (黑线)      确认极性！
TP4056 OUT+     面包板 5V 轨                     用于给 ESP32 供电
TP4056 OUT-     面包板 GND 轨
```

### 3.3 实物接线检查清单

- [ ] INMP441 的 L/R 脚接 GND（选左声道）
- [ ] MAX98357 的 GAIN 脚悬空（默认 9dB），SD 脚接 VIN（拉高使能）
- [ ] ST7789 的 BLK（背光）接 3.3V（常亮），或接 GPIO 做 PWM 调光
- [ ] ST7789 的 CS 脚不接，代码设 -1（单 SPI 设备不需要片选）
- [ ] 电池红线接 TP4056 BAT+，黑线接 BAT-，不要接反（接反 TP4056 会烧）
- [ ] TP4056 先插 USB 充电测试，观察 LED 状态（红=充电中，蓝/绿=充满）
- [ ] 所有模块 VCC 从面包板正极轨取电，不要从 ESP32 3.3V 引脚直接取（ESP32 3.3V LDO 输出只有 500-600mA）

---

## 4. 软件架构

### 4.1 系统分层

```
┌──────────────┐  ┌──────────────────────────────────┐
│   ESP32 固件  │  │         Go 云端服务              │
│              │  │                                  │
│ ┌──────────┐ │  │  ┌────────────────────────────┐  │
│ │表情渲染   │ │  │  │ WebSocket Hub              │  │
│ │TFT_eSPI  │ │  │  │ (多设备连接管理)             │  │
│ └──────────┘ │  │  └──────────┬─────────────────┘  │
│              │  │             │                    │
│ ┌──────────┐ │  │  ┌──────────▼─────────────────┐  │
│ │音频采集   │ │  │  │ STT Pipeline               │  │
│ │I2S→PCM   │─┼──┼─►│ Whisper API / 本地 Whisper  │  │
│ │→WebSocket│ │  │  └──────────┬─────────────────┘  │
│ └──────────┘ │  │             │                    │
│              │  │  ┌──────────▼─────────────────┐  │
│ ┌──────────┐ │  │  │ LLM 对话                    │  │
│ │音频播放   │◄┼──┼──│ Claude / OpenAI / Qwen     │  │
│ │WebSocket │ │  │  └──────────┬─────────────────┘  │
│ │→I2S播放  │ │  │             │                    │
│ └──────────┘ │  │  ┌──────────▼─────────────────┐  │
│              │  │  │ TTS Pipeline                │  │
│ ┌──────────┐ │  │  │ Edge TTS / 火山引擎 / 讯飞  │  │
│ │WiFi管理   │ │  │  └──────────┬─────────────────┘  │
│ │自动重连   │ │  │             │                    │
│ └──────────┘ │  │  ┌──────────▼─────────────────┐  │
│              │  │  │ 对话状态管理                  │  │
│ ┌──────────┐ │  │  │ (system prompt/上下文)       │  │
│ │主状态机   │ │  │  └────────────────────────────┘  │
│ └──────────┘ │  │                                  │
└──────────────┘  └──────────────────────────────────┘
```

### 4.2 ESP32 固件状态机

```
                       ┌─────────┐
         上电 ────────►│  启动   │
                       └────┬────┘
                            │ WiFi 连接
                       ┌────▼────┐
             连接失败  │ 待机中   │◄──────────────┐
            (重试3秒)  │ (眨眼)   │               │
                       └────┬────┘               │
                            │ WebSocket 连上       │
                       ┌────▼────┐               │
             监听中无音 │  空闲   │               │
            (随机表情)  │        │               │
                       └────┬────┘               │
                            │ 检测到声音(VAD)      │
                       ┌────▼────┐               │
                       │ 聆听中   │               │
                       │ (耳朵动画)│              │
                       └────┬────┘               │
                            │ 音频发往云端          │
                       ┌────▼────┐               │
                       │ 思考中   │               │
                       │ (转圈动画)│              │
                       └────┬────┘               │
                            │ LLM 返回文字          │
                       ┌────▼────┐               │
                       │ 说话中   │               │
                       │ (嘴张合动画)│             │
                       └────┬────┘               │
                            │ TTS 播放完成         │
                            └────────────────────┘
```

### 4.3 音频数据流

```
INMP441 (24-bit I2S)
    │
    ▼
ESP32 I2S 驱动 (读 32-bit，右移 16 位取高 16-bit → 16-bit PCM)
    │
    ▼
环形缓冲区 (Ring Buffer, 4096 samples / frame)
    │
    ▼
VAD (简单能量阈值检测 → 判断是否在说话)
    │
    ▼ 检测到语音段
分帧发送 WebSocket BIN (4096 bytes / msg)
    │
    ▼
──────────── WiFi ────────────
    │
    ▼
Go WebSocket Server
    │
    ├─► [收集音频帧 → 拼成 WAV] → Whisper API → text
    ├─► [text + system prompt + 历史] → LLM API → response
    └─► [response text] → TTS API → MP3/AAC bytes
                                       │
    ┌──────────────────────────────────┘
    ▼
WebSocket BIN → ESP32 环形接收缓冲
    │
    ▼
I2S 输出 → MAX98357 → 喇叭
```

### 4.4 FreeRTOS 任务分配（ESP32 双核）

```
Core 0 (Protocol Core - 协议核):
  ├── WiFi 保活 (优先)
  ├── WebSocket 收发
  ├── 音频编码/解码 (可选 Opus 压缩)
  └── HTTP OTA 升级

Core 1 (Application Core - 应用核):
  ├── I2S 音频采集 (DMA, 低 CPU 占用)
  ├── I2S 音频播放 (DMA, 低 CPU 占用)
  ├── TFT 屏幕刷新 (SPI DMA)
  ├── VAD 检测
  └── 舵机控制
```

---

## 5. 固件端代码资料与库

### 5.1 开发环境

| 工具 | 用途 | 下载 |
|------|------|------|
| Arduino IDE 2.x | 固件编写和烧录 | [arduino.cc](https://www.arduino.cc/en/software) |
| PlatformIO (推荐) | VS Code 插件，比 Arduino IDE 强大 | VS Code 插件市场搜 `PlatformIO` |
| ESP32 官方工具链 | 命令行开发 | 搜索 `espressif/arduino-esp32` |

推荐用 PlatformIO + VS Code。Arduino IDE 的库管理和自动补全不如 PlatformIO。

### 5.2 核心库清单

```
platformio.ini 中的依赖：

[env:esp32dev]
platform = espressif32
board = esp32dev
framework = arduino
monitor_speed = 115200

lib_deps =
    ; 音频流处理（录音+播放+I2S+格式转换一体化）
    pschatzmann/arduino-audio-tools @ ^1.0
    
    ; WebSocket 客户端
    links2004/WebSockets @ ^2.4
    
    ; ST7789 屏幕驱动
    bodmer/TFT_eSPI @ ^2.5
    
    ; JSON 解析（云端协议用）
    bblanchon/ArduinoJson @ ^7.0
    
    ; WiFi 管理（简化 WiFi 连接）
    tzapu/WiFiManager @ ^2.0
```

### 5.3 各库参考资料

| 库 | 文档/GitHub | 关键内容 |
|---|-----------|---------|
| **arduino-audio-tools** | [github.com/pschatzmann/arduino-audio-tools](https://github.com/pschatzmann/arduino-audio-tools) | I2S 配置、格式转换、Stream 抽象层、WebSocket 音频流示例 |
| **WebSockets** | [github.com/Links2004/arduinoWebSockets](https://github.com/Links2004/arduinoWebSockets) | WebSocket client/server API、ping/pong 心跳 |
| **TFT_eSPI** | [github.com/Bodmer/TFT_eSPI](https://github.com/Bodmer/TFT_eSPI) | User_Setup.h 配置、绘图 API、Sprite（离屏缓冲）|
| **ArduinoJson** | [arduinojson.org](https://arduinojson.org/) | JSON 序列化/反序列化、内存用量计算器 |
| **WiFiManager** | [github.com/tzapu/WiFiManager](https://github.com/tzapu/WiFiManager) | 首次配网 AP 模式、WiFi 自动重连 |

### 5.4 ESP32 I2S 关键参数（INMP441）

```
参数                  值              说明
────────────────────────────────────────────
采样率                16000 Hz        语音识别标准采样率
位宽                  16 bit          24-bit 数据取高 16 位
声道                  单声道          INMP441 L/R 接地
I2S 总线频率          2.048 MHz       16000 × 32 × 2 × 2 (stereo)
DMA 缓冲区            8 buffers       4096 bytes/buffer
数据格式              I2S_PHILIPS_MODE 标准 I2S 格式
```

### 5.5 ESP32 I2S 关键参数（MAX98357 播放）

```
参数                  值              说明
────────────────────────────────────────────
采样率                22050/24000 Hz  TTS 常用采样率
位宽                  16 bit
声道                  单声道          可升混为双声道
功放增益              9dB (默认)      GAIN 脚悬空
```

### 5.6 ST7789 TFT_eSPI 配置要点

PlatformIO 中 TFT_eSPI 需要手动配置 `User_Setup.h`。两种方式：

**方式 A（推荐）**：在 `platformio.ini` 中指定自定义配置路径：

```ini
build_flags =
    -DUSER_SETUP_LOADED=1
    -include"include/User_Setup.h"
```

然后在项目 `include/User_Setup.h` 写入自己的配置。

**方式 B**：直接改库文件（不推荐，库更新会覆盖）。

### 5.7 音频压缩（可选优化）

原始 PCM 16-bit 16kHz 单声道 = 32KB/s = 256Kbps。如果网络不稳定，加 Opus 压缩：

```
库: pschatzmann/arduino-libopus
压缩后: ~16Kbps (20:1 压缩比)
CPU 开销: ESP32 双核轻松跑
```

不加压缩的话 256Kbps WiFi 也完全够用，手机热点都无压力。初学者建议先不加 Opus，走原始 PCM，调通后再优化。

---

## 6. 固件端关键代码骨架

### 6.1 主程序结构（main.cpp）

```cpp
#include <Arduino.h>
#include <WiFi.h>
#include <WebSocketsClient.h>
#include <AudioTools.h>
#include <TFT_eSPI.h>
#include <ArduinoJson.h>

// ============ 引脚定义 ============
// I2S 麦克风
#define I2S_MIC_SD    32
#define I2S_MIC_WS    33
#define I2S_MIC_SCK   25

// I2S 喇叭
#define I2S_SPK_BCLK  26
#define I2S_SPK_LRCLK 27
#define I2S_SPK_DIN   14

// TFT (SPI)
#define TFT_SCLK  18
#define TFT_MOSI  23
#define TFT_DC     2
#define TFT_RST    4

// ============ 全局对象 ============
TFT_eSPI tft;
WebSocketsClient ws;
I2SStream i2sIn;   // 麦克风输入
I2SStream i2sOut;  // 喇叭输出

// 环形缓冲区
RingBuffer<uint8_t> audioRingBuffer(16384);  // 16KB

// 状态机
enum State { IDLE, LISTENING, THINKING, SPEAKING };
State currentState = IDLE;

// WiFi 配置
const char* WIFI_SSID = "YOUR_SSID";
const char* WIFI_PASS = "YOUR_PASSWORD";
const char* WS_HOST   = "192.168.x.x";  // 你 Go 服务的地址
const uint16_t WS_PORT = 8080;

// ============ 任务原型 ============
void taskAudioCapture(void* param);
void taskAudioPlayback(void* param);
void taskDisplayUpdate(void* param);
void taskWebSocket(void* param);
void taskStateMachine(void* param);

// ============ 表情绘制 ============
void drawIdleFace() {
    // 画眨眼的圆形脸
    tft.fillScreen(TFT_BLACK);
    tft.fillCircle(120, 100, 60, TFT_WHITE);  // 脸
    tft.fillCircle(95, 90, 8, TFT_BLACK);     // 左眼
    tft.fillCircle(145, 90, 8, TFT_BLACK);    // 右眼
    // 嘴（弧线）
    tft.drawArc(120, 110, 20, 15, 0, 180, TFT_BLACK, 3);
}

void drawListeningFace() {
    // 大耳朵/聆听
    tft.fillScreen(TFT_BLACK);
    tft.fillCircle(120, 100, 60, TFT_CYAN);
    tft.fillCircle(90, 85, 12, TFT_WHITE);    // 左眼白
    tft.fillCircle(150, 85, 12, TFT_WHITE);   // 右眼白
    tft.fillCircle(93, 82, 6, TFT_BLACK);     // 左瞳孔(大、注意)
    tft.fillCircle(153, 82, 6, TFT_BLACK);    // 右瞳孔
}

void drawThinkingFace() {
    // 旋转的小点表示思考
    static int dotAngle = 0;
    tft.fillScreen(TFT_BLACK);
    tft.fillCircle(120, 100, 60, TFT_YELLOW);
    tft.fillCircle(100, 95, 6, TFT_BLACK);    // 左眼
    tft.fillCircle(140, 95, 6, TFT_BLACK);    // 右眼
    // 旋转的小圆点
    float rad = dotAngle * PI / 180.0;
    int dx = 20 * cos(rad);
    int dy = 20 * sin(rad);
    tft.fillCircle(120 + dx, 115 + dy, 5, TFT_BLACK);
    dotAngle = (dotAngle + 15) % 360;
}

void drawSpeakingFace(int amplitude) {
    // 根据音量大小改变嘴的大小
    int mouthSize = map(amplitude, 0, 100, 5, 30);
    tft.fillScreen(TFT_BLACK);
    tft.fillCircle(120, 100, 60, TFT_GREEN);
    tft.fillCircle(95, 90, 8, TFT_BLACK);     // 左眼
    tft.fillCircle(145, 90, 8, TFT_BLACK);    // 右眼
    tft.fillCircle(120, 115, mouthSize, TFT_BLACK);  // 嘴
}

// ============ 音频采集任务 ============
void taskAudioCapture(void* param) {
    // I2S 配置：16kHz, 16bit, 单声道
    I2SConfig i2sCfg = i2sIn.defaultConfig(RX_MODE);
    i2sCfg.i2s_format = I2S_STD_FORMAT;
    i2sCfg.sample_rate = 16000;
    i2sCfg.bits_per_sample = 16;
    i2sCfg.channels = 1;
    i2sCfg.pin_data = I2S_MIC_SD;
    i2sCfg.pin_ws = I2S_MIC_WS;
    i2sCfg.pin_bck = I2S_MIC_SCK;
    i2sIn.begin(i2sCfg);

    uint8_t buffer[512];
    while (true) {
        size_t bytesRead = i2sIn.readBytes(buffer, sizeof(buffer));
        if (bytesRead > 0) {
            audioRingBuffer.writeArray(buffer, bytesRead);
        }
        vTaskDelay(1);
    }
}

// ============ WebSocket 事件处理 ============
void webSocketEvent(WStype_t type, uint8_t* payload, size_t length) {
    switch (type) {
        case WStype_CONNECTED:
            Serial.println("[WS] Connected");
            break;
        case WStype_DISCONNECTED:
            Serial.println("[WS] Disconnected, retry in 3s...");
            break;
        case WStype_BIN:
            // 收到云端 TTS 音频 → 写入 I2S 播放缓冲
            i2sOut.write(payload, length);
            break;
        case WStype_TEXT: {
            // 收到云端控制指令（JSON）
            StaticJsonDocument<256> doc;
            deserializeJson(doc, payload);
            const char* cmd = doc["cmd"];
            if (strcmp(cmd, "state") == 0) {
                const char* newState = doc["state"];
                if (strcmp(newState, "thinking") == 0) currentState = THINKING;
                else if (strcmp(newState, "speaking") == 0) currentState = SPEAKING;
            }
            break;
        }
        case WStype_ERROR:
            Serial.println("[WS] Error");
            break;
    }
}

// ============ 音频上传任务 ============
void taskAudioUpload(void* param) {
    uint8_t buffer[1024];
    while (true) {
        if (currentState == LISTENING && ws.isConnected()) {
            size_t avail = audioRingBuffer.available();
            if (avail >= sizeof(buffer)) {
                audioRingBuffer.readArray(buffer, sizeof(buffer));
                ws.sendBIN(buffer, sizeof(buffer));
            }
        }
        vTaskDelay(5);
    }
}

// ============ 显示更新任务 ============
void taskDisplayUpdate(void* param) {
    while (true) {
        switch (currentState) {
            case IDLE:     drawIdleFace(); break;
            case LISTENING: drawListeningFace(); break;
            case THINKING:  drawThinkingFace(); break;
            case SPEAKING:  drawSpeakingFace(50); break;
        }
        vTaskDelay(50); // 20fps
    }
}

// ============ VAD 检测 ============
bool detectVoiceActivity() {
    static unsigned long lastVoiceTime = 0;
    const int SILENCE_THRESHOLD = 500;  // 需要根据实际环境调
    const int MIN_VOICE_MS = 300;       // 最短有效语音长度
    const int SILENCE_TIMEOUT_MS = 800; // 说话停顿多久认为结束

    uint8_t buf[256];
    size_t avail = audioRingBuffer.available();
    if (avail < sizeof(buf)) return false;

    audioRingBuffer.peekArray(buf, sizeof(buf));
    int16_t* samples = (int16_t*)buf;
    int sum = 0;
    for (int i = 0; i < sizeof(buf)/2; i++) {
        sum += abs(samples[i]);
    }
    int avg = sum / (sizeof(buf)/2);

    unsigned long now = millis();
    if (avg > SILENCE_THRESHOLD) {
        lastVoiceTime = now;
        return true;
    }
    return (now - lastVoiceTime < SILENCE_TIMEOUT_MS);
}

// ============ 状态机任务 ============
void taskStateMachine(void* param) {
    while (true) {
        switch (currentState) {
            case IDLE:
                if (detectVoiceActivity()) {
                    currentState = LISTENING;
                    // 通知云端开始接收
                    ws.sendTXT("{\"cmd\":\"start_listening\"}");
                }
                break;
            case LISTENING:
                if (!detectVoiceActivity()) {
                    currentState = THINKING;
                    ws.sendTXT("{\"cmd\":\"stop_listening\"}");
                }
                break;
            case SPEAKING:
                // 云端发 state: idle 时切回 IDLE
                break;
            default:
                break;
        }
        vTaskDelay(50);
    }
}

// ============ Setup ============
void setup() {
    Serial.begin(115200);

    // TFT 初始化
    tft.init();
    tft.setRotation(0);
    tft.fillScreen(TFT_BLACK);
    tft.drawString("TinyBot v0.1", 50, 110, 2);
    delay(1000);

    // WiFi 连接
    WiFi.begin(WIFI_SSID, WIFI_PASS);
    while (WiFi.status() != WL_CONNECTED) {
        delay(500);
        tft.fillScreen(TFT_BLACK);
        tft.drawString("WiFi...", 80, 110, 2);
    }
    tft.drawString("WiFi OK!", 80, 110, 2);

    // WebSocket 连接
    ws.begin(WS_HOST, WS_PORT, "/ws");
    ws.onEvent(webSocketEvent);
    ws.setReconnectInterval(3000);

    // I2S 喇叭输出配置
    I2SConfig spkCfg = i2sOut.defaultConfig(TX_MODE);
    spkCfg.sample_rate = 24000;
    spkCfg.bits_per_sample = 16;
    spkCfg.channels = 1;
    spkCfg.pin_data = I2S_SPK_DIN;
    spkCfg.pin_ws = I2S_SPK_LRCLK;
    spkCfg.pin_bck = I2S_SPK_BCLK;
    i2sOut.begin(spkCfg);

    // 创建 FreeRTOS 任务
    xTaskCreatePinnedToCore(taskAudioCapture,  "AudioIn",  8192, NULL, 2, NULL, 1);
    xTaskCreatePinnedToCore(taskAudioUpload,   "AudioUp",  4096, NULL, 1, NULL, 0);
    xTaskCreatePinnedToCore(taskDisplayUpdate, "Display",  4096, NULL, 1, NULL, 1);
    xTaskCreatePinnedToCore(taskStateMachine,  "State",    2048, NULL, 1, NULL, 1);

    currentState = IDLE;
}

void loop() {
    ws.loop();
    delay(10);
}
```

### 6.2 配置文件（include/User_Setup.h）

```cpp
// TFT_eSPI 配置 - ESP32 + ST7789 240x240
#define USER_SETUP_INFO "TinyBot_ST7789_240x240"

#define ST7789_DRIVER
#define TFT_RGB_ORDER TFT_RGB
#define TFT_WIDTH  240
#define TFT_HEIGHT 240

// ESP32 VSPI 引脚
#define TFT_MOSI  23
#define TFT_MISO  19
#define TFT_SCLK  18
#define TFT_CS    -1     // 不接
#define TFT_DC     2
#define TFT_RST    4

// 字号加载
#define LOAD_GLCD
#define LOAD_FONT2
#define LOAD_FONT4
#define LOAD_FONT6
#define LOAD_GFXFF
#define SMOOTH_FONT

#define SPI_FREQUENCY  40000000
```

### 6.3 平台配置（platformio.ini）

```ini
[env:esp32dev]
platform = espressif32
board = esp32dev
framework = arduino
monitor_speed = 115200
upload_speed = 921600

board_build.flash_mode = dio
board_build.f_flash = 40000000L

lib_deps =
    pschatzmann/arduino-audio-tools @ ^1.0.0
    links2004/WebSockets @ ^2.4.2
    bodmer/TFT_eSPI @ ^2.5.43
    bblanchon/ArduinoJson @ ^7.0.4
    tzapu/WiFiManager @ ^2.0.17

build_flags =
    -DCORE_DEBUG_LEVEL=0
    -DUSER_SETUP_LOADED=1
    -include"include/User_Setup.h"
```

---

## 7. 云端 Go 后端骨架

### 7.1 项目结构

```
cloud-backend/
├── main.go              # 入口，HTTP/WS 服务器
├── go.mod
├── config.yaml          # 配置文件
├── internal/
│   ├── ws/
│   │   └── hub.go       # WebSocket 连接管理
│   ├── stt/
│   │   └── whisper.go   # Whisper API 封装
│   ├── llm/
│   │   └── chat.go      # LLM 对话（Claude/OpenAI）
│   ├── tts/
│   │   └── edge.go      # TTS 合成（Edge-TTS/火山引擎）
│   └── session/
│       └── session.go   # 对话上下文管理
└── web/
    └── index.html       # (可选) 调试用网页
```

### 7.2 main.go 骨架

```go
package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
)

func main() {
	hub := ws.NewHub()
	go hub.Run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws.Serve(hub, w, r)
	})

	go func() {
		log.Println("TinyBot cloud starting on :8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutting down...")
}
```

### 7.3 WebSocket Hub

```go
package ws

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	ID   string
	Conn *websocket.Conn
	Send chan []byte
	Hub  *Hub

	// 对话会话
	Session *session.Session
}

type AudioMessage struct {
	Type    string `json:"type"`    // "audio" | "cmd"
	Cmd     string `json:"cmd"`     // "start_listening" | "stop_listening"
	Audio   []byte `json:"-"`       // binary audio data
}

type Hub struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			delete(h.clients, client.ID)
			h.mu.Unlock()
		}
	}
}
```

### 7.4 STT 模块

```go
package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type WhisperClient struct {
	APIKey  string
	BaseURL string // https://api.openai.com/v1
}

const SystemPrompt = `你是一个可爱的拳头大小陪伴机器人，名字叫"小圆"。
你的性格：温暖、幽默、偶尔调皮。
回答要求：
- 每次回复 1-3 句话，不要长篇大论
- 用口语化的中文
- 偶尔加入拟声词（嗯、哦、哈哈）
- 如果用户心情不好，先共情再给建议
- 可以适当反问，引导对话继续`

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

func (c *WhisperClient) Transcribe(ctx context.Context, audioData []byte) (string, error) {
	// 构建 multipart form (WAV 16kHz mono 16-bit)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "audio.wav")
	part.Write(audioData)
	writer.WriteField("model", "whisper-1")
	writer.WriteField("language", "zh")
	writer.Close()

	req, _ := http.NewRequestWithContext(ctx, "POST",
		c.BaseURL+"/audio/transcriptions", body)
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Text string `json:"text"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Text, nil
}
```

### 7.5 LLM 对话模块

```go
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

type ClaudeClient struct {
	APIKey  string
	BaseURL string // https://api.anthropic.com/v1
}

func (c *ClaudeClient) Chat(ctx context.Context, history []ChatMessage) (string, error) {
	// 注入 system prompt
	messages := append([]ChatMessage{
		{Role: "system", Content: stt.SystemPrompt},
	}, history...)

	body := map[string]interface{}{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 300,
		"messages":   messages,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequestWithContext(ctx, "POST",
		c.BaseURL+"/messages", bytes.NewReader(jsonBody))
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	// ... 解析返回
}
```

### 7.6 TTS 模块（Edge-TTS，免费）

```go
package tts

import (
	"context"
	"fmt"
	"net/http"
)

// Edge-TTS 是微软免费的 TTS 服务，通过 WebSocket 调用
// 不需要 API Key
type EdgeTTS struct {
	Voice   string // zh-CN-XiaoxiaoNeural (女声) 或 zh-CN-YunxiNeural (男声)
}

// Synthesize 返回 MP3 音频字节流
func (e *EdgeTTS) Synthesize(ctx context.Context, text string) ([]byte, error) {
	// 实现通过 WebSocket 连接 speech.platform.bing.com
	// 协议: wss://speech.platform.bing.com/consumer/speech/synthesize/readaloud/edge/v1
	// 参考: github.com/xxx/edge-tts (MIT License)
	//
	// 或者用 HTTP API 替代方案:
	// 火山引擎 TTS (有免费额度): https://www.volcengine.com/product/tts
	// 讯飞 TTS (有免费额度): https://www.xfyun.cn/services/online_tts
	//
	// 最简单的方式: HTTP GET
	// https://api.edge-tts.com/v1/tts?text=你好&voice=zh-CN-XiaoxiaoNeural
}
```

### 7.7 对话处理 Pipeline

```go
package pipeline

import (
	"context"
	"log"
)

type Pipeline struct {
	STT  *stt.WhisperClient
	LLM  *llm.ClaudeClient
	TTS  *tts.EdgeTTS
}

func (p *Pipeline) Process(ctx context.Context, client *ws.Client,
	audioChunks <-chan []byte, stateCh chan<- string) {

	// 1. 收集音频 → 转文字
	stateCh <- "thinking"
	var audioBuf []byte
	for chunk := range audioChunks {
		audioBuf = append(audioBuf, chunk...)
	}
	text, err := p.STT.Transcribe(ctx, audioBuf)
	if err != nil {
		log.Printf("STT error: %v", err)
		return
	}
	if text == "" {
		log.Println("No speech detected")
		return
	}
	log.Printf("User: %s", text)

	// 2. LLM 对话
	client.Session.AddMessage("user", text)
	history := client.Session.GetHistory()
	response, err := p.LLM.Chat(ctx, history)
	if err != nil {
		log.Printf("LLM error: %v", err)
		return
	}
	log.Printf("Bot: %s", response)
	client.Session.AddMessage("assistant", response)

	// 3. TTS 合成
	stateCh <- "speaking"
	audioBytes, err := p.TTS.Synthesize(ctx, response)
	if err != nil {
		log.Printf("TTS error: %v", err)
		return
	}

	// 4. 发送音频回设备
	client.Send <- audioBytes

	// 5. 回到空闲
	stateCh <- "idle"
}
```

---

## 8. 组装流程

### 8.1 安装开发环境

```bash
# 1. 安装 VS Code
# 2. 在 VS Code 扩展市场搜索 "PlatformIO IDE"，安装
# 3. 重启 VS Code

# 4. PlatformIO 主页 → New Project
#    Board: Espressif ESP32 Dev Module
#    Framework: Arduino
#    Location: 选 firmware-esp32/

# 5. 将上面的 platformio.ini 和 src/main.cpp 复制到项目
```

### 8.2 组装硬件顺序

1. **面包板供电轨先搭好**：红线插正极轨（3.3V），蓝线插负极轨（GND），将 ESP32 的 3.3V 和 GND 连到面包板
2. **ESP32 插上面包板**：跨中间凹槽，USB 口朝外
3. **ST7789 屏幕先接**：5 根线插好，写个简单的 TFT 测试程序确认屏幕亮
4. **INMP441 麦克风**：3 根信号线 + VCC/GND，用 TFT 屏显示波形验证收音正常
5. **MAX98357 + 喇叭**：3 根信号线 + VIN(5V) + GND，写段代码播个提示音
6. **TP4056 + 电池**：最后接，确认 ESP32 能通过 TP4056 的 OUT 口取电
7. **烧录固件**：USB 口插电脑，PlatformIO 点 Upload
8. **启动云端 Go 服务**：`go run main.go`
9. **ESP32 上电**：观察 TFT 屏幕显示 WiFi 连接状态和表情

### 8.3 调试技巧

- **串口监视器**：PlatformIO 底部 Monitor，115200 波特率，看 WiFi/WS 连接日志
- **屏幕做调试输出**：没串口线的时候，在 TFT 上画文字 `tft.drawString("WS Connected!", 0, 0, 2)`
- **音频调试**：先在 ESP32 上循环"录音→立即播放"，确认整个音频链路通，再连云端
- **WiFi 配网**：第一次用 WiFiManager 库，ESP32 开机变热点，手机连上热点配 WiFi 密码

### 8.4 常见问题

| 问题 | 排查 |
|------|------|
| 屏幕白屏 | 检查 RST 引脚是否接对；确认 TFT_eSPI 中只启用了 `ST7789_DRIVER` |
| 屏幕花屏/颜色错 | `TFT_RGB_ORDER TFT_BGR` 试试 |
| 麦克风录到噪音 | INMP441 的 L/R 接地了没；采样率是否 16000 |
| 喇叭无声 | MAX98357 SD 脚是否拉高；VIN 是否接的 5V（不是 3.3V）|
| 声音断续 | WebSocket 缓冲不够、WiFi 信号弱；降低音频采样率到 8kHz |
| ESP32 频繁重启 | 检查供电是否足（最少 500mA）；TP4056 输出电流够不够 |
| 编译报错 | 确认 platformio.ini 中 board 是 `esp32dev`；库版本是否兼容 |

---

## 附录 A：淘宝搜索链接速查

以下关键词直接复制到淘宝搜索框：

```
"ESP32 开发板 CH340C Type-C 已焊排针"
"INMP441 麦克风模块 I2S 带排针"
"MAX98357 I2S 功放模块 带排针"
"40mm 4R3W 小喇叭"
"1.3寸 ST7789 IPS 240x240 模块 带排针"
"TP4056 Type-C 充电模块 带保护"
"503040 聚合物锂电池 500mAh"
"400孔面包板 带凹槽"
"杜邦线套装 公母母母公公"
```

## 附录 B：备选屏幕方案（SSD1306 OLED 低配）

如果 ST7789 超预算，用 SSD1306 0.96" OLED 替代（¥8-12）：

**库替换**：
```
lib_deps =
    adafruit/Adafruit SSD1306 @ ^2.5
    adafruit/Adafruit GFX Library @ ^1.11
```

**接线**（I2C，只有 4 根线）：
```
ESP32 3.3V → OLED VCC
ESP32 GND  → OLED GND
ESP32 GPIO21 → OLED SDA
ESP32 GPIO22 → OLED SCL
```

**代码**：
```cpp
#include <Adafruit_SSD1306.h>
Adafruit_SSD1306 display(128, 64, &Wire, -1);

void setup() {
    display.begin(SSD1306_SWITCHCAPVCC, 0x3C);
    display.clearDisplay();
    display.setTextColor(SSD1306_WHITE);
    display.setTextSize(2);
    display.println("TinyBot");
    display.display();
}
```

缺点是单色（蓝/白），只能画像素表情，做不了彩色动画。48 行代码搞定所有表情。优势是 I2C 接线超简单（4 根线），功耗更低。

---

## 附录 C：后续迭代路线

| 版本 | 内容 | 预估追加成本 |
|------|------|:---:|
| v0.1 | 基础对话（当前方案） | ¥60-80 |
| v0.2 | + 2 轮行走（N20 底盘 + 电机驱动） | ¥50-60 |
| v0.3 | + 触觉传感器（震动开关） + TOF 测距避障 | ¥20 |
| v0.4 | + 自定义 3D 打印外壳 | ¥20 |
| v1.0 | 自己画 PCB + 嘉立创 SMT 贴片（5 片起订） | ¥30-50 |

---

*文档版本: 0.1 | 日期: 2026-05-13 | 作者: Kyle*
