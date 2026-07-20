# Tests

[中文文档](README.zh-CN.md)

PlatformIO does not ship a classic unit-test harness here; validate with staged firmwares.

### 1. Flash a test binary

```bash
pio run --target upload
pio device monitor --baud 115200
```

### 2. Serial checks

Each stage prints status via `Serial.println()`:

- **step1_blink.cpp**: `LED ON` / `LED OFF`
- **step2_oled.cpp**: `OLED init OK!` or errors
- **step3_audio.cpp**: mic level numbers

### 3. Hardware behavior

- **step1**: blue LED blinks once per second
- **step2**: OLED shows TinyPal / Hello World
- **step3**: speaker beep + live level bar

### 4. Order

Finish each stage before the next. See the [step-by-step build guide](../../../docs/step-by-step-build-guide.md).
