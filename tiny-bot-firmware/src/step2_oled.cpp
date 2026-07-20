/**
 * ============================================================
 * Step 2: OLED 屏幕测试固件
 * ============================================================
 * 目标: 验证 OLED 屏幕 (SSD1306 128x64) 正常工作
 * 内容: 在屏幕上显示文字和简单图形
 * 硬件: ESP32 + SSD1306 OLED (I2C 接口)
 * 引脚: SDA=GPIO21, SCL=GPIO22
 * ============================================================
 *
 * 接线:
 *   OLED VCC  → ESP32 3.3V
 *   OLED GND  → ESP32 GND
 *   OLED SDA  → ESP32 GPIO21
 *   OLED SCL  → ESP32 GPIO22
 *
 * 如何使用:
 * 1. 先完成 Step 1 (Blink) 确保开发板正常
 * 2. 按上述接线连接 OLED 屏幕
 * 3. 烧录此固件
 * 4. OLED 屏幕应显示 "TinyPal" 和 "Hello World!"
 *
 * 通过标准:
 * - OLED 屏幕正常亮起
 * - 显示文字清晰可见
 * - 串口输出 "OLED init OK!"
 * ============================================================
 */

#include <Arduino.h>
#include <Wire.h>                    // I2C 通信库
#include <Adafruit_GFX.h>            // 图形库（OLED 的基础）
#include <Adafruit_SSD1306.h>        // SSD1306 OLED 驱动库

// ----------------------------------------------------------
// OLED 屏幕配置
// ----------------------------------------------------------
// 屏幕分辨率（SSD1306 常见分辨率）
#define SCREEN_WIDTH  128  // 像素宽度
#define SCREEN_HEIGHT 64   // 像素高度
// 复位引脚设置为 -1 表示不使用硬件复位（软件复位）
#define OLED_RESET    -1
// I2C 地址，SSD1306 默认一般是 0x3C，也有的是 0x3D
#define OLED_ADDR    0x3C

// ----------------------------------------------------------
// 创建 OLED 显示对象
// ----------------------------------------------------------
// Adafruit_SSD1306 需要屏幕宽高、Wire 对象指针、复位引脚
Adafruit_SSD1306 display(SCREEN_WIDTH, SCREEN_HEIGHT, &Wire, OLED_RESET);

void setup() {
    // 初始化串口（用于调试输出）
    Serial.begin(115200);
    Serial.println("========================================");
    Serial.println("TinyBot Firmware - Step 2: OLED Test");
    Serial.println("========================================");

    // ----------------------------------------------------------
    // 初始化 I2C 通信
    // ----------------------------------------------------------
    // ESP32 的 I2C 引脚：
    // - GPIO21: SDA (数据线)
    // - GPIO22: SCL (时钟线)
    // Wire.begin() 使用默认 I2C 总线
    Wire.begin(21, 22);

    // ----------------------------------------------------------
    // 初始化 OLED 屏幕
    // ----------------------------------------------------------
    // SSD1306_SWITCHCAPVCC = 使用内部升压供电
    // OLED_ADDR = I2C 从机地址（0x3C）
    if (!display.begin(SSD1306_SWITCHCAPVCC, OLED_ADDR)) {
        // 初始化失败
        Serial.println("ERROR: OLED init FAILED!");
        Serial.println("Please check:");
        Serial.println("  1. OLED VCC -> 3.3V");
        Serial.println("  2. OLED GND -> GND");
        Serial.println("  3. OLED SDA -> GPIO21");
        Serial.println("  4. OLED SCL -> GPIO22");
        // 初始化失败时进入死循环，防止继续运行
        while (true) {
            delay(1000);
        }
    }

    Serial.println("OLED init OK!");
    Serial.println("Displaying content...");

    // ----------------------------------------------------------
    // 配置显示内容
    // ----------------------------------------------------------
    // 清屏
    display.clearDisplay();

    // 设置绘制颜色（SSD1306 是单色屏幕，只有白色）
    // SSD1306_WHITE = 像素点亮（显示内容）
    // SSD1306_BLACK = 像素熄灭（背景）
    display.setTextColor(SSD1306_WHITE);

    // 设置文字大小 (1=最小，越来越大)
    display.setTextSize(2);

    // 设置光标位置（x, y 坐标）
    display.setCursor(20, 25);

    // 打印第一行文字
    display.println("TinyPal");

    // 设置较小的文字
    display.setTextSize(1);
    display.setCursor(30, 50);
    display.println("Hello World!");

    // 将上述内容推送到屏幕显示
    // 注意：每次修改显示内容后都需要调用 display.display()
    display.display();

    Serial.println("Display done!");
    Serial.println("========================================");
}

void loop() {
    // Step 2 不需要 loop，屏幕静态显示即可
    // 但可以添加一些简单的动画效果

    delay(50);
}

/**
 * 知识点解释:
 *
 * 1. I2C 通信
 *    - I2C 是两线式串行通信协议（SDA + SCL）
 *    - 多个设备可以共享同一组总线
 *    - 每个设备有唯一的地址（这里 OLED 是 0x3C）
 *
 * 2. Adafruit 库
 *    - Adafruit_GFX: 通用图形库，提供画点、线、圆、文字等函数
 *    - Adafruit_SSD1306: 针对 SSD1306 OLED 屏幕的具体实现
 *    - 这两个库配合使用，先有 GFX 才有 SSD1306
 *
 * 3. 屏幕坐标系统
 *    - 左上角是 (0, 0)
 *    - x 向右增加，y 向下增加
 *    - 128x64 的屏幕，x 范围 0-127，y 范围 0-63
 *
 * 4. setTextSize()
 *    - 文字大小，1 是最小（约 6x8 像素）
 *    - 每增加 1，文字宽高各增加约 6x8 像素
 *    - setTextSize(2) 大约是 12x16 像素
 *
 * 5. display.display()
 *    - 这是一个非常常见的错误源
 *    - 所有的绘制操作（println、drawLine 等）都是先写入缓冲区
 *    - 必须调用 display.display() 才会真正发送到屏幕
 *    - 如果屏幕不显示，首先检查是否调用了 display.display()
 */