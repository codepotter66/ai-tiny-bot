# 测试说明

[English](README.md)

## PlatformIO 测试

PlatformIO 没有传统的单元测试框架，但可以通过以下方式进行功能验证：

### 1. 烧录测试固件

```bash
pio run --target upload
pio device monitor --baud 115200
```

### 2. 串口输出验证

每个测试固件都会通过 `Serial.println()` 输出状态信息：

- **step1_blink.cpp**: 输出 "LED ON" / "LED OFF"
- **step2_oled.cpp**: 输出 "OLED init OK!" 或错误信息
- **step3_audio.cpp**: 输出麦克风音量数值

### 3. 观察硬件行为

- **step1**: ESP32 小蓝灯每秒闪烁一次
- **step2**: OLED 屏幕显示 "TinyPal" 和 "Hello World!"
- **step3**: 喇叭发出 "哔" 声，音量条实时显示

### 4. 测试顺序

建议按顺序完成所有测试，确保每个阶段都通过后再进行下一步。

详见 [Step-by-Step Build Guide](../../../docs/step-by-step-build-guide.md)。
