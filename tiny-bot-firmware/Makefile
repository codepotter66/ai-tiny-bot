# ============================================================
# TinyBot Firmware - Makefile for PlatformIO
# ============================================================
# 提供便捷命令来编译、烧录和测试 ESP32 固件
#
# 用法:
#   make help      - 显示所有可用命令
#   make build     - 编译当前默认固件
#   make upload   - 烧录到 ESP32
#   make monitor  - 打开串口监视器
#   make clean    - 清理编译产物
#
# 分阶段测试:
#   make blink    - 编译并烧录 Blink 测试
#   make oled     - 编译并烧录 OLED 测试
#   make audio    - 编译并烧录音频测试
#   make main     - 编译并烧录完整固件
# ============================================================

# PlatformIO 命令：优先 PATH，否则回退到本机常见安装路径
PIO := $(shell command -v pio 2>/dev/null || command -v platformio 2>/dev/null || true)
ifeq ($(PIO),)
  PIO := $(wildcard $(HOME)/.platformio/penv/bin/pio)
endif
ifeq ($(PIO),)
  $(error 未找到 PlatformIO CLI (pio)。请安装 VS Code PlatformIO 扩展，或执行: curl -fsSL https://raw.githubusercontent.com/platformio/platformio-core-installer/master/install.sh | sh)
endif

# ============================================================
# 默认目标：显示帮助
# ============================================================
.PHONY: help
help:
	@echo ""
	@echo "TinyBot Firmware - 可用命令:"
	@echo ""
	@echo "  编译和烧录:"
	@echo "    make build       - 编译默认固件"
	@echo "    make upload     - 烧录到 ESP32"
	@echo "    make monitor    - 打开串口监视器 (115200)"
	@echo "    make clean       - 清理编译产物"
	@echo ""
	@echo "  分阶段测试固件:"
	@echo "    make blink       - 编译并烧录 Blink 测试固件"
	@echo "    make oled        - 编译并烧录 OLED 测试固件"
	@echo "    make audio       - 编译并烧录音频测试固件"
	@echo "    make main        - 编译并烧录完整固件"
	@echo ""
	@echo "  Wokwi 模拟:"
	@echo "    make wokwi       - 打开 Wokwi 在线模拟器"
	@echo ""
	@echo "  其他:"
	@echo "    make size        - 查看固件大小"
	@echo ""
	@echo "  PlatformIO 原生命令（高级用户）:"
	@echo "    $(PIO) run -e blink        - 编译 Blink 环境"
	@echo "    $(PIO) run -e oled         - 编译 OLED 环境"
	@echo "    $(PIO) run -e audio        - 编译 Audio 环境"
	@echo "    $(PIO) run -e main         - 编译 Main 环境"
	@echo ""

# ============================================================
# 编译相关
# ============================================================
.PHONY: build
build:
	@echo "编译 TinyBot 固件..."
	$(PIO) run

.PHONY: clean
clean:
	@echo "清理编译产物..."
	$(PIO) run --target clean
	rm -rf .pio

# ============================================================
# 烧录
# ============================================================
.PHONY: upload
upload:
	@echo "烧录固件到 ESP32..."
	$(PIO) run --target upload

# ============================================================
# 串口监视
# ============================================================
.PHONY: monitor
monitor:
	@echo "打开串口监视器 (115200 波特率)..."
	@echo "按 Ctrl+C 退出"
	$(PIO) device monitor

# ============================================================
# 固件大小分析
# ============================================================
.PHONY: size
size:
	@echo "固件大小分析..."
	$(PIO) run --target size

# ============================================================
# 分阶段测试固件
# -----------------------------------------------------------
# 使用 PlatformIO 的多环境功能
# 每个环境在 platformio.ini 中定义不同的 src_filter
# ============================================================

# Step 1: Blink 测试
.PHONY: blink
blink:
	@echo "编译 Blink 测试固件..."
	$(PIO) run -e blink
	@echo ""
	@echo "烧录到 ESP32..."
	$(PIO) run -e blink --target upload
	@echo ""
	@echo "Blink 测试完成！小蓝灯应每秒闪烁一次"

# Step 2: OLED 测试
.PHONY: oled
oled:
	@echo "编译 OLED 测试固件..."
	$(PIO) run -e oled
	@echo ""
	@echo "烧录到 ESP32..."
	$(PIO) run -e oled --target upload
	@echo ""
	@echo "OLED 测试完成！屏幕应显示 TinyPal 和 Hello World!"

# Step 3: 音频测试
.PHONY: audio
audio:
	@echo "编译音频测试固件..."
	$(PIO) run -e audio
	@echo ""
	@echo "烧录到 ESP32..."
	$(PIO) run -e audio --target upload
	@echo ""
	@echo "音频测试完成！应听到提示音，麦克风音量在串口输出"

# 主固件
.PHONY: main
main:
	@echo "编译主固件..."
	$(PIO) run -e main
	@echo ""
	@echo "烧录到 ESP32..."
	$(PIO) run -e main --target upload
	@echo ""
	@echo "主固件烧录完成！"

# ============================================================
# Wokwi 模拟
# ============================================================
.PHONY: wokwi
wokwi:
	@echo "打开 Wokwi 在线模拟器..."
	@echo ""
	@echo "使用方法:"
	@echo "1. 访问 https://wokwi.com"
	@echo "2. 创建新项目，导入 diagram.json"
	@echo "3. 或使用 VSCode Wokwi 扩展"
	@open https://wokwi.com