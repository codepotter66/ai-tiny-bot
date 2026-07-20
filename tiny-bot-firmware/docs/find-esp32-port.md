# ESP32 串口端口查找指南

## macOS 查找方法

### 方法 1: ls 命令（推荐）

```bash
ls /dev/cu.* /dev/tty.* | grep -E "(usbserial|usbmodem|ttyACM)"
```

常见输出：
- `/dev/cu.usbserial-10`
- `/dev/cu.usbserial-110` (端口号可能变化！)
- `/dev/cu.usbmodem10`
- `/dev/tty.usbserial-10`

### ⚠️ 重要：macOS 端口号会变化！

**macOS 的 USB 串口编号不是固定的**，每次重新插拔或重启后，端口号可能会递增：
- 第一次：`/dev/cu.usbserial-10`
- 第二次：`/dev/cu.usbserial-110`
- 第三次：`/dev/cu.usbserial-210`
- 以此类推...

**每次烧录前都要重新检查端口**：
```bash
# 每次烧录前执行
ls /dev/cu.usb*
# 如果端口变了，更新 platformio.ini
sed -i '' 's|upload_port = .*|upload_port = /dev/cu.usbserial-110|' platformio.ini
```

### 方法 2: 使用 python（自动检测）

```bash
python3 -c "import glob; print('\n'.join(sorted(glob.glob('/dev/cu.usb*'))))"
```

### 方法 3: 使用 system_profiler

```bash
system_profiler SPUSBDataType | grep -A 5 "CP2102\|CH340\|FTDI"
```

## Linux 查找方法

```bash
ls -l /dev/ttyUSB* /dev/ttyACM* /dev/ttyUSB* 2>/dev/null
# 或
dmesg | grep tty
```

常见输出：`/dev/ttyUSB0`, `/dev/ttyACM0`

## Windows 查找方法

```powershell
# 方法 1: 使用 device manager
devmgmt.msc

# 方法 2: 使用 mode 命令
mode

# 方法 3: 使用 COM 端口列表
[System.IO.Ports.SerialPort]::GetPortNames()
```

## 自动检测脚本

创建一个 `find-esp32.sh` 脚本：

```bash
#!/bin/bash
echo "=== 查找 ESP32 串口 ==="

# macOS
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "macOS 检测到..."
    ports=$(ls /dev/cu.usb* 2>/dev/null)
    if [ -z "$ports" ]; then
        echo "未找到 USB 串口设备"
    else
        echo "可用串口："
        echo "$ports"
        # 自动选择第一个
        first=$(echo "$ports" | head -1)
        echo ""
        echo "自动选择: $first"
        echo "export ESPI_PORT=$first" >> ~/.bash_profile
        echo "已添加到 ~/.bash_profile"
    fi
fi

# Linux
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    echo "Linux 检测到..."
    ports=$(ls /dev/ttyUSB* /dev/ttyACM* 2>/dev/null)
    if [ -z "$ports" ]; then
        echo "未找到 USB 串口设备"
    else
        echo "可用串口："
        echo "$ports"
    fi
fi
```

使用方法：
```bash
chmod +x find-esp32.sh
./find-esp32.sh
```

## PlatformIO 配置

找到端口后，在 `platformio.ini` 中配置：

```ini
[env:esp32dev]
upload_port = /dev/cu.usbserial-10   # macOS
# upload_port = /dev/ttyUSB0          # Linux
# upload_port = COM3                  # Windows
```

## 常见问题

### 问题 1: 找不到端口

可能原因：
1. USB 线只充电不能传输数据（换一根线）
2. ESP32 驱动未安装
3. USB 权限不足

解决：
```bash
# Linux 添加权限
sudo chmod 666 /dev/ttyUSB0

# macOS 检查驱动
brew list | grep -i ch340  # 可能有驱动问题
```

### 问题 2: 权限被拒绝

```bash
# Linux
sudo usermod -a -G dialout $USER
# 然后重新登录
```

### 问题 3: 端口被占用

```bash
# 查找占用进程
lsof /dev/cu.usbserial-10

# 杀死占用进程
kill -9 <PID>
```

## 一键更新 platformio.ini

找到端口后，自动更新配置：

```bash
# 假设找到 /dev/cu.usbserial-10
sed -i '' 's|upload_port = .*|upload_port = /dev/cu.usbserial-10|' platformio.ini
```

或直接编辑 `platformio.ini`：
```ini
upload_port = /dev/cu.usbserial-10
```

## 验证连接

确认 ESP32 正常连接：

```bash
# 查看串口信息
screen /dev/cu.usbserial-10 115200
# 按 Ctrl+A 然后输入 :quit 退出 screen

# 或使用 minicom
minicom -D /dev/cu.usbserial-10 -b 115200
```

## 参考

- [PlatformIO Upload 文档](https://docs.platformio.org/page/usb载.html)
- [ESP32 USB 驱动](https://docs.espressif.com/projects/esp-idf/en/latest/esp32/hw-reference/esp32/get-started-usb-uart.html)