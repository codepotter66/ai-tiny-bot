# TinyPal — 最低成本桌面陪伴机器人

> 无移动 | ¥45-70（基础版）| ¥70-105（含摄像头）| 零焊接 | 显示器 + 麦克风 + 喇叭 + 电池 + 云端 LLM

---

## 1. 核心设计理念

不做移动，只做对话。把所有复杂功能交给云端，本地只做：

```
收音 → 发云端 → 播语音
显示表情 / 文字反馈
（可选）拍照 → 发云端 → 视觉感知
```

这样可以把 BOM 压到 ¥45-70。第一版先做语音对话，等稳定后再加摄像头（+¥25-35）。

---

## 2. 硬件方案（BOM）

### 2.1 基础版 BOM（¥45-70）

| 序号 | 部件 | 型号 | 数量 | 约价 | 淘宝搜索关键词 |
|:---:|------|------|:---:|:---:|---------------|
| 1 | ESP32 主控 | ESP32-WROOM-32 30Pin（CH340）| 1 | ¥10-15 | `ESP32 开发板 30Pin CH340` |
| 2 | I2S 硅麦 | MSM261S4030SREC（兼容 INMP441）| 1 | ¥3-5 | `MSM261S4030SREC I2S 麦克风` |
| 3 | I2S 功放 | MAX98357 模块 | 1 | ¥4-6 | `MAX98357 I2S 功放模块` |
| 4 | 小喇叭 | 3W 4Ω（30-40mm）| 1 | ¥3-5 | `3W 4R 小喇叭 40mm` |
| 5 | 显示屏 | 0.96" SSD1306 OLED（I2C，128×64）| 1 | ¥8-12 | `SSD1306 0.96寸 OLED I2C 模块` |
| 6 | 聚合物锂电池 | 602530 / 602535（400-600mAh）| 1 | ¥6-10 | `602530 聚合物锂电池 3.7V 带保护板` |
| 7 | 充电模块 | TP4056 Type-C | 1 | ¥1.5-3 | `TP4056 Type-C 充电模块` |
| 8 | 杜邦线 | 母对母 15 根 | 1 | ¥2-3 | `杜邦线 母母 15根` |

### 2.2 摄像头可选 BOM 追加（¥25-35）

| 序号 | 部件 | 型号 | 数量 | 约价 | 淘宝搜索关键词 |
|:---:|------|------|:---:|:---:|---------------|
| 9 | ESP32-CAM | OV2640（CH340C 烧录版）| 1 | ¥25-35 | `ESP32-CAM OV2640 CH340C` |

**注意**：ESP32-CAM 没有 USB 供电口，需要单独杜邦线从主控板取电（5V/GND）。

### 2.3 电池续航分析

| 规格 | 参数 |
|------|------|
| 电池型号 | 602530 / 602535（扁平 60×25×3mm，适合拳头大小）|
| 容量 | 400-600mAh |
| 电压 | 3.7V（充满 4.2V）|

**功耗估算**：

| 工作状态 | 功耗来源 | 电流 | 时长 |
|---------|---------|:----:|:----:|
| **空闲/眨眼** | ESP32 + OLED | 60-80mA | ~6-8 小时 |
| **语音对话** | ESP32 + WiFi + 麦克风 + 功放播 TTS | 150-250mA | ~2-3 小时 |
| **连续对话+显示** | 全负载 | 200-350mA | ~1.5-2 小时 |

```
续航估算：
  500mAh / 200mA（平均）≈ 2.5 小时连续对话
  实际使用（间歇对话+空闲）≈ 4-6 小时
  待机（仅 OLED 显示）≈ 6-8 小时

加摄像头后：ESP32-CAM 额外消耗约 100-150mA
  总续航约下降 20-30%
```

---

## 3. 系统架构

### 3.1 基础版架构（无摄像头）

```
┌─────────────────────────────────────────────────────┐
│                      TinyPal 基础版                   │
│                                                      │
│   ┌──────────┐    ┌───────────┐    ┌──────────┐    │
│   │ MSM261   │    │  ESP32    │    │ MAX98357  │    │
│   │ 硅麦      │◄──┤  主控      ├──►│ 功放       │──► 喇叭│
│   └──────────┘    │           │    └──────────┘    │
│                   │  WiFi     │                     │
│   ┌──────────┐    │           │                     │
│   │ SSD1306  │◄───┤           │                     │
│   │ OLED     │    └───────────┘                     │
│   └──────────┘         │                             │
│                         │ TP4056 + 锂电池            │
└─────────────────────────┼─────────────────────────────┘
                          │ WiFi WebSocket
                          ▼
┌─────────────────────────────────────────────────────┐
│                   云端 LLM Agent                     │
│                                                      │
│   WebSocket ← 音频 PCM ← 语音 → Whisper → 文字       │
│                    │                                  │
│                    ▼                                  │
│              云端 LLM（Claude/GPT/Qwen）             │
│                    │                                  │
│                    ▼                                  │
│            文字 → 语音 TTS → 音频 MP3                 │
│                    │                                  │
│                    ▼                                  │
│         WebSocket → 音频数据 → ESP32 → 喇叭           │
└─────────────────────────────────────────────────────┘
```

### 3.2 摄像头版架构（可选扩展）

```
┌─────────────────────────────────────────────────────┐
│                 TinyPal 摄像头版                      │
│                                                      │
│   ┌──────────┐    ┌───────────┐    ┌──────────┐    │
│   │ MSM261   │    │  ESP32    │    │ MAX98357  │    │
│   │ 硅麦      │◄──┤  主控      ├──►│ 功放       │──► 喇叭│
│   └──────────┘    │           │    └──────────┘    │
│                   │           │                    │
│   ┌──────────┐    │           │                    │
│   │ SSD1306  │◄───┤           │                    │
│   │ OLED     │    └───────────┘                    │
│   └──────────┘         │                            │
│                         │                            │
│   ┌──────────────────────────────┐                  │
│   │  ESP32-CAM (独立供电)        │                  │
│   │  OV2640 摄像头               │                  │
│   │  每 5-10 秒拍照 → HTTP POST  │                  │
│   └──────────────────────────────┘                  │
│                         │                            │
│                   TP4056 + 锂电池                   │
└─────────────────────────┼────────────────────────────┘
                          │ WiFi
                          ▼
┌─────────────────────────────────────────────────────┐
│                   云端 LLM Agent                     │
│                                                      │
│   WebSocket ← 音频 PCM ← 语音 → Whisper → 文字      │
│        ↑                                              │
│        │ 周期图片 HTTP POST                           │
│        │                                              │
│   多模态分析 ───► 图片 + 语音 → LLM → 回复            │
│                    │                                  │
│                    ▼                                  │
│            文字 → 语音 TTS → 音频 MP3                 │
│                    │                                  │
│                    ▼                                  │
│         WebSocket → 音频数据 → ESP32 → 喇叭           │
└─────────────────────────────────────────────────────┘
```

**为什么用双机而不是单芯片**：
- ESP32-WROOM-32 没有 DVP 摄像头接口
- ESP32-CAM 没有 USB 供电和调试口，烧录麻烦
- 两块板子各司其职，代码更简单，维护更容易

---

## 4. 接线图

### 4.1 基础版接线（主控板）

```
ESP32 主控板              目标模块              引脚        说明
──────────────────────────────────────────────────────────────
3.3V      ───────────────  SSD1306 OLED VCC
GND       ───────────────  SSD1306 OLED GND
GPIO21    ───────────────  SSD1306 OLED SDA     (I2C 数据)
GPIO22    ───────────────  SSD1306 OLED SCL     (I2C 时钟)

3.3V      ───────────────  MSM261 VCC
GND       ───────────────  MSM261 GND
          ───────────────  MSM261 L/R → GND    (选左声道)
GPIO32    ───────────────  MSM261 SD            (I2S 数据)
GPIO33    ───────────────  MSM261 WS            (I2S 左右声道)
GPIO25    ───────────────  MSM261 SCK           (I2S 时钟)

5V        ───────────────  MAX98357 VIN
GND       ───────────────  MAX98357 GND
          ───────────────  MAX98357 GAIN → 悬空  (默认 9dB)
          ───────────────  MAX98357 SD → 5V     (使能)
GPIO26    ───────────────  MAX98357 BCLK        (I2S 时钟)
GPIO27    ───────────────  MAX98357 LRCLK       (I2S 左右声道)
GPIO14    ───────────────  MAX98357 DIN         (I2S 数据)
          ───────────────  MAX98357 SPK+/SPK- → 喇叭

──────────────────────────────────────────────────────────────
电池供电回路：
TP4056 OUT+  ──────────────  ESP32 VIN（或 USB 5V 针脚）
TP4056 OUT-  ──────────────  ESP32 GND
TP4056 IN+   ──────────────  电池正极（红线）
TP4056 IN-   ──────────────  电池负极（黑线）
TP4056 USB-C  ──────────────  充电线（边用边充）

USB Micro-USB  ────────────  烧录 / 调试（平时可断开）
```

**接线说明**：
- I2C OLED：4 根线（SDA/SCL/VCC/GND），占用 GPIO21/22
- I2S 麦克风：MSM261S4030SREC，L/R 脚接地选左声道，占用 GPIO32/33/25
- I2S 功放：MAX98357 VIN 接 5V（USB 或电池供电都够），占用 GPIO26/27/14
- 电池：TP4056 OUT 直接接 ESP32 VIN，插 USB 边充边用，拔 USB 电池接管

### 4.2 摄像头接线（可选追加）

```
ESP32-CAM                    目标                  说明
───────────────────────────────────────────────────
3.3V     ──────────────────  (摄像头板自带 3.3V LDO)
GND      ──────────────────  (摄像头板 GND)
5V       ──────────────────  ESP32 主控板的 5V     (从主控取电)
GND      ──────────────────  ESP32 主控板的 GND    (共地)

注意：ESP32-CAM 没有 USB 口，烧录需要用 CH340C 烧录器
      或者通过主控 ESP32 做桥接烧录
```

**摄像头供电注意**：ESP32-CAM 需要 5V 供电（200-300mA），可以从主控板的 5V 引脚取电。如果主控板用 USB 供电，同时给 CAM 供电没问题；如果只用电池供电，CAM 会拉低电池续航。

---

## 5. 固件代码

### 5.1 主控板固件（main.cpp）

```cpp
#include <Arduino.h>
#include <WiFi.h>
#include <WebSocketsClient.h>
#include <Wire.h>
#include <Adafruit_GFX.h>
#include <Adafruit_SSD1306.h>
#include <driver/i2s.h>

// ============ 引脚定义 ============
#define I2S_MIC_SD    32
#define I2S_MIC_WS    33
#define I2S_MIC_SCK   25

#define I2S_SPK_BCLK  26
#define I2S_SPK_LRCLK 27
#define I2S_SPK_DIN   14

#define OLED_SDA      21
#define OLED_SCL      22

// ============ OLED 配置 ============
#define SCREEN_WIDTH  128
#define SCREEN_HEIGHT 64
#define OLED_RESET    -1
#define OLED_ADDR     0x3C

Adafruit_SSD1306 display(SCREEN_WIDTH, SCREEN_HEIGHT, &Wire, OLED_RESET);

// ============ WiFi 配置 ============
const char* WIFI_SSID = "YOUR_WIFI_SSID";
const char* WIFI_PASS = "YOUR_WIFI_PASSWORD";
const char* WS_HOST   = "your-server.com";  // 替换为你 Go 服务的地址
const uint16_t WS_PORT = 8080;
const char* WS_PATH    = "/ws";

// ============ WebSocket ============
WebSocketsClient wsClient;

// ============ 状态 ============
enum State { CONNECTING, IDLE, LISTENING, THINKING, SPEAKING };
State currentState = CONNECTING;

// ============ OLED 绘制函数 ============
void drawConnecting() {
    display.clearDisplay();
    display.setTextSize(1);
    display.setTextColor(SSD1306_WHITE);
    display.setCursor(0, 28);
    display.println(" TinyPal");
    display.setTextSize(1);
    display.setCursor(20, 45);
    display.println("Connecting...");
    display.display();
}

void drawIdle(const char* msg = "小主人好呀!") {
    display.clearDisplay();
    display.setTextSize(1);
    display.setTextColor(SSD1306_WHITE);
    display.setCursor(0, 5);
    display.println(" TinyPal");
    display.drawLine(0, 15, 128, 15, SSD1306_WHITE);
    display.setTextSize(2);
    display.setCursor(10, 28);
    display.println(msg);
    display.setTextSize(1);
    display.setCursor(0, 56);
    display.println("Listening...");
    display.display();
}

void drawListening() {
    display.clearDisplay();
    display.setTextSize(1);
    display.setCursor(0, 5);
    display.println(" TinyPal");
    display.drawLine(0, 15, 128, 15, SSD1306_WHITE);
    display.setTextSize(2);
    display.setCursor(15, 30);
    display.println("[...]");
    // 波形动画
    static int t = 0;
    int h = (sin(t * 0.3) + 1) * 10 + 5;
    display.fillRect(50, 50 - h/2, 4, h, SSD1306_WHITE);
    display.fillRect(58, 50 - 10, 4, 20, SSD1306_WHITE);
    display.fillRect(66, 50 - h/2, 4, h, SSD1306_WHITE);
    display.display();
    t++;
}

void drawThinking() {
    display.clearDisplay();
    display.setTextSize(1);
    display.setCursor(0, 5);
    display.println(" TinyPal");
    display.drawLine(0, 15, 128, 15, SSD1306_WHITE);
    display.setTextSize(2);
    display.setCursor(20, 30);
    display.println("[...]");
    display.setTextSize(1);
    display.setCursor(30, 50);
    display.println("Thinking...");
    display.display();
}

void drawSpeaking() {
    display.clearDisplay();
    display.setTextSize(1);
    display.setCursor(0, 5);
    display.println(" TinyPal");
    display.drawLine(0, 15, 128, 15, SSD1306_WHITE);
    display.setTextSize(2);
    display.setCursor(20, 30);
    display.println(">>>");
    // 音量条动画
    for (int i = 0; i < 8; i++) {
        int h = random(5, 25);
        display.fillRect(20 + i * 12, 55 - h, 8, h, SSD1306_WHITE);
    }
    display.display();
}

void updateDisplay() {
    switch (currentState) {
        case CONNECTING: drawConnecting(); break;
        case IDLE: drawIdle(); break;
        case LISTENING: drawListening(); break;
        case THINKING: drawThinking(); break;
        case SPEAKING: drawSpeaking(); break;
    }
}

// ============ I2S 麦克风配置 ============
void setupI2SMic() {
    i2s_config_t i2s_config = {
        .mode = (i2s_mode_t)(I2S_MODE_MASTER | I2S_MODE_RX),
        .sample_rate = 16000,
        .bits_per_sample = I2S_BITS_PER_SAMPLE_16BIT,
        .channel_format = I2S_CHANNEL_FMT_ONLY_LEFT,
        .communication_format = I2S_COMM_FORMAT_STAND_I2S,
        .intr_alloc_flags = ESP_INTR_FLAG_LEVEL1,
        .dma_buf_count = 8,
        .dma_buf_len = 64,
        .use_apll = false,
        .tx_desc_auto_clear = false,
        .fixed_mclk = 0
    };
    i2s_pin_config_t pin_config = {
        .bck_io_num = I2S_MIC_SCK,
        .ws_io_num = I2S_MIC_WS,
        .data_out_num = I2S_PIN_NO_CHANGE,
        .data_in_num = I2S_MIC_SD
    };
    i2s_driver_install(I2S_NUM_0, &i2s_config, 0, NULL);
    i2s_set_pin(I2S_NUM_0, &pin_config);
    i2s_zero_dma_buffer(I2S_NUM_0);
}

// ============ I2S 功放配置 ============
void setupI2SSpeaker() {
    i2s_config_t i2s_config = {
        .mode = (i2s_mode_t)(I2S_MODE_MASTER | I2S_MODE_TX),
        .sample_rate = 24000,
        .bits_per_sample = I2S_BITS_PER_SAMPLE_16BIT,
        .channel_format = I2S_CHANNEL_FMT_ONLY_LEFT,
        .communication_format = I2S_COMM_FORMAT_STAND_I2S,
        .intr_alloc_flags = ESP_INTR_FLAG_LEVEL1,
        .dma_buf_count = 8,
        .dma_buf_len = 256,
        .use_apll = false,
        .tx_desc_auto_clear = true,
        .fixed_mclk = 0
    };
    i2s_pin_config_t pin_config = {
        .bck_io_num = I2S_SPK_BCLK,
        .ws_io_num = I2S_SPK_LRCLK,
        .data_out_num = I2S_SPK_DIN,
        .data_in_num = I2S_PIN_NO_CHANGE
    };
    i2s_driver_install(I2S_NUM_1, &i2s_config, 0, NULL);
    i2s_set_pin(I2S_NUM_1, &pin_config);
}

// ============ WebSocket 回调 ============
void webSocketEvent(WStype_t type, uint8_t* payload, size_t length) {
    switch (type) {
        case WStype_CONNECTED:
            currentState = IDLE;
            Serial.println("[WS] Connected!");
            break;
        case WStype_DISCONNECTED:
            currentState = CONNECTING;
            Serial.println("[WS] Disconnected!");
            break;
        case WStype_BIN:
            if (currentState != SPEAKING) currentState = SPEAKING;
            size_t written = 0;
            i2s_write(I2S_NUM_1, payload, length, &written, portMAX_DELAY);
            currentState = IDLE;
            break;
        case WStype_TEXT: {
            String text = String((char*)payload);
            if (text.indexOf("thinking") >= 0) currentState = THINKING;
            else if (text.indexOf("speaking") >= 0) currentState = SPEAKING;
            else if (text.indexOf("idle") >= 0) currentState = IDLE;
            else if (text.indexOf("listening") >= 0) currentState = LISTENING;
            break;
        }
        case WStype_ERROR:
            Serial.println("[WS] Error!");
            break;
    }
}

// ============ 音频采集任务 ============
void audioTask(void* param) {
    uint8_t audioBuffer[1024];
    size_t bytesRead;

    while (true) {
        esp_err_t result = i2s_read(I2S_NUM_0, audioBuffer, sizeof(audioBuffer),
                                     &bytesRead, portMAX_DELAY);

        if (result == ESP_OK && bytesRead > 0 && wsClient.connected()) {
            int16_t* samples = (int16_t*)audioBuffer;
            int sum = 0;
            int count = bytesRead / 2;
            for (int i = 0; i < count; i++) {
                sum += abs(samples[i]);
            }
            int avg = sum / count;
            const int THRESHOLD = 500;

            if (avg > THRESHOLD) {
                currentState = LISTENING;
            } else {
                if (currentState == LISTENING) {
                    currentState = THINKING;
                }
            }

            if (currentState == LISTENING || currentState == THINKING) {
                wsClient.sendBIN(audioBuffer, bytesRead);
            }
        }
        vTaskDelay(1);
    }
}

// ============ 显示任务 ============
void displayTask(void* param) {
    while (true) {
        updateDisplay();
        vTaskDelay(100);
    }
}

// ============ Setup ============
void setup() {
    Serial.begin(115200);

    Wire.begin(OLED_SDA, OLED_SCL);
    if (!display.begin(SSD1306_SWITCHCAPVCC, OLED_ADDR)) {
        Serial.println("OLED init failed!");
    }
    display.clearDisplay();
    display.display();

    setupI2SMic();
    setupI2SSpeaker();

    display.clearDisplay();
    display.setTextSize(1);
    display.setCursor(20, 28);
    display.println("WiFi...");
    display.display();

    WiFi.begin(WIFI_SSID, WIFI_PASS);
    while (WiFi.status() != WL_CONNECTED) {
        delay(300);
    }
    Serial.println("WiFi connected");

    wsClient.begin(WS_HOST, WS_PORT, WS_PATH);
    wsClient.onEvent(webSocketEvent);
    wsClient.setReconnectInterval(3000);
    wsClient.enableHeartbeat(15000, 3000, 2);

    xTaskCreatePinnedToCore(audioTask, "Audio", 4096, NULL, 2, NULL, 0);
    xTaskCreatePinnedToCore(displayTask, "Display", 2048, NULL, 1, NULL, 1);
}

void loop() {
    wsClient.loop();
    delay(10);
}
```

### 5.2 ESP32-CAM 固件（摄像头端，独立运行）

```cpp
#include <esp_camera.h>
#include <HTTPClient.h>
#include <WiFi.h>

// WiFi 和服务器配置
const char* WIFI_SSID = "YOUR_WIFI_SSID";
const char* WIFI_PASS = "YOUR_WIFI_PASSWORD";
const char* SERVER_URL = "http://your-server.com/camera";  // Go 服务地址

// 摄像头引脚定义（ESP32-CAM 默认引脚）
#define PWDN_GPIO_NUM     32
#define RESET_GPIO_NUM    -1
#define XCLK_GPIO_NUM      0
#define SIOD_GPIO_NUM     26
#define SIOC_GPIO_NUM     27
#define Y9_GPIO_NUM       35
#define Y8_GPIO_NUM       34
#define Y7_GPIO_NUM       39
#define Y6_GPIO_NUM       36
#define Y5_GPIO_NUM        5
#define Y4_GPIO_NUM       18
#define Y3_GPIO_NUM       19
#define Y2_GPIO_NUM       21
#define VSYNC_GPIO_NUM    25
#define HREF_GPIO_NUM     23
#define PCLK_GPIO_NUM     22

void setup() {
    Serial.begin(115200);

    // WiFi 连接
    WiFi.begin(WIFI_SSID, WIFI_PASS);
    Serial.print("Connecting to WiFi");
    while (WiFi.status() != WL_CONNECTED) {
        delay(300);
        Serial.print(".");
    }
    Serial.println("\nWiFi connected: " + WiFi.localIP().toString());

    // 摄像头初始化
    camera_config_t config;
    config.pin_pwdn  = PWDN_GPIO_NUM;
    config.pin_reset = RESET_GPIO_NUM;
    config.pin_xclk  = XCLK_GPIO_NUM;
    config.pin_sscb_sda = SIOD_GPIO_NUM;
    config.pin_sscb_scl = SIOC_GPIO_NUM;
    config.pin_d7 = Y9_GPIO_NUM;
    config.pin_d6 = Y8_GPIO_NUM;
    config.pin_d5 = Y7_GPIO_NUM;
    config.pin_d4 = Y6_GPIO_NUM;
    config.pin_d3 = Y5_GPIO_NUM;
    config.pin_d2 = Y4_GPIO_NUM;
    config.pin_d1 = Y3_GPIO_NUM;
    config.pin_d0 = Y2_GPIO_NUM;
    config.pin_vsync = VSYNC_GPIO_NUM;
    config.pin_href  = HREF_GPIO_NUM;
    config.pin_pclk  = PCLK_GPIO_NUM;
    config.xclk_freq_hz = 20000000;
    config.pixel_format = PIXFORMAT_JPEG;
    config.frame_size = FRAMESIZE_SVGA;   // 800x600，性价比最高
    config.jpeg_quality = 10;              // 0-63，数值越小质量越高
    config.fb_count = 2;                  // 双缓冲

    esp_err_t err = esp_camera_init(&config);
    if (err != ESP_OK) {
        Serial.printf("Camera init failed with error 0x%x\n", err);
        return;
    }

    Serial.println("Camera ready!");
}

void loop() {
    static unsigned long lastCapture = 0;
    unsigned long now = millis();

    // 每 5-10 秒拍一张照片
    if (now - lastCapture > 5000) {
        camera_fb_t* fb = esp_camera_fb_get();
        if (!fb) {
            Serial.println("Camera capture failed");
            return;
        }

        if (fb->len > 0) {
            HTTPClient http;
            http.begin(SERVER_URL);
            http.addHeader("Content-Type", "image/jpeg");

            int httpCode = http.POST(fb->buf, fb->len);
            if (httpCode > 0) {
                Serial.printf("[CAM] Frame sent, response: %d\n", httpCode);
            } else {
                Serial.printf("[CAM] POST failed: %s\n", http.errorToString(httpCode).c_str());
            }
            http.end();
        }

        esp_camera_fb_return(fb);
        lastCapture = now;
    }

    delay(10);
}
```

**烧录 ESP32-CAM 的方式**：
1. 买带 CH340C 烧录的 ESP32-CAM（¥25-35 套装），自带 USB 口
2. 或者买普通 ESP32-CAM + 外接 CH340 烧录器（¥5-10）

### 5.3 platformio.ini（主控板）

```ini
[env:esp32dev]
platform = espressif32
board = esp32dev
framework = arduino
monitor_speed = 115200

lib_deps =
    links2004/WebSockets @ ^2.4.2
    adafruit/Adafruit SSD1306 @ ^2.5
    adafruit/Adafruit GFX Library @ ^1.11

build_flags =
    -DCORE_DEBUG_LEVEL=0
```

### 5.4 platformio.ini（ESP32-CAM，单独项目）

```ini
[env:esp32cam]
platform = espressif32
board = esp32-cam
framework = arduino
monitor_speed = 115200
upload_speed = 115200

board_build.flash_mode = dio
board_build.f_flash = 40000000L

lib_deps =

build_flags =
    -DCORE_DEBUG_LEVEL=0
    -DCAMERA_MODEL_ESP32CAM
```

---

## 6. 云端 Go 服务

### 6.1 服务架构

```
WebSocket (音频) ◄─────── TinyPal 主控板 (ESP32)
                           │
                           ▼
                      音频 PCM 收集
                           │
                           ▼
                   Whisper STT → 文字
                           │
                           ▼
                   文字 + 图片 → LLM 多模态分析
                           │
                           ▼
                   LLM 回复 → 文字
                           │
              ┌────────────┴────────────┐
              ▼                          ▼
        Edge-TTS 合成语音           OLED 显示文字
              │                          │
              ▼                          │
      WebSocket 音频 ◄───────── 主控板播放
```

### 6.2 Go 服务代码

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "sync"
    "time"

    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true },
}

type Session struct {
    History   []ChatMessage
    LastImage []byte
    mu        sync.Mutex
}

type ChatMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type CameraPayload struct {
    Timestamp time.Time `json:"timestamp"`
}

var sessions = make(map[*websocket.Conn]*Session)
var sessionsMu sync.RWMutex

// ============ WebSocket 音频处理 ============
func wsHandler(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("WebSocket upgrade error: %v", err)
        return
    }
    defer conn.Close()

    session := &Session{History: []ChatMessage{
        {Role: "system", Content: "你是一个可爱的桌面陪伴机器人，名字叫小圆。回答简洁有趣，口语化。"},
    }}
    sessionsMu.Lock()
    sessions[conn] = session
    sessionsMu.Unlock()

    audioBuf := []byte{}

    for {
        msgType, data, err := conn.ReadMessage()
        if err != nil {
            break
        }

        switch msgType {
        case websocket.BinaryMessage:
            // 音频数据，累积到缓冲区
            audioBuf = append(audioBuf, data...)
            // 简单 VAD：如果超过 3 秒音频，触发识别
            if len(audioBuf) > 96000 { // 3s * 16kHz * 16bit
                go processAudio(session, audioBuf, conn)
                audioBuf = []byte{}
            }

        case websocket.TextMessage:
            // 控制指令（如 {"cmd": "reset"})
            handleCommand(session, string(data))
        }
    }

    sessionsMu.Lock()
    delete(sessions, conn)
    sessionsMu.Unlock()
}

// ============ 图片接收 ============
func cameraHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        return
    }

    imgData, err := io.ReadAll(r.Body)
    if err != nil {
        log.Printf("Camera: read body error: %v", err)
        w.WriteHeader(400)
        return
    }

    // 保存最新图片（供语音对话时使用）
    sessionsMu.Lock()
    for _, session := range sessions {
        session.mu.Lock()
        session.LastImage = imgData
        session.mu.Unlock()
    }
    sessionsMu.Unlock()

    log.Printf("Camera: received %d bytes", len(imgData))
    w.WriteHeader(200)
}

// ============ 处理音频 ============
func processAudio(session *Session, audioData []byte, conn *websocket.Conn) {
    ctx := context.Background()

    // STT
    text, err := stt.Transcribe(ctx, audioData)
    if err != nil || text == "" {
        return
    }
    log.Printf("[STT] %s", text)

    // 发送给客户端状态更新
    conn.WriteJSON(map[string]string{"state": "thinking"})

    // LLM 对话（带上图片）
    session.mu.Lock()
    img := session.LastImage
    session.mu.Unlock()

    resp, err := llm.ChatWithImage(ctx, session.History, text, img)
    if err != nil {
        log.Printf("[LLM] error: %v", err)
        return
    }

    log.Printf("[LLM] %s", resp)

    session.mu.Lock()
    session.History = append(session.History,
        ChatMessage{Role: "user", Content: text},
        ChatMessage{Role: "assistant", Content: resp})
    session.mu.Unlock()

    // 发送 TTS 音频
    audioResp, err := tts.Synthesize(ctx, resp)
    if err == nil {
        conn.WriteJSON(map[string]string{"state": "speaking"})
        conn.WriteMessage(websocket.BinaryMessage, audioResp)
    }

    // 发送文本到 OLED 显示
    conn.WriteJSON(map[string]string{"state": "idle", "text": resp})
}

// ============ 处理指令 ============
func handleCommand(session *Session, cmd string) {
    var msg map[string]interface{}
    if err := json.Unmarshal([]byte(cmd), &msg); err != nil {
        return
    }
    if c, ok := msg["cmd"].(string); ok {
        switch c {
        case "reset":
            session.mu.Lock()
            session.History = []ChatMessage{
                {Role: "system", Content: "你是一个可爱的桌面陪伴机器人，名字叫小圆。"},
            }
            session.mu.Unlock()
        }
    }
}

// ============ 主函数 ============
func main() {
    http.HandleFunc("/ws", wsHandler)
    http.HandleFunc("/camera", cameraHandler)

    fmt.Println("TinyPal server running on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### 6.3 STT / LLM / TTS 模块（占位，需要你对接实际 API）

```go
package main

import (
    "context"
    "io"
    "net/http"
    "strings"
)

// ============ STT (Whisper) ============
func transcribeAudio(audioData []byte) (string, error) {
    // 实际用 your-whisper-service:9000
    req, _ := http.NewRequest("POST", "http://localhost:9000/asr", bytes.NewReader(audioData))
    req.Header.Set("Content-Type", "audio/wav")
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    return strings.Trim(string(body), "\""), nil
}

// ============ LLM (Claude/OpenAI) ============
func chatWithImage(ctx context.Context, history []ChatMessage, text string, img []byte) (string, error) {
    // Claude API 支持图片输入
    // body := map[string]interface{}{
    //     "model": "claude-3-5-sonnet-20241014",
    //     "messages": append(history, ChatMessage{
    //         Role:    "user",
    //         Content: []map[string]interface{}{
    //             {"type": "text", "text": text},
    //             {"type": "image", "source": map[string]interface{}{
    //                 "type":      "base64",
    //                 "media_type": "image/jpeg",
    //                 "data":       base64.StdEncoding.EncodeToString(img),
    //             }},
    //         },
    //     }),
    // }
    // ...
    return "我看到你了！", nil
}

// ============ TTS (Edge-TTS) ============
func synthesizeSpeech(text string) ([]byte, error) {
    // 实际用 edge-tts 库：await edge_tts.Communicate(text, "zh-CN-XiaoxiaoNeural")
    return []byte{}, nil
}
```

---

## 7. 购买清单

### 7.1 基础版购买（¥45-70）

```
淘宝搜索关键词（直接复制）：

"ESP32 开发板 30Pin CH340"
"MSM261S4030SREC I2S 麦克风 带排针"
"MAX98357 I2S 功放模块 带排针"
"3W 4R 小喇叭 40mm 内磁"
"SSD1306 0.96寸 OLED I2C 模块"
"602530 聚合物锂电池 3.7V 带保护板"
"TP4056 Type-C 充电模块"
"杜邦线 母母 15根"
```

### 7.2 摄像头版追加购买

```
"ESP32-CAM OV2640 CH340C"    ← 带 CH340 烧录版的更好烧录
```

### 7.3 工具准备

- USB Micro-USB 数据线（烧录用）
- 镊子（杜邦线接头辅助）
- （可选）CH340 烧录器（买 ESP32-CAM 选带 CH340 的套装就不用单独买）

---

## 8. 快速开始顺序

```
第 1 步：组装主控板（基础版 BOM）
  1. 按第 4 节接线图连接所有线
  2. PlatformIO 烧录 main.cpp
  3. 修改 WiFi SSID / Password 和 WS_HOST
  4. 接上电池，观察 OLED 显示

第 2 步：启动云端服务
  1. go run main.go
  2. 服务监听 :8080

第 3 步：测试语音对话
  1. 对着麦克风说话
  2. 观察 OLED 状态变化
  3. 听 TTS 回复

第 4 步（可选）：加摄像头
  1. 买 ESP32-CAM（¥25-35）
  2. PlatformIO 新建项目，烧录 cam.ino
  3. 修改 WiFi 和 SERVER_URL
  4. 摄像头独立供电，观察日志
```

---

## 9. 成本总结

| 版本 | BOM 成本 | 功能 |
|------|:-------:|------|
| **TinyPal 基础版** | ¥45-70 | 听 + 说 + 看 + 电池续航 4-6h |
| **TinyPal + 摄像头** | ¥70-105 | 听 + 说 + 看 + 电池 + 视觉感知 |
| **TinyBot（含行走）** | ¥145-163 | 听 + 说 + 看 + 电池 + 四足行走 |

---

*文档版本: 2.0 | 日期: 2026-05-13 | 重写：摄像头作为可选模块整合到主文档，统一架构、接线、代码*