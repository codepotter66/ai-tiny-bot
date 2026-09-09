# 本地 Demo 前端

单文件 HTML 调试器，浏览器直接打开就能跟 agent 对话。

[English](README.md)

## 用法

### 1. 启动 cloud-agent

```bash
cd ../   # tiny-bot-cloud-agent 根目录
make run
```

或者跑 fake-device 验证协议：

```bash
./bin/fake-device -device-id tinypal-demo -code DEMO-1234
```

### 2. 打开 demo

**推荐（与 agent 同域，部署后可用）：**

```bash
make run
# 浏览器打开 http://localhost:5678/demo/
```

线上部署后：手机需 **HTTPS**（见下方）；电脑 localhost 仍可用 `http://localhost:5678/demo/`。

**网页与音箱并行**：demo 默认设备是 `tinypal-demo` / `DEMO-1234`（用户 `u-demo`），和固件 `config.h` 里的 Device ID **不是同一个**。请勿在网页里填音箱的 ID 再点配网（`force=1` 会换 token，音箱掉线）。远端补登记且不重启容器：`make seed-demo-remote`。

**手机远程（无域名，推荐）：**

```bash
cd "$DEPLOY_SERVER_DIR/tiny-bot-cloud-agent"
docker compose --profile tunnel up -d
docker compose logs cloudflared   # 复制 https://xxx.trycloudflare.com
# 手机打开 https://xxx.trycloudflare.com/demo/
```

**手机远程（自签 HTTPS，固定 IP）：**

```bash
# 安全组开放 443
docker compose --profile https-selfsigned up -d
# 手机 https://<服务器IP>/demo/ ，首次信任证书
```

**有域名时：**

1. `.env` 设置 `TB_PUBLIC_DOMAIN=demo.example.com`
2. `docker compose --profile https up -d`
3. `https://demo.example.com/demo/`

**本地独立 HTTP 服务（开发调试）：**

```bash
make demo
# 浏览器打开 http://localhost:8888，Server URL 填 http://localhost:5678

open demo/index.html   # Safari 可能限制麦克风
```

### 3. 在浏览器里

1. Server URL 留空则自动使用当前站点；本地独立打开时填 `http://localhost:5678`
2. 点 **「1. 配网」** → 自动 POST /provision 拿 token，存到 localStorage
3. 点 **「2. 连接 WebSocket」** → 建立 WS 连接，发 hello
4. 按住 **「按住说话」**（手机端点右上角 **设置** 可展开配网项）
5. 松开 → 自动转 PCM → 分片发 audio → 发 end
6. 听回复，看 STT/LLM 流式文本出现在对话区

### 4. 用真 LLM 看到对话内容

`.env` 里改：

```bash
TB_PROVIDER_LLM=openai_compat
TB_OPENAI_COMPAT_BASE_URL=https://api.minimaxi.com
TB_OPENAI_COMPAT_API_KEY=sk-cp-xxx
TB_OPENAI_COMPAT_MODEL=MiniMax-M3
```

重启 cloud-agent，再 demo 一次。

## 功能

| 能力 | 实现 |
|---|---|
| HTTP 配网 | `POST /provision?device_id=…&code=…` |
| WebSocket 连接 | 带 hello/hello.ok 鉴权 |
| 录音 | `MediaRecorder` → `decodeAudioData` → 重采样到 16k → Int16 → base64 |
| 分片发 audio | 4KB base64 / chunk |
| 发 end | 录音停止时触发 |
| 收 stt/text/audio/done | 解析协议消息 |
| 文本显示 | STT 显示用户；LLM 流式累积显示 assistant |
| 音频播放 | Int16 → Float32 → AudioContext，按顺序串接（不重叠） |
| 日志 | 底部滚动日志（所有 WS 进出 + 系统事件） |

## 调试小技巧

- **F12** → Network / WS 标签看原始帧
- **左下角 Token** → 显示前 16 位（前缀一样就说明 token 没变）
- **底部日志** → 任何协议错误都在这里
- **清空日志 / 清空对话** → 不影响 WS 连接状态

## 限制

- 仅 Chrome / Edge / Safari 16+（getUserMedia + Web Audio API 兼容性）
- **麦克风仅支持安全上下文**：`https://` 或 `http://localhost`；`http://公网IP` / `http://192.168.x.x` / `file://` 均不可用（手机尤其严格）
- 浏览器要授权麦克风
- 一台浏览器一个会话（不持多设备）
