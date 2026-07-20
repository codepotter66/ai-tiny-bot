# TinyPal 接线测试计划 — 方案 B（分阶段）

> 目标：每阶段完成一个模块的接线 + 验证，不积累问题
> 开发工具：VS Code + PlatformIO
> 总阶段：6 步

---

## 如何接电源（先看这里）

整机只有**一个主供电入口：ESP32 开发板**。OLED / 麦克风 / 功放都从 ESP32 的 `3.3V` 或 `5V` 取电，不需要再单独接墙插。

### 方式 A：USB 供电（开发推荐）

1. 用**能传数据**的 USB 线，把 ESP32 接到电脑（或 5V USB 充电器）
2. 板上电源灯亮 → 已供电；同一根线也可烧录固件
3. 桌面联调、第 3–6 步都可以先只用这一种，不必接电池

### 方式 B：锂电池 + TP4056（便携成品）

1. 锂电池红/黑 → TP4056 的 `B+` / `B-`（充电输入）
2. TP4056 的 `OUT+` / `OUT-` → ESP32 的 `VIN` / `GND`
3. 给电池充电：充电器插 TP4056 的 USB-C（不是插 ESP32）
4. 细节与万用表验证见下方第 1 步、第 2 步

### 电源关系

```
  [电脑 / 充电器 USB] ──→ ESP32 USB口 ──┐
                                         ├─→ ESP32 内部稳压
  [锂电池] → TP4056 → ESP32 VIN/GND ────┘      │
                                               ├─→ 3.3V → OLED、麦克风(MS4030)
                                               └─→ 5V   → 功放(MAX98357)、喇叭驱动
```

**注意：**

1. 极性千万别反：电池红→B+，黑→B-；OUT+→VIN，OUT-→GND
2. 开发时若已用 USB 供电，可先不接 VIN
3. 麦克风吃 `3.3V`，功放吃 `5V`，不要接反

---

## 整体接线总图（先看一遍心里有数）

```
┌──────────────────────────────────────────────────────────┐
│                                                          │
│  ┌──────────┐  红 ──→ TP4056 OUT+ ──→ ESP32 VIN       │
│  │  锂电池   │  黑 ──→ TP4056 OUT- ──→ ESP32 GND      │
│  │ 602535   │       │                                   │
│  │ 3.7V     │  TP4056 IN+ ←─── 电池红线                 │
│  │ 600mAh   │  TP4056 IN- ←─── 电池黑线                 │
│  └──────────┘  TP4056 USB-C ←── 充电线（边充边用）       │
│                                                          │
│  ┌──────────┐                                           │
│  │  ESP32   │  ← USB 烧录线                            │
│  │  主控板   │                                           │
│  │          │  GPIO21 ──→ OLED SDA                    │
│  │  3.3V ───┼──→ OLED VCC                             │
│  │  GND  ───┼──→ OLED GND                             │
│  │  GPIO22 ──┼──→ OLED SCL                             │
│  │          │                                           │
│  │  GPIO32 ──┼──→ MS4030 SD                           │
│  │  GPIO33 ──┼──→ MS4030 WS                           │
│  │  GPIO25 ──┼──→ MS4030 SCK                          │
│  │  3.3V ───┼──→ MS4030 VDD（有的板丝印为 VCC）      │
│  │  GND  ───┼──→ MS4030 GND                          │
│  │          │  MS4030 L/R ───→ GND（选左声道）         │
│  │          │                                           │
│  │  GPIO26 ──┼──→ MAX98357 BCLK                       │
│  │  GPIO27 ──┼──→ MAX98357 LRC（有的板丝印为 LRCLK）  │
│  │  GPIO14 ──┼──→ MAX98357 DIN                        │
│  │  5V    ───┼──→ MAX98357 VIN                        │
│  │  GND  ───┼──→ MAX98357 GND                         │
│  │          │  MAX98357 SPK+/SPK- ───→ 喇叭           │
│  └──────────┘                                           │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

---

## 第 1 步：电池 + TP4056 供电测试

### 1.1 接线

```
锂电池（3.7V）              TP4056 充电模块
┌──────────────┐         ┌──────────────────┐
│  红线 = 正极  │ ──────→ │  B+ / IN+        │  ← 电池进来充电
│  黑线 = 负极  │ ──────→ │  B- / IN-        │
└──────────────┘         │  USB-C           │  ← 插充电器给电池充
                         │  OUT+ / OUT-     │  ← 本步先不接 ESP32
                         └──────────────────┘

接线顺序（断电、先不插充电器）：
1. 电池红线 → TP4056 B+（有的板标 IN+）
2. 电池黑线 → TP4056 B-（有的板标 IN-）
3. 确认没接反后再测电压（下一步）

说明：B+/B- 是「接电池」；OUT+/OUT- 是「输出给 ESP32」。
不要把电池直接接到 ESP32 的 3.3V。
```

### 1.2 万用表检查（接好后测）

```
步骤 1：万用表自检
  BEEP 档 → 红笔黑笔相碰 → 有蜂鸣 ✅

步骤 2：测充电模块 OUT 输出
  DCV 20V
  红笔 → TP4056 OUT+（标 B+ 或 OUT+）
  黑笔 → TP4056 OUT-（标 B- 或 OUT-）

  读数：
    4.8V - 5.2V ✅ → 充电模块正常
    0V ❌ → 重新检查接线

步骤 3：确认正负极没有接反（没蜂鸣说明没短路）
  BEEP 档
  红笔 → TP4056 OUT+
  黑笔 → TP4056 OUT-

  没蜂鸣 ✅（没有短路）
```

### 1.3 通过标准

- 万用表测 OUT+ / OUT- 显示 4.8V-5.2V ✅
- OUT+ / OUT- 之间没有蜂鸣（没有短路）✅

**通过 → 进入第 2 步**

---

## 第 2 步：电池 → ESP32 供电

若你暂时只用电脑 USB 给 ESP32 供电（方式 A），可跳过本步，直接做第 3 步烧录；做便携成品时再回来接电池。

### 2.1 接线

```
TP4056（电池已接好）          ESP32 DevKit
┌──────────────────┐         ┌──────────────────┐
│  OUT+            │ ──────→ │  VIN（或标 5V）   │  ← 板载稳压输入
│  OUT-            │ ──────→ │  GND（任意一个）  │
│  B+ / B-         │ ── 已接电池               │
│  USB-C           │ ── 需要充电时再插         │
└──────────────────┘         └──────────────────┘

只需 2 根杜邦线：
  OUT+ → VIN
  OUT- → GND

不要接到 3.3V 针脚（电压过高会损坏芯片）。
```

### 2.2 万用表检查

```
步骤 1：确认 ESP32 3.3V 正常
  DCV 20V
  红笔 → ESP32 的 3.3V 引脚
  黑笔 → ESP32 的 GND 引脚

  读数：
    3.28V - 3.36V ✅ → ESP32 供电正常
    <3.0V ❌ → 检查 TP4056 OUT 是否接对 VIN
    0V ❌ → 检查 GND 位置，或 TP4056 OUT 有没有电

步骤 2：确认 3.3V 和 GND 没有短路
  BEEP 档
  红笔 → 3.3V 引脚
  黑笔 → GND 引脚

  没蜂鸣 ✅（3.3V 和 GND 之间无短路）
```

### 2.3 通过标准

- 3.3V 引脚显示 3.28V-3.36V ✅
- 3.3V / GND 之间没有短路 ✅

**通过 → 进入第 3 步**

---

## 第 3 步：烧录 Blink 测试固件

### 3.1 安装 PlatformIO

1. VS Code 扩展市场搜索 **PlatformIO IDE**
2. 安装后重启 VS Code
3. 重启后左侧边栏出现 **PI** 图标

### 3.2 新建项目

1. 点击 PI 图标 → **New Project**
2. Name: `tinypal-blink`
3. Board: `Espressif ESP32 Dev Module`
4. Framework: `Arduino`
5. Location: 选择你项目目录
6. 点击 **Finish** 等待创建完成

### 3.3 写入测试代码

打开 `src/main.cpp`，删除全部内容，替换为：

```cpp
#include <Arduino.h>

// ESP32 内置 LED 通常在 GPIO2
#define LED_PIN 2

void setup() {
    pinMode(LED_PIN, OUTPUT);
    Serial.begin(115200);
    Serial.println("Blink test started");
}

void loop() {
    digitalWrite(LED_PIN, HIGH);
    Serial.println("LED ON");
    delay(1000);
    digitalWrite(LED_PIN, LOW);
    Serial.println("LED OFF");
    delay(1000);
}
```

### 3.4 烧录

1. USB 线连接 ESP32 到电脑
2. VS Code 底部 PlatformIO 工具栏
3. 点击 **→ Upload**（向右的箭头）
4. 等待编译 + 上传（约 1-2 分钟）

### 3.5 验证

烧录成功后：
- ESP32 开发板上的小蓝灯每 1 秒闪烁一次 ✅
- 打开 **Serial Monitor**（PI 工具栏 → 🔍 图标）
- 波特率选 **115200**
- 看到 "LED ON / LED OFF" 交替打印 ✅

### 3.6 通过标准

- 内置 LED 每秒闪烁一次 ✅
- 串口输出正常 ✅

**通过 → 进入第 4 步**

---

## 第 4 步：接线 OLED 屏幕

### 4.1 接线

```
OLED 模块（4针，从左到右）    ESP32 开发板
┌────────────────────────┐
│  VCC   ───────────────→│  3.3V
│  GND   ───────────────→│  GND（任意）
│  SDA   ───────────────→│  GPIO21
│  SCL   ───────────────→│  GPIO22
└────────────────────────┘
```

**4 根杜邦线**，全部插在面包板上，然后杜邦线另一头插 ESP32。

### 4.2 写入屏幕测试代码

在 PlatformIO 中新建项目 `tinypal-oled`，或在现有项目中新建文件 `src/oled_test.cpp`：

```cpp
#include <Arduino.h>
#include <Wire.h>
#include <Adafruit_GFX.h>
#include <Adafruit_SSD1306.h>

#define SCREEN_WIDTH  128
#define SCREEN_HEIGHT 64
#define OLED_RESET    -1
#define OLED_ADDR    0x3C

Adafruit_SSD1306 display(SCREEN_WIDTH, SCREEN_HEIGHT, &Wire, OLED_RESET);

void setup() {
    Serial.begin(115200);

    Wire.begin(21, 22);  // SDA=GPIO21, SCL=GPIO22

    if (!display.begin(SSD1306_SWITCHCAPVCC, OLED_ADDR)) {
        Serial.println("OLED init FAILED!");
        while (true);
    }

    Serial.println("OLED init OK!");
    display.clearDisplay();
    display.setTextColor(SSD1306_WHITE);
    display.setTextSize(2);
    display.setCursor(20, 25);
    display.println("TinyPal");
    display.setTextSize(1);
    display.setCursor(30, 50);
    display.println("Hello World!");
    display.display();
}

void loop() {
    // 屏幕静态显示，不需要 loop 刷新
}
```

### 4.3 添加依赖库

打开 `platformio.ini`，在 `[env:esp32dev]` 下添加：

```ini
lib_deps =
    adafruit/Adafruit SSD1306 @ ^2.5
    adafruit/Adafruit GFX Library @ ^1.11
```

点击 **Build**（打勾图标）编译一次，确认没有报错（`Adafruit SSD1306` 库会自动下载）。

### 4.4 烧录

点击 **→ Upload** 烧录到 ESP32。

### 4.5 验证

烧录完成后：
- **OLED 屏幕亮起来**，显示 "TinyPal" 和 "Hello World!" ✅
- 串口输出 "OLED init OK!" ✅

如果屏幕不亮：
- 检查 4 根杜邦线是否插紧（VCC/GND/SDA/SCL 顺序对不对）
- 交换 SDA 和 SCL 试试（有些板子刚好反的）
- 用万用表测 OLED VCC 引脚是否有 3.3V

### 4.6 通过标准

- OLED 屏幕正常显示文字 ✅

**通过 → 进入第 5 步**

---

## 第 5 步：接线音频（麦克风 + 功放 + 喇叭）

### 5.1 接线

**MS4030 麦克风（常见 6 针）**：

常见模块焊针为：`SD / VDD / GND / L/R / WS / SCK`。丝印 `VDD` 与偶见的 `VCC` 是同一根电源正极，接到 ESP32 **3.3V**（不要接 5V）。少数板子多一颗空脚 `DC`，不用接。

> **软件注意：** MS4030（INMP441 兼容）输出 **24-bit 数据落在 32-bit I2S 槽**。固件必须按 32-bit 读入再右移对齐成 16-bit PCM；若按 16-bit 直接读，回放会电音/杂乱。实时回放测试时麦克风尽量远离喇叭，避免啸叫。

```
MS4030 模块           ESP32
┌──────────────────┐
│  VDD  ──────────→│  3.3V   ← 电源正极（有的板丝印为 VCC）
│  GND  ──────────→│  GND    ← 电源负极 / 地
│  L/R  ──────────→│  GND    ← 声道选择：接地 = 左声道
│  SD   ──────────→│  GPIO32 ← I2S 数据输出（麦克风 → ESP32）
│  WS   ──────────→│  GPIO33 ← I2S 字选择 / 左右时钟（LRCLK）
│  SCK  ──────────→│  GPIO25 ← I2S 位时钟（BCLK）
└──────────────────┘
```

**MS4030 各焊针说明**：

| 焊针 | 全称 / 含义 | 作用 | 本项目接法 |
|------|-------------|------|------------|
| VDD | Voltage Drain / 电源正极（等同 VCC） | 给麦克风芯片供电，工作电压约 1.8–3.3V | ESP32 3.3V |
| GND | Ground / 地 | 电源回路与信号参考地，必须与 ESP32 共地 | ESP32 GND |
| L/R | Left / Right Select / 声道选择 | 决定数据走左/右声道；接 GND=左，接 VDD=右 | GND（选左声道，与固件一致） |
| SD | Serial Data / 串行数据 | 麦克风把数字音频送给 ESP32（I2S DATA / DOUT） | GPIO32 |
| WS | Word Select / 字选择（也称 LRCLK） | 帧同步时钟，标识当前采样属左还是右声道，由 ESP32 提供 | GPIO33 |
| SCK | Serial Clock / 串行时钟（也称 BCLK） | I2S 位时钟，决定每一位何时采样，由 ESP32 提供 | GPIO25 |
| DC（可选） | 部分 7 针板才有 | 本方案不用，悬空即可 | 空（不接） |

**MAX98357 功放**：

常见模块丝印为 `LRC`（不是 `LRCLK`）——二者是同一根脚：Left/Right Clock，左右声道帧时钟。接到 ESP32 `GPIO27` 即可。

```
MAX98357 模块        ESP32 / 喇叭
┌──────────────────┐
│  VIN  ──────────→│  5V     ← 功放电源（注意不是 3.3V）
│  GND  ──────────→│  GND    ← 地
│  DIN  ──────────→│  GPIO14 ← I2S 数据输入（ESP32 → 功放）
│  BCLK ──────────→│  GPIO26 ← I2S 位时钟
│  LRC  ──────────→│  GPIO27 ← I2S 字选择 / 左右时钟（有的板丝印为 LRCLK）
│  GAIN ──────────→│  悬空   ← 增益脚，悬空默认约 9dB
│  SD   ──────────→│  5V     ← Shutdown 使能：接高电平才工作
│  SPK+ ──────────→│  喇叭红线
│  SPK- ──────────→│  喇叭黑线
└──────────────────┘
```

**MAX98357 各焊针说明**：

| 焊针 | 全称 / 含义 | 作用 | 本项目接法 |
|------|-------------|------|------------|
| VIN | Voltage In / 电源输入 | 给功放芯片和喇叭驱动供电 | ESP32 5V（不要接 3.3V） |
| GND | Ground / 地 | 电源与信号地，与 ESP32、喇叭共地 | ESP32 GND |
| DIN | Data In / 数据输入 | 接收 ESP32 发来的 I2S 数字音频 | GPIO14 |
| BCLK | Bit Clock / 位时钟 | I2S 位时钟（与麦克风侧 SCK 同类，但走独立引脚） | GPIO26 |
| LRC | Left/Right Clock / 左右时钟（等同 LRCLK） | 帧同步（与麦克风侧 WS 同类） | GPIO27 |
| GAIN | Gain / 增益选择 | 通过接法改变放大倍数；悬空约为 9dB | 悬空 |
| SD | Shutdown / 关断控制 | 低电平关断、高电平使能；不接高则无声 | 接 5V（与 VIN 同电位） |
| SPK+ | Speaker Positive / 喇叭正极 | 差分输出正极 | 喇叭红线 |
| SPK- | Speaker Negative / 喇叭负极 | 差分输出负极 | 喇叭黑线 |

**接线清单**：
- MS4030：6 根线（VDD/GND/L/R/SD/WS/SCK）
- MAX98357：信号与电源 5 根（VIN/GND/DIN/BCLK/LRC）+ SD 使能 1 根 + 喇叭 2 根

### 5.2 写音频测试代码

```cpp
#include <Arduino.h>
#include <driver/i2s.h>
#include <Wire.h>
#include <Adafruit_GFX.h>
#include <Adafruit_SSD1306.h>

#define SCREEN_WIDTH  128
#define SCREEN_HEIGHT 64
#define OLED_RESET    -1
#define OLED_ADDR    0x3C

Adafruit_SSD1306 display(SCREEN_WIDTH, SCREEN_HEIGHT, &Wire, OLED_RESET);

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
        .bck_io_num = 25,
        .ws_io_num = 33,
        .data_out_num = I2S_PIN_NO_CHANGE,
        .data_in_num = 32
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
        .bck_io_num = 26,
        .ws_io_num = 27,
        .data_out_num = 14,
        .data_in_num = I2S_PIN_NO_CHANGE
    };

    i2s_driver_install(I2S_NUM_1, &i2s_config, 0, NULL);
    i2s_set_pin(I2S_NUM_1, &pin_config);
}

// ============ 播放提示音（简单音调） ============
void playBeep() {
    // 播放一个 1kHz 的正弦波测试音（持续 500ms）
    const int sampleRate = 24000;
    const int duration = 500; // ms
    const int numSamples = sampleRate * duration / 1000;
    int16_t buffer[numSamples];

    for (int i = 0; i < numSamples; i++) {
        float t = (float)i / sampleRate;
        buffer[i] = (int16_t)(16000 * sin(2 * PI * 1000 * t));  // 1kHz
    }

    size_t written;
    i2s_write(I2S_NUM_1, buffer, sizeof(buffer), &written, portMAX_DELAY);
}

void setup() {
    Serial.begin(115200);

    // OLED
    Wire.begin(21, 22);
    display.begin(SSD1306_SWITCHCAPVCC, OLED_ADDR);
    display.clearDisplay();
    display.setTextColor(SSD1306_WHITE);
    display.setTextSize(2);
    display.setCursor(10, 10);
    display.println("TinyPal");
    display.setTextSize(1);
    display.setCursor(0, 35);
    display.println("Testing...");
    display.setCursor(0, 50);
    display.println("Mic + Speaker");
    display.display();

    // I2S 麦克风和功放
    setupI2SMic();
    setupI2SSpeaker();

    Serial.println("I2S initialized, playing beep...");
    playBeep();
    Serial.println("Beep played!");
}

void loop() {
    // 读取麦克风数据并打印音量（测试录音是否正常）
    uint8_t buffer[512];
    size_t bytesRead;

    esp_err_t result = i2s_read(I2S_NUM_0, buffer, sizeof(buffer),
                                 &bytesRead, 10 / portTICK_PERIOD_MS);

    if (result == ESP_OK && bytesRead > 0) {
        int16_t* samples = (int16_t*)buffer;
        int count = bytesRead / 2;
        long sum = 0;
        for (int i = 0; i < count; i++) {
            sum += abs(samples[i]);
        }
        int avg = sum / count;
        Serial.print("Mic volume: ");
        Serial.println(avg);

        // 显示音量条
        display.clearDisplay();
        display.setTextSize(1);
        display.setCursor(0, 10);
        display.println("TinyPal Audio Test");
        display.setCursor(0, 30);
        display.print("Mic: ");
        display.println(avg);

        int barWidth = map(constrain(avg, 0, 8000), 0, 8000, 0, 100);
        display.fillRect(0, 45, barWidth, 10, SSD1306_WHITE);
        display.display();
    }

    delay(50);
}
```

### 5.3 烧录后验证

**麦克风测试**：
- 对着 MS4030 说话或拍手
- 串口 Monitor 看到音量数值在跳动 ✅
- OLED 屏幕上的音量条在变化 ✅

**喇叭测试**：
- 上电后播放一声 "哔" ✅
- 如果没声音，检查 MAX98357 SD 脚是否接了 5V（使能端）
- 检查喇叭接线正负极是否正确

### 5.4 常见问题

| 问题 | 排查 |
|------|------|
| 没有声音 | MAX98357 SD 脚有没有接 5V？GAIN 脚有没有悬空？喇叭接线正负极？ |
| 声音很小 | VIN 是否在 5V？（不是 3.3V）3W 喇叭阻抗是否 4Ω？ |
| 麦克风录不到 | VDD 是否接到 3.3V（不是 5V）？L/R 是否接地？SD/WS/SCK 是否插紧？ |
| 录到全是杂音 / 电音 | 固件须 32-bit 读 MS4030 再右移（`MIC_PCM_SHIFT`）；杜邦线勿超过 30cm；回放时麦远离喇叭防啸叫 |

### 5.5 通过标准

- 喇叭能发出声音 ✅
- 麦克风能采集到音频数据（串口有音量输出）✅
- OLED 显示音量条 ✅

**通过 → 进入第 6 步**

---

## 第 6 步：烧录完整 TinyPal 固件 + 云端服务

### 6.1 完整接线确认

对照接线总图，确认所有模块接线完成：

```
□ TP4056 → 电池 ✅
□ TP4056 OUT → ESP32 VIN / GND ✅
□ ESP32 → OLED (SDA/SCL/VCC/GND) ✅
□ ESP32 → MS4030 (VDD/GND/L/R/SD/WS/SCK) ✅
□ ESP32 → MAX98357 (VIN/GND/DIN/BCLK/LRC/SD) ✅
□ MAX98357 → 喇叭 ✅
□ ESP32 USB → 电脑（烧录用）✅
□ TP4056 USB-C → 充电线（边用边充）✅
```

### 6.2 启动云端服务 + seed 设备

在跑 `tiny-bot-cloud-agent` 的机器上（本地或 `47.x.x.x` 服务器）：

```bash
cd tiny-bot-cloud-agent
make build-all   # 或已有 ./bin/tiny-bot-cloud-agent 与 ./bin/seed
./bin/seed -device-id tinypal-01 -pairing-code ABCD-1234
./bin/tiny-bot-cloud-agent   # 服务监听 :5678
```

**`seed` 做什么：** 在云端数据库里预登记一台设备（`device_id` + `pairing_code`）。  
这是「入场登记」，**不是**让板子立刻连上；板子上电后还会走 WiFi → `/provision` 拿 token → WebSocket。  
固件 `config.h` 的 `TB_DEVICE_ID` / `TB_PAIRING_CODE` 必须与 seed 参数一致。  
未 seed 时 OLED 会一直 `Reconnecting...`，串口常见 `device not found` / HTTP 401。

### 6.3 完整固件

修改 `tiny-bot-firmware/include/config.h`（WiFi、`HTTP_BASE`/`WS_HOST`、设备 ID/配对码）后：

```bash
cd tiny-bot-firmware
make main
make monitor   # 看串口：应出现 WiFi OK、provision 成功
```

### 6.4 最终验证

```
1. ESP32 上电
2. OLED 显示 Hold BOOT to talk（不再一直 Reconnecting）
3. 按住 BOOT 说话，松开
4. 云端 STT → LLM → TTS，喇叭播放 ✅
```

---

## 阶段检查清单

| 阶段 | 完成标准 | 状态 |
|------|---------|:---:|
| 第 1 步 | TP4056 OUT 输出 4.8V-5.2V | □ |
| 第 2 步 | ESP32 3.3V 正常，无短路 | □ |
| 第 3 步 | Blink 固件运行，LED 闪烁 | □ |
| 第 4 步 | OLED 显示文字正常 | □ |
| 第 5 步 | 喇叭发声 + 麦克风采集正常 | □ |
| 第 6 步 | 完整 TinyPal 运行，语音对话正常 | □ |

**6 步全部完成 → TinyPal 制作完成！** 🎉

---

*文档版本: 1.0 | 日期: 2026-05-13 | 分阶段接线测试计划*