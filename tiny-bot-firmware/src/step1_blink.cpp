/**
 * ============================================================
 * Step 1: Blink 测试固件
 * ============================================================
 * 目标: 验证 ESP32 开发板正常工作
 * 内容: LED 每秒闪烁一次，串口输出状态
 * 硬件: ESP32 Dev Module（内置 LED 在 GPIO2）
 * ============================================================
 *
 * 如何使用:
 * 1. 用 USB 线连接 ESP32 到电脑
 * 2. 在 VS Code 中打开 PlatformIO 插件
 * 3. 打开此文件，点击底部 "→ Upload" 烧录
 * 4. 烧录完成后，打开 Serial Monitor (115200 波特率)
 * 5. 看到 "LED ON / LED OFF" 交替打印，小蓝灯每秒闪烁一次
 *
 * 通过标准:
 * - 内置 LED 每秒闪烁一次
 * - 串口输出正常
 * ============================================================
 */

#include <Arduino.h>

// ESP32 开发板内置 LED 通常在 GPIO2
// 部分开发板可能不同，请参考你手上的开发板文档
#define LED_PIN 2

void setup() {
    // 初始化 LED 引脚为输出模式
    pinMode(LED_PIN, OUTPUT);

    // 初始化串口通信，用于调试输出
    // 115200 波特率是 ESP32 常用的默认速率
    Serial.begin(115200);

    // 等待串口连接稳定（确保 Serial Monitor 能看到输出）
    delay(1000);

    // 打印开始信息
    Serial.println("========================================");
    Serial.println("TinyBot Firmware - Step 1: Blink Test");
    Serial.println("========================================");
    Serial.println("ESP32 is running!");
    Serial.println("LED should blink every 1 second.");
    Serial.println("========================================");
}

void loop() {
    // 点亮 LED
    digitalWrite(LED_PIN, HIGH);
    Serial.println("LED ON");
    delay(1000);  // 延时 1000ms = 1秒

    // 熄灭 LED
    digitalWrite(LED_PIN, LOW);
    Serial.println("LED OFF");
    delay(1000);
}

/**
 * 知识点解释:
 *
 * 1. setup() vs loop()
 *    - setup(): 开发板上电后只执行一次，用于初始化
 *    - loop(): 初始化完成后不断循环执行
 *    - 这是 Arduino 框架的核心设计模式
 *
 * 2. pinMode(pin, mode)
 *    - pin: GPIO 引脚编号
 *    - mode: INPUT(输入) / OUTPUT(输出) / INPUT_PULLUP(上拉输入)
 *    - 数字引脚作为输出前必须先设置模式
 *
 * 3. digitalWrite(pin, value)
 *    - 给引脚写入 HIGH(高电平/3.3V) 或 LOW(低电平/0V)
 *    - HIGH 时 LED点亮（部分开发板相反，视电路设计而定）
 *
 * 4. delay(ms)
 *    - 延时函数，单位毫秒
 *    - 1000ms = 1秒
 *    - 延时期间 CPU 无法处理其他任务
 *
 * 5. Serial.begin(115200)
 *    - 初始化串口通信
 *    - 115200 是与 Serial Monitor 通信的速率
 *    - 双方速率必须匹配才能正常通信
 */