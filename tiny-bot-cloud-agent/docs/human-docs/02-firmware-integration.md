# 02. 固件对接指南

> 写于 2026-06-08
>
> 给 [tiny-bot-firmware](../../tiny-bot-firmware/) 嵌入式开发者看的逐步对接文档。
> 配套机器可读版：[docs/firmware-integration/](../firmware-integration/README.md)

## TL;DR

1. 在跑 cloud-agent 的机器上用 [cmd/seed](../../cmd/seed/) **预登记** device_id + pairing_code（写入数据库，不是立刻连上设备）
2. 把相同 ID/码写入固件 `config.h`；设备上电 → 连 WiFi → `POST /provision?device_id=…&code=…` 拿 token 存 NVS
3. 打开 `ws://host:5678/ws` → 发 `hello` → 收到 `hello.ok` 后开始录音
4. 录音数据分 chunk 发 `audio`（base64 PCM）→ 发 `end`
5. 收 `stt` / `text` / `audio` / `done` → 播放 PCM
6. 回到待机，等下一次按键

未执行 `seed` 时 `/provision` 会返回 `device not found`（HTTP 401），固件 OLED 常表现为一直 `Reconnecting...`。

完整消息 schema 见 [ws-protocol.schema.json](../firmware-integration/ws-protocol.schema.json)；HTTP 接口见 [http-api.openapi.yaml](../firmware-integration/http-api.openapi.yaml)。

## 对接步骤

### 步骤 0：环境准备

- 准备一台能跑 Go 1.22+ 的机器（编译 cloud-agent）
- 在该机器上跑 `./bin/seed -device-id tinypal-01 -pairing-code ABCD-1234`：**在云端 SQLite 登记设备**（入场许可）。seed 本身不会让 ESP32 上线；板子之后用同一组 ID/码去 provision
- 把 `tinypal-01` 和 `ABCD-1234` 写入固件 `config.h`（生产用 `idf.py menuconfig` 或 `platformio.ini` 的 build_flags 注入）

```c
// config.h
#define TB_DEVICE_ID     "tinypal-01"
#define TB_PAIRING_CODE  "ABCD-1234"   // 首次启动后会被清掉
#define TB_HOST          "your.domain.example"
```

### 步骤 1：上电 / 重连逻辑

```
on boot:
    nvs_init()
    wifi_connect()
    if nvs_has("tb_token") == false:
        // 首次启动：走配网
        token = http_post("/provision?device_id=...&code=...")
        nvs_save("tb_token", token)
        nvs_save("tb_provisioned", true)
    // 拿到 token 后进入 ws loop
```

### 步骤 2：HTTP 配网（仅首次）

```c
// 伪代码
String url = String("http://") + TB_HOST + "/provision?device_id=" + TB_DEVICE_ID + "&code=" + TB_PAIRING_CODE;
HTTPClient http;
http.begin(url);
int code = http.POST("");  // 空 body
if (code == 200) {
    String body = http.getString();
    String token = parse_json_field(body, "token");  // 64 字符 hex
    nvs_set("tb_token", token);
} else if (code == 401) {
    // pairing code 错 或 设备已 bound
    // 已 bound 的话，token 应已存在 NVS；用旧 token 即可
} else {
    // 网络问题，重试
}
```

返回示例：
```json
{"device_id": "tinypal-01", "token": "a1b2c3...64chars", "hint": "..."}
```

### 步骤 3：WebSocket 连接

```c
String ws_url = String("ws://") + TB_HOST + "/ws";
WebSocketClient ws;
ws.connect(ws_url);
```

无 subprotocol、无 origin 限制。

### 步骤 4：发 hello

```json
{
  "type": "hello",
  "device_id": "tinypal-01",
  "token": "a1b2c3...64chars",
  "proto": 1
}
```

等 `hello.ok`：

```json
{"type": "hello.ok", "sample_rate": 16000, "proto": 1}
```

如果收到 `error`，断开重连：
- `AUTH_FAIL`：token 错了，重新走步骤 2
- `BAD_PROTO`：固件版本太旧或太新
- `TAKEN_OVER`：同一 device 已有活跃连接（可能别的实例在线）

### 步骤 5：录音 → 发 audio

按键按下 → 开始录音。每 32-100ms 把 PCM 缓冲发一次：

```json
{
  "type": "audio",
  "seq": 0,
  "data": "<base64 of 1024-3200 bytes PCM>"
}
```

#### PCM 格式（关键！）

| 项 | 值 |
|---|---|
| 采样率 | 16000 Hz |
| 位深 | 16 bit |
| 声道 | 1（mono） |
| 字节序 | little-endian（标准 PCM） |
| 编码 | base64（标准 RFC 4648，带 `+` `/`） |

ESP32 端拿到 I2S 麦克风的 `int16_t` 数组后，**直接当作字节流 base64 编码**，不要做 swap、归一化、padding 处理。

**chunk 大小建议**：

| 采样率 | 32 ms | 50 ms | 100 ms |
|---|---|---|---|
| 16000 | 1024 样本 = 2048 字节 | 1600 = 3200 字节 | 3200 = 6400 字节 |

太小 → 帧头开销大；太大 → 端到端延迟高。**64 字节 base64 编码后 ~85 字节**，建议总消息 ≤ 4 KiB。

### 步骤 6：发 end

按键松开（或 VAD 检测到静音）→ 发：

```json
{"type": "end"}
```

### 步骤 7：等回复

服务端会按顺序发：

1. `{"type": "stt", "text": "<用户说的话>"}`（STT 识别结果）
2. （可选）若触发 tool：`{"type":"status","step":"…","phase":"start","text":"查询天气"}` 等静默进度；可穿插 `tool`
3. `{"type": "text", "text": "嗯"}`（LLM 最终可见回复的 token，可能很多个）
4. `{"type": "text", "text": "，"}`
5. ...（持续约 1-3 秒）
6. `{"type": "audio", "seq": 1, "data": "<base64 PCM>"}`（TTS 第一个 chunk）
7. `{"type": "audio", "seq": 2, ...}`（更多 TTS chunk，可能多个）
8. `{"type": "done"}`（turn 结束）

**`status` 消息**：单轮 turn 内任务步骤进度（`phase`=`start`/`running`/`done`/`error`）。**不触发 TTS**，固件应用 `text`（建议 ≤21 字符）更新 OLED。旧固件可忽略未知 type。

固件收到 `audio` 后：
- base64 解码 → 喂给 I2S 功放（DMA buffer）
- 不用等 `done` 才开始播，**边收边播**

### 步骤 8：回到待机

收到 `done` 后：
- 清空 I2S 缓冲
- OLED 显示"小主人好呀"
- 等下一次按键

## 时序要求

| 指标 | 目标 | 实测（mock LLM） |
|---|---|---|
| HTTP 配网总耗时 | < 3 s | < 200 ms |
| WS 鉴权往返 | < 500 ms | < 50 ms |
| `end` → `stt` | < 500 ms | 取决于 ASR |
| `stt` → 第一个 `audio` | < 1.5 s | ~600 ms |
| 单 turn 总耗时 | < 5 s | < 2 s |
| WS idle 超时 | 60 s | 60 s |

如果连续 60 秒无消息，服务端会断开。固件应实现：
- 每 30 s 发一次 `{"type": "ping"}`
- 收到任何响应说明连接活着
- 30 s 没响应就重连

## 错误码参考

| code | 含义 | 客户端应做 |
|---|---|---|
| `AUTH_FAIL` | 鉴权失败 | 清 NVS 里的 token，重新走步骤 2 |
| `BAD_PROTO` | 协议版本不对 | 升级固件 |
| `STT_FAIL` | ASR 失败 | 重试一次；连续 3 次失败就重连 |
| `LLM_FAIL` | LLM 失败 | 重试；连续失败上报用户"网络不好" |
| `TTS_FAIL` | TTS 失败 | 跳过播放，显示文本 |
| `INTERNAL` | 服务端 bug | 上报日志；重连 |
| `TAKEN_OVER` | 同设备别处连接 | 等 5 s 再连 |

## 常见坑

### 1. base64 编码变体

**必须用标准 base64**（带 `+` `/`，padding 用 `=`）。**不要用 URL-safe 变体**（`-` `_`，无 padding），那是 base64url，服务端解码会失败。

ESP32 端用 `mbedtls` 的 `mbedtls_base64_encode` 默认就是标准 base64，OK。

### 2. 字节序

ESP32 是 little-endian。`int16_t` 数组直接 memcpy 到 `uint8_t` 数组就是正确的 PCM 字节序。不要做 `__builtin_bswap16` 之类。

### 3. JSON 转义

用户输入和 LLM 输出可能包含 `"` `\` `}` 等。固件侧 ArduinoJson / cJSON 都自动转义，不用手写。

### 4. 大端小端 STT 时间戳

v1 我们不用 `ts_ms` 字段，**固件可以不填**。即使填了也仅用于日志调试。

### 5. 多个 `audio` chunk 之间不要 sleep

`audio` chunks 是按时间顺序发的。固件应该一收到就喂 I2S，I2S DMA 会自动按采样率播放。如果中间加 sleep，会丢字。

### 6. 不要在 turn 中途重连

收到 `done` 之前不要断开重连，否则 turn 中断、用户听不到完整回复。

## 完整流程时序图

```
ESP32                              Server
  │                                  │
  │── POST /provision ──────────────►│  (仅首次)
  │◄─ 200 {token: "..."} ────────────│
  │                                  │
  │── WS upgrade ──────────────────►│
  │◄─ 101 Switching Protocols ──────│
  │                                  │
  │── {type: "hello", ...} ────────►│
  │◄─ {type: "hello.ok", ...} ──────│
  │                                  │
  │── {type: "audio", seq: 0} ────►│
  │── {type: "audio", seq: 1} ────►│
  │── {type: "audio", seq: 2} ────►│
  │   ...                            │
  │── {type: "end"} ───────────────►│
  │                                  │ (ASR)
  │◄─ {type: "stt", text: "..."} ───│
  │                                  │ (LLM stream)
  │◄─ {type: "text", text: "嗯"} ──│
  │◄─ {type: "text", text: "，"} ──│
  │                                  │ (TTS stream)
  │◄─ {type: "audio", seq: 1, ...} │
  │◄─ {type: "audio", seq: 2, ...} │
  │   ...                            │
  │◄─ {type: "done"} ───────────────│
  │                                  │
  │   (回到待机)                     │
```

## 示例代码片段（ArduinoJson + WebSocketClient）

```cpp
#include <ArduinoJson.h>
#include <WebSocketClient.h>

void sendAudio(WSClient &ws, const int16_t *pcm, size_t samples) {
    StaticJsonDocument<4096> doc;
    doc["type"] = "audio";
    doc["seq"] = seq_counter++;
    
    // base64 encode
    size_t out_len;
    mbedtls_base64_encode(nullptr, 0, &out_len, (uint8_t*)pcm, samples * 2);
    char *b64 = (char*)malloc(out_len);
    mbedtls_base64_encode((uint8_t*)b64, out_len, &out_len, (uint8_t*)pcm, samples * 2);
    doc["data"] = b64;
    free(b64);
    
    String out;
    serializeJson(doc, out);
    ws.send(out);
}

void sendEnd(WSClient &ws) {
    StaticJsonDocument<64> doc;
    doc["type"] = "end";
    String out;
    serializeJson(doc, out);
    ws.send(out);
}
```

## 联调步骤

1. 跑 cloud-agent： `./bin/tiny-bot-cloud-agent`
2. 跑 fake-device： `TB_FAKE_ASR_TEXT="今天天气" ./bin/fake-device -code ABCD-1234 -device-id tinypal-01`
3. 看到 "DONE — turn complete" → 链路通了
4. 烧录固件，把 `WS_HOST` 指到 `ws://<your-host>:5678/ws`
5. 按固件上的按键，看 OLED 是否显示从云端返回的回复文本

## 相关代码 / 文档

| 路径 | 内容 |
|---|---|
| [internal/ws/protocol.go](../../internal/ws/protocol.go) | 消息常量、struct 定义 |
| [internal/ws/session.go](../../internal/ws/session.go) | 服务端 session 状态机 |
| [docs/firmware-integration/ws-protocol.schema.json](../firmware-integration/ws-protocol.schema.json) | 机器可读 schema |
| [docs/firmware-integration/http-api.openapi.yaml](../firmware-integration/http-api.openapi.yaml) | HTTP OpenAPI |
| [CHANGELOG.md](../../CHANGELOG.md) | 协议变更记录 |
| [../tiny-bot-firmware/CLAUDE.md](../../tiny-bot-firmware/CLAUDE.md) | 固件项目规范 |
| [../tiny-bot-firmware/src/main.cpp](../../tiny-bot-firmware/src/main.cpp) | 固件主入口 |
