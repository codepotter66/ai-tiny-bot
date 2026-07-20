# PlatformIO CLI 常用命令

PlatformIO 是一个跨平台的嵌入式开发工具命令行接口。

## 项目初始化和编译

```bash
# 编译项目（首次会下载框架和库）
pio run

# 编译指定环境（platformio.ini 中定义的环境）
pio run -e blink       # 编译 blink 环境
pio run -e oled       # 编译 oled 环境
pio run -e audio      # 编译 audio 环境

# 编译并烧录到硬件
pio run -t upload

# 指定环境编译+烧录
pio run -e blink -t upload
```

## 串口监视

```bash
# 打开串口监视器（查看 Serial.print 输出）
pio device monitor

# 指定波特率
pio device monitor --baud 115200

# 刷新监视器
pio device monitor --baud 115200 --once
```

## 烧录相关

```bash
# 仅烧录固件（不编译）
pio run -t upload

# 擦除芯片再烧录
pio run -t upload --erase

# 上传文件系统数据（如 SPIFFS/LittleFS）
pio run -t uploadfs
```

## 清理和构建

```bash
# 清理编译产物
pio run --target clean

# 查看固件大小
pio run --target size

# 显示编译固件的详细信息
pio run --target sizes
```

## 库管理

```bash
# 安装库（可指定版本）
pio lib install "adafruit/Adafruit SSD1306 @ ^2.5"

# 搜索库
pio lib search "ssd1306"

# 列出已安装的库
pio lib list

# 查看库详情
pio lib show "adafruit/Adafruit SSD1306"

# 卸载库
pio lib uninstall "adafruit/Adafruit SSD1306"
```

## 项目管理

```bash
# 新建项目（交互式）
pio project init

# 新建项目（指定参数）
pio project init --board esp32dev --framework arduino

# 查看项目数据
pio project data
```

## Boards 和 Platforms

```bash
# 查看已安装的平台
pio platform list

# 安装平台
pio platform install espressif32

# 搜索平台
pio platform search "esp32"

# 查看支持的开发板
pio boards esp32dev

# 查看平台详情
pio platform show espressif32
```

## 环境管理

```bash
# 列出所有环境
pio env list

# 显示环境详细信息
pio env show

# 编译所有环境
pio compile --all
```

## 其他常用

```bash
# 运行单元测试（如果有 test/ 目录）
pio test

# 远程操作（如通过民间 IoT 框架烧录）
pio remote run

# 更新 PlatformIO 核心
pio upgrade

# 更新平台和库
pio upgrade --update-only
```

## 常用场景

### 1. 日常开发流程

```bash
# 1. 写代码
# 2. 编译检查
pio run

# 3. 烧录到硬件
pio run -t upload

# 4. 打开串口看输出
pio device monitor --baud 115200
```

### 2. 分环境开发

```bash
# 编译并烧录 Blink 测试
pio run -e blink -t upload

# 编译并烧录 OLED 测试
pio run -e oled -t upload

# 编译并烧录音频测试
pio run -e audio -t upload
```

### 3. 清理重编译

```bash
# 清理旧编译产物
rm -rf .pio

# 重新编译（强制重新下载依赖）
pio run
```

### 4. 快速查看固件大小

```bash
pio run --target size
# 输出类似：
# .pio/build/esp32dev/firmware.elf
#-section size
# .text   .data    .bss
# 383920   12356   81440
```

## Makefile 封装

本项目使用 Makefile 简化操作：

```bash
make help      # 显示所有可用命令
make build    # 编译
make upload   # 烧录
make monitor  # 串口监视
make clean    # 清理

make blink    # 编译并烧录 Blink 测试
make oled     # 编译并烧录 OLED 测试
make audio    # 编译并烧录音频测试
make main     # 编译并烧录主固件
```

## 故障排查

```bash
# 查看详细编译输出（调试问题）
pio run --verbose

# 强制重新安装依赖
pio lib install --force "adafruit/Adafruit SSD1306"

# 更新所有平台和库
pio upgrade
pio platform update
pio lib update
```

## 参考链接

- [PlatformIO 官方文档](https://docs.platformio.org/)
- [PlatformIO CLI 参考](https://docs.platformio.org/en/latest/core.html)