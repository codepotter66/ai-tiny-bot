/**
 * ============================================================
 * Step 3: 音频模块测试固件（麦克风 + 功放）
 * ============================================================
 * 目标: 验证麦克风采集和功放播放正常工作
 * 内容: 启动提示音 + 麦克风实时回放到喇叭，OLED 显示音量
 * 硬件: ESP32 + MS4030 I2S 麦克风 + MAX98357 I2S 功放
 * ============================================================
 *
 * 接线:
 *
 * 麦克风 MS4030 (I2S 输入):
 *   VCC  → ESP32 3.3V
 *   GND  → ESP32 GND
 *   L/R  → ESP32 GND  (选择左声道，接地)
 *   DC   → 空（不用接）
 *   SD   → ESP32 GPIO32 (I2S 数据输入)
 *   WS   → ESP32 GPIO33 (字选择)
 *   SCK  → ESP32 GPIO25 (串行时钟)
 *
 * 功放 MAX98357 (I2S 输出):
 *   VIN  → ESP32 5V  (注意是 5V 不是 3.3V！)
 *   GND  → ESP32 GND
 *   DIN  → ESP32 GPIO14 (I2S 数据输出)
 *   BCLK → ESP32 GPIO26 (位时钟)
 *   LRCLK→ ESP32 GPIO27 (字选择/左 右时钟)
 *   GAIN → 悬空（默认增益 9dB）
 *   SD   → ESP32 5V  (使能脚，接高电平启用)
 *   SPK+ → 喇叭红线
 *   SPK- → 喇叭黑线
 *
 * 如何使用:
 * 1. 先完成 Step 1 和 Step 2
 * 2. 按上述接线连接麦克风和功放
 * 3. 烧录此固件
 * 4. 启动时先听到一声「哔」
 * 5. 对着麦克风说话，喇叭应实时回放；OLED 显示音量条
 * 6. 麦克风尽量别正对喇叭，否则容易啸叫
 *
 * 注意:
 * - 实时回放时麦克风尽量远离喇叭，避免啸叫。
 *
 * 通过标准:
 * - 喇叭能发出启动提示音
 * - 对着麦克风说话，喇叭能实时听到自己的声音
 * - OLED / 串口显示音量数值
 * ============================================================
 */

#include <Arduino.h>
#include <driver/i2s.h>             // ESP32 I2S 驱动
#include <Wire.h>                    // I2C（用于 OLED）
#include <Adafruit_GFX.h>            // 图形库
#include <Adafruit_SSD1306.h>        // OLED 驱动

// ============================================================
// OLED 配置
// ============================================================
#define SCREEN_WIDTH  128
#define SCREEN_HEIGHT 64
#define OLED_RESET    -1
#define OLED_ADDR    0x3C

Adafruit_SSD1306 display(SCREEN_WIDTH, SCREEN_HEIGHT, &Wire, OLED_RESET);

// ============================================================
// I2S 麦克风配置 (MS4030)
// ============================================================
// I2S_NUM_0 = I2S 总线 0（用于麦克风输入）
#define I2S_MIC_CH     I2S_NUM_0
#define I2S_MIC_SCK    25   // 串行时钟 (BCLK)
#define I2S_MIC_WS     33   // 字选择 (L/RCLK)
#define I2S_MIC_SD     32   // 串行数据 (DIN)
#define I2S_MIC_RATE   16000  // 采样率 16kHz（适合语音）

// ============================================================
// I2S 功放配置 (MAX98357)
// ============================================================
// I2S_NUM_1 = I2S 总线 1（用于喇叭输出）
#define I2S_SPK_CH     I2S_NUM_1
#define I2S_SPK_DIN    14   // 串行数据输入到功放
#define I2S_SPK_BCLK   26   // 位时钟
#define I2S_SPK_LRCLK  27   // 字选择
#define I2S_SPK_RATE   16000  // 与麦克风同采样率，便于实时回放

// ============================================================
// 音频缓冲区 / 回放增益
// ============================================================
#define MIC_BUFFER_SIZE 512   // 麦克风缓冲区大小（字节）
#define LOOPBACK_GAIN   2     // 回放增益；太大易啸叫，可改为 1

// ============================================================
// I2S 麦克风初始化
// ============================================================
void setupI2SMic() {
    Serial.println("Setting up I2S microphone...");

    i2s_config_t i2s_config = {
        .mode = (i2s_mode_t)(I2S_MODE_MASTER | I2S_MODE_RX),
        .sample_rate = I2S_MIC_RATE,
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

    esp_err_t err = i2s_driver_install(I2S_MIC_CH, &i2s_config, 0, NULL);
    if (err != ESP_OK) {
        Serial.print("I2S mic driver install failed: ");
        Serial.println(esp_err_to_name(err));
        return;
    }

    err = i2s_set_pin(I2S_MIC_CH, &pin_config);
    if (err != ESP_OK) {
        Serial.print("I2S mic pin config failed: ");
        Serial.println(esp_err_to_name(err));
        return;
    }

    i2s_zero_dma_buffer(I2S_MIC_CH);
    Serial.println("I2S microphone ready!");
}

// ============================================================
// I2S 功放初始化
// ============================================================
void setupI2SSpeaker() {
    Serial.println("Setting up I2S speaker...");

    // I2S 配置结构体
    i2s_config_t i2s_config = {
        .mode = (i2s_mode_t)(I2S_MODE_MASTER | I2S_MODE_TX),  // 主模式 + 发送
        .sample_rate = I2S_SPK_RATE,                          // 采样率
        .bits_per_sample = I2S_BITS_PER_SAMPLE_16BIT,         // 16bit 采样
        .channel_format = I2S_CHANNEL_FMT_ONLY_LEFT,          // 只输出左声道
        .communication_format = I2S_COMM_FORMAT_STAND_I2S,    // 标准 I2S 格式
        .intr_alloc_flags = ESP_INTR_FLAG_LEVEL1,            // 中断优先级
        .dma_buf_count = 8,                                   // DMA 缓冲区数量
        .dma_buf_len = 64,                                    // DMA 缓冲区长度（约 4ms @ 16kHz）
        .use_apll = false,                                    // 不使用 APLL 时钟
        .tx_desc_auto_clear = true,                          // 自动清除发送描述符
        .fixed_mclk = 0                                      // 固定 MCLK 时钟
    };

    // 引脚配置结构体
    i2s_pin_config_t pin_config = {
        .bck_io_num = I2S_SPK_BCLK,    // 位时钟
        .ws_io_num = I2S_SPK_LRCLK,    // 字选择
        .data_out_num = I2S_SPK_DIN,   // 功放数据输入
        .data_in_num = I2S_PIN_NO_CHANGE   // 不读取数据
    };

    // 安装 I2S 驱动
    esp_err_t err = i2s_driver_install(I2S_SPK_CH, &i2s_config, 0, NULL);
    if (err != ESP_OK) {
        Serial.print("I2S speaker driver install failed: ");
        Serial.println(esp_err_to_name(err));
        return;
    }

    // 配置 I2S 引脚
    err = i2s_set_pin(I2S_SPK_CH, &pin_config);
    if (err != ESP_OK) {
        Serial.print("I2S speaker pin config failed: ");
        Serial.println(esp_err_to_name(err));
        return;
    }

    Serial.println("I2S speaker ready!");
}

// ============================================================
// 播放提示音（测试功放是否正常工作）
// ============================================================
void playBeep() {
    Serial.println("Playing beep sound...");

    // 参数配置（分块写入，避免一次在栈上分配上万样本导致栈溢出）
    const int sampleRate = I2S_SPK_RATE;   // 采样率
    const int frequency = 1000;            // 频率 1kHz
    const int durationMs = 500;            // 持续时间 500ms
    const int chunkSamples = 256;
    int16_t buffer[chunkSamples];
    const int totalSamples = sampleRate * durationMs / 1000;
    int sampleIndex = 0;

    while (sampleIndex < totalSamples) {
        int n = totalSamples - sampleIndex;
        if (n > chunkSamples) {
            n = chunkSamples;
        }

        for (int i = 0; i < n; i++) {
            float t = (float)(sampleIndex + i) / sampleRate;
            buffer[i] = (int16_t)(16000 * sin(2 * PI * frequency * t));
        }

        size_t bytesWritten = 0;
        esp_err_t err = i2s_write(
            I2S_SPK_CH, buffer, n * sizeof(int16_t), &bytesWritten, portMAX_DELAY);
        if (err != ESP_OK) {
            Serial.print("I2S write failed: ");
            Serial.println(esp_err_to_name(err));
            return;
        }
        sampleIndex += n;
    }

    Serial.println("Beep played!");
}

// ============================================================
// 主程序入口
// ============================================================
void setup() {
    // 初始化串口
    Serial.begin(115200);
    Serial.println("========================================");
    Serial.println("TinyBot Firmware - Step 3: Audio Test");
    Serial.println("========================================");

    // ----------------------------------------------------------
    // 初始化 OLED
    // ----------------------------------------------------------
    Wire.begin(21, 22);
    if (!display.begin(SSD1306_SWITCHCAPVCC, OLED_ADDR)) {
        Serial.println("ERROR: OLED init FAILED!");
        while (true) delay(1000);
    }
    Serial.println("OLED init OK!");

    // 显示初始界面
    display.clearDisplay();
    display.setTextColor(SSD1306_WHITE);
    display.setTextSize(2);
    display.setCursor(10, 10);
    display.println("TinyPal");
    display.setTextSize(1);
    display.setCursor(0, 35);
    display.println("Audio Test...");
    display.setCursor(0, 50);
    display.println("Mic + Speaker");
    display.display();

    // ----------------------------------------------------------
    // 初始化 I2S 音频模块
    // ----------------------------------------------------------
    setupI2SMic();
    setupI2SSpeaker();

    // 播放提示音测试功放（启动时应听到一声「哔」）
    delay(200);  // 等 I2S 时钟稳定再写数据
    playBeep();

    Serial.println("========================================");
    Serial.println("Audio loopback running!");
    Serial.println("Speak into the mic — speaker should play it live.");
    Serial.println("Keep mic away from speaker to reduce feedback.");
    Serial.println("========================================");
}

void loop() {
    static uint32_t lastUiMs = 0;
    int16_t micBuffer[MIC_BUFFER_SIZE / sizeof(int16_t)];
    size_t bytesRead = 0;

    // 16-bit 直读 → 写喇叭（已验证可回放说话；杂音问题另议）
    esp_err_t err = i2s_read(
        I2S_MIC_CH, micBuffer, sizeof(micBuffer), &bytesRead, 20 / portTICK_PERIOD_MS);
    if (err != ESP_OK || bytesRead == 0) {
        return;
    }

    int sampleCount = (int)(bytesRead / sizeof(int16_t));
    long sum = 0;
    for (int i = 0; i < sampleCount; i++) {
        int32_t s = (int32_t)micBuffer[i] * LOOPBACK_GAIN;
        if (s > 32767) {
            s = 32767;
        } else if (s < -32768) {
            s = -32768;
        }
        micBuffer[i] = (int16_t)s;
        sum += abs(micBuffer[i]);
    }
    int average = sampleCount > 0 ? (int)(sum / sampleCount) : 0;

    size_t bytesWritten = 0;
    err = i2s_write(
        I2S_SPK_CH, micBuffer, bytesRead, &bytesWritten, 20 / portTICK_PERIOD_MS);
    if (err != ESP_OK) {
        Serial.print("I2S write failed: ");
        Serial.println(esp_err_to_name(err));
    }

    uint32_t now = millis();
    if (now - lastUiMs < 100) {
        return;
    }
    lastUiMs = now;

    Serial.print("Mic volume: ");
    Serial.println(average);

    display.clearDisplay();
    display.setTextSize(1);
    display.setCursor(0, 0);
    display.println("TinyPal Loopback");
    display.setCursor(0, 16);
    display.println("Mic -> Speaker live");
    display.setCursor(0, 32);
    display.print("Mic: ");
    display.println(average);
    int barWidth = map(constrain(average, 0, 8000), 0, 8000, 0, 100);
    display.fillRect(0, 48, barWidth, 10, SSD1306_WHITE);
    display.display();
}

/**
 * 知识点解释:
 *
 * 1. 实时回放：麦克风与功放统一 16kHz，读到的 16-bit 样本直接写出
 * 2. OLED/串口降频刷新，避免 display() 拖慢音频
 * 3. 麦远离喇叭可减少啸叫；LOOPBACK_GAIN 过大也会加重啸叫
 */