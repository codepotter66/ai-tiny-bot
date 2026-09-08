/**
 * ============================================================
 * TinyBot 主固件 - AI 语音对话机器人
 * ============================================================
 * 目标: 实现完整的 AI 对话功能
 * 描述: 接收语音输入 -> 上传到云端 Agent -> 播放语音回复
 * 硬件: ESP32 + SSD1306 OLED + MS4030 麦克风 + MAX98357 功放
 * 云端: WebSocket 连接 tiny-bot-cloud-agent
 * ============================================================
 *
 * 状态机：
 *   IDLE     - 免，能量 VAD 免提聆听（仅 cloud.isReady()）
 *   RECORD   - 录音中，按 chunk 发 audio；静音或超时发 end
 *   WAITING  - 已发 end，等 stt/text/audio（不跑 VAD）
 *   PLAYING  - TTS 音频写入 I2S 喇叭（不跑 VAD，不打断）
 *   ERROR    - 短暂错误提示后回 IDLE
 *
 * 流程：
 *   1. 上电初始化（WiFi、OLED、I2S）
 *   2. 等待 WiFi 连接成功
 *   3. cloud_client.begin()  →  必要时 POST /provision 拿 token
 *   4. cloud_client 自动开 WS → 发 hello → 收到 hello.ok → READY
 *   5. IDLE 读麦：音量超阈值 → RECORD → 边录边发
 *   6. 静音足够久（且已说够 minSpeech）或超时 → 发 end → WAITING
 *   7. 收到 audio → 喂 I2S 喇叭
 *   8. 收到 done → IDLE（rearm delay 后再听）
 * ============================================================
 */

#include <Arduino.h>
#include <WiFi.h>
#include <driver/i2s.h>
#include <Wire.h>
#include <Adafruit_GFX.h>
#include <Adafruit_SSD1306.h>

#include "config.h"
#include "cloud_client.h"
#include "oled_face.h"

// ============================================================
// 硬件
// ============================================================
#define LED_PIN 2

// OLED
#define SCREEN_WIDTH  128
#define SCREEN_HEIGHT 64
#define OLED_RESET    -1
Adafruit_SSD1306 display(SCREEN_WIDTH, SCREEN_HEIGHT, &Wire, OLED_RESET);

// I2S MIC
const i2s_port_t I2S_MIC_PORT = I2S_NUM_0;

// I2S SPK
const i2s_port_t I2S_SPK_PORT = I2S_NUM_1;

// ============================================================
// 状态
// ============================================================
enum class FsmState { IDLE, RECORD, WAITING, PLAYING, ERROR };

FsmState fsm = FsmState::IDLE;
unsigned long recordStartMs_ = 0;
unsigned long vadLastLoudMs_ = 0;
unsigned long vadRearmAtMs_ = 0;
unsigned long errorUntilMs_ = 0;
size_t recordSamplesSent_ = 0;
String lastUserText_;
String lastAssistantText_;
bool turnActive_ = false;  // WAITING/PLAYING 期间为 true

// Cloud 客户端
CloudClient cloud(HTTP_BASE, WS_HOST, WS_PORT, WS_PATH,
                  TB_DEVICE_ID, TB_PAIRING_CODE);

// ============================================================
// 函数前置声明
// ============================================================
void setupOLED();
void setupI2SMic();
void setupI2SSpeaker();
void connectWiFi();
void displayState();
void stopPlayback();
void goIdle();

bool readMicChunk(int16_t* out, size_t samples);
void writeSpkChunk(const int16_t* pcm, size_t samples);
void playBeep(int freq_hz, int duration_ms);
int chunkPeakPercent(const int16_t* buf, size_t samples);
void startVadRecord(const int16_t* firstChunk);
void finishVadRecord(bool discard);

void onSTT(const String& t, const String& lang);
void onText(const String& t);
void onTTS(const int16_t* pcm, size_t n);
void onDone();
void onCloudError(const String& code, const String& msg);
void onTool(const String& tool, const String& args);
void onStatus(const String& step, const String& phase, const String& text);
void onCloudReady();
void onCloudDisconnected();

// ============================================================
// OLED
// ============================================================
void setupOLED() {
  Wire.begin(OLED_SDA_PIN, OLED_SCL_PIN);
  if (!display.begin(SSD1306_SWITCHCAPVCC, OLED_I2C_ADDR)) {
    Serial.println("OLED init FAILED!");
    return;
  }
  display.clearDisplay();
  display.display();
  oledFaceBegin(&display);
}

void displayState() {
  switch (fsm) {
    case FsmState::IDLE:
      if (!cloud.isReady()) {
        oledFaceSet(FaceMood::Sleepy, "Reconnecting...");
      } else {
        oledFaceSet(FaceMood::Idle, "Speak anytime");
      }
      break;
    case FsmState::RECORD:
      oledFaceSet(FaceMood::Listen, "Listening...");
      break;
    case FsmState::WAITING:
      oledFaceSet(FaceMood::Think, "Thinking...");
      break;
    case FsmState::PLAYING:
      oledFaceSet(FaceMood::Happy, "Speaking");
      break;
    case FsmState::ERROR:
      oledFaceSet(FaceMood::Sad, "Error");
      break;
  }
  oledFaceTick(millis());
}

void stopPlayback() {
  i2s_zero_dma_buffer(I2S_SPK_PORT);
}

void goIdle() {
  turnActive_ = false;
  lastAssistantText_ = "";
  fsm = FsmState::IDLE;
  vadRearmAtMs_ = millis() + TB_VAD_REARM_DELAY_MS;
  displayState();
}

// ============================================================
// I2S
// ============================================================
void setupI2SMic() {
  i2s_config_t cfg = {
      .mode = (i2s_mode_t)(I2S_MODE_MASTER | I2S_MODE_RX),
      .sample_rate = I2S_MIC_SAMPLE_RATE,
      .bits_per_sample = I2S_BITS_PER_SAMPLE_16BIT,
      .channel_format = I2S_CHANNEL_FMT_ONLY_LEFT,
      .communication_format = I2S_COMM_FORMAT_STAND_I2S,
      .intr_alloc_flags = ESP_INTR_FLAG_LEVEL1,
      .dma_buf_count = 8,
      .dma_buf_len = 64,
      .use_apll = false,
      .tx_desc_auto_clear = false,
      .fixed_mclk = 0,
  };
  i2s_pin_config_t pin = {
      .bck_io_num = I2S_MIC_SCK_PIN,
      .ws_io_num = I2S_MIC_WS_PIN,
      .data_out_num = I2S_PIN_NO_CHANGE,
      .data_in_num = I2S_MIC_SD_PIN,
  };
  i2s_driver_install(I2S_MIC_PORT, &cfg, 0, NULL);
  i2s_set_pin(I2S_MIC_PORT, &pin);
  i2s_zero_dma_buffer(I2S_MIC_PORT);
}

void setupI2SSpeaker() {
  i2s_config_t cfg = {
      .mode = (i2s_mode_t)(I2S_MODE_MASTER | I2S_MODE_TX),
      .sample_rate = I2S_SPK_SAMPLE_RATE,
      .bits_per_sample = I2S_BITS_PER_SAMPLE_16BIT,
      .channel_format = I2S_CHANNEL_FMT_ONLY_LEFT,
      .communication_format = I2S_COMM_FORMAT_STAND_I2S,
      .intr_alloc_flags = ESP_INTR_FLAG_LEVEL1,
      .dma_buf_count = 8,
      .dma_buf_len = 256,
      .use_apll = false,
      .tx_desc_auto_clear = true,
      .fixed_mclk = 0,
  };
  i2s_pin_config_t pin = {
      .bck_io_num = I2S_SPK_BCLK_PIN,
      .ws_io_num = I2S_SPK_LRCLK_PIN,
      .data_out_num = I2S_SPK_DIN_PIN,
      .data_in_num = I2S_PIN_NO_CHANGE,
  };
  i2s_driver_install(I2S_SPK_PORT, &cfg, 0, NULL);
  i2s_set_pin(I2S_SPK_PORT, &pin);
}

bool readMicChunk(int16_t* out, size_t samples) {
  size_t bytesRead = 0;
  size_t want = samples * sizeof(int16_t);
  esp_err_t r = i2s_read(I2S_MIC_PORT, out, want, &bytesRead, 50 / portTICK_PERIOD_MS);
  if (r != ESP_OK) return false;
  return bytesRead == want;
}

void writeSpkChunk(const int16_t* pcm, size_t samples) {
  size_t written = 0;
  i2s_write(I2S_SPK_PORT, pcm, samples * sizeof(int16_t), &written, portMAX_DELAY);
}

void playBeep(int freq_hz, int duration_ms) {
  // 分块写，避免 int16_t buf[4800] 撑爆 loopTask 栈（约 8KB）
  const int sampleRate = I2S_SPK_SAMPLE_RATE;
  const int n = sampleRate * duration_ms / 1000;
  if (n <= 0 || n > 4800) return;
  const int chunk = 256;
  int16_t buf[chunk];
  for (int i = 0; i < n; ) {
    int m = n - i;
    if (m > chunk) m = chunk;
    for (int j = 0; j < m; j++) {
      float t = (float)(i + j) / sampleRate;
      buf[j] = (int16_t)(12000 * sin(2 * PI * freq_hz * t));
    }
    size_t w = 0;
    i2s_write(I2S_SPK_PORT, buf, m * sizeof(int16_t), &w, portMAX_DELAY);
    i += m;
  }
}

int chunkPeakPercent(const int16_t* buf, size_t samples) {
  int peak = 0;
  for (size_t i = 0; i < samples; i++) {
    int v = buf[i];
    if (v < 0) v = -v;
    if (v > peak) peak = v;
  }
  return (int)((peak * 100L) / 32768);
}

void startVadRecord(const int16_t* firstChunk) {
  Serial.println("[main] VAD speech start");
  recordStartMs_ = millis();
  vadLastLoudMs_ = recordStartMs_;
  recordSamplesSent_ = 0;
  turnActive_ = true;
  lastAssistantText_ = "";
  lastUserText_ = "";
  fsm = FsmState::RECORD;
  cloud.markRecordStart();
  displayState();
  cloud.sendAudio(firstChunk, AUDIO_CHUNK_SAMPLES);
  recordSamplesSent_ += AUDIO_CHUNK_SAMPLES;
}

void finishVadRecord(bool discard) {
  const unsigned long dur = millis() - recordStartMs_;
  if (discard || dur < (unsigned long)TB_VAD_MIN_SPEECH_MS ||
      recordSamplesSent_ * sizeof(int16_t) < 200) {
    Serial.printf("[main] VAD discard (dur=%lu ms, samples=%u)\n",
                  dur, (unsigned)recordSamplesSent_);
    if (recordSamplesSent_ > 0) {
      cloud.sendInterrupt();
    }
    cloud.markRecordEnd();
    goIdle();
    return;
  }
  Serial.printf("[main] VAD end, sending end (dur=%lu ms)\n", dur);
  cloud.sendEnd();
  cloud.markRecordEnd();
  fsm = FsmState::WAITING;
  displayState();
}

// ============================================================
// WiFi
// ============================================================
void connectWiFi() {
  Serial.printf("WiFi connecting to %s\n", WIFI_SSID);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  int attempts = 0;
  while (WiFi.status() != WL_CONNECTED && attempts < 60) {
    delay(500);
    Serial.print(".");
    attempts++;
  }
  if (WiFi.status() == WL_CONNECTED) {
    Serial.printf("\nWiFi OK, IP=%s\n", WiFi.localIP().toString().c_str());
  } else {
    Serial.println("\nWiFi FAIL");
  }
}

// ============================================================
// 云端回调
// ============================================================
void onSTT(const String& t, const String& lang) {
  if (!turnActive_) return;
  lastUserText_ = t;
  Serial.printf("[main] STT lang=%s: %s\n", lang.c_str(), t.c_str());
}

void onText(const String& t) {
  if (!turnActive_) return;
  lastAssistantText_ += t;
  Serial.printf("[main] TEXT: %s\n", t.c_str());
}

void onTTS(const int16_t* pcm, size_t samples) {
  if (!turnActive_) {
    Serial.println("[main] WARN: TTS after interrupt, dropping");
    return;
  }
  if (fsm != FsmState::WAITING && fsm != FsmState::PLAYING) {
    Serial.println("[main] WARN: got TTS audio in unexpected state, dropping");
    return;
  }
  if (fsm != FsmState::PLAYING) {
    fsm = FsmState::PLAYING;
    displayState();
  }
  writeSpkChunk(pcm, samples);
}

void onDone() {
  if (!turnActive_) {
    Serial.println("[main] TURN DONE (ignored, already idle)");
    return;
  }
  Serial.println("[main] TURN DONE");
  goIdle();
}

void onCloudError(const String& code, const String& msg) {
  Serial.printf("[main] ERROR %s: %s\n", code.c_str(), msg.c_str());
  stopPlayback();
  turnActive_ = false;

  if (code == "AUTH_FAIL" || code == "PROV_AUTH" || code == "PROV_NET" ||
      code == "TAKEN_OVER") {
    fsm = FsmState::IDLE;
    vadRearmAtMs_ = millis() + TB_VAD_REARM_DELAY_MS;
    oledFaceSet(FaceMood::Sleepy, "Reconnecting...");
    oledFaceTick(millis());
    return;
  }

  fsm = FsmState::ERROR;
  errorUntilMs_ = millis() + 2000;
  oledFaceSet(FaceMood::Sad, code.c_str());
  oledFaceTick(millis());
}

void onTool(const String& tool, const String& args) {
  Serial.printf("[main] TOOL %s %s\n", tool.c_str(), args.c_str());
  (void)args;
  // OLED 优先由 status 更新；无 status 时兜底显示 tool 名
  if (turnActive_ || fsm == FsmState::IDLE) {
    String s = "Tool:" + tool;
    if (s.length() > 21) s = s.substring(0, 21);
    FaceMood m = FaceMood::Think;
    if (fsm == FsmState::IDLE) m = FaceMood::Idle;
    else if (fsm == FsmState::PLAYING) m = FaceMood::Happy;
    else if (fsm == FsmState::RECORD) m = FaceMood::Listen;
    oledFaceSet(m, s.c_str());
    oledFaceTick(millis());
  }
}

void onStatus(const String& step, const String& phase, const String& text) {
  Serial.printf("[main] STATUS step=%s phase=%s text=%s\n",
                step.c_str(), phase.c_str(), text.c_str());
  (void)step;
  (void)phase;
  if (!(turnActive_ || fsm == FsmState::WAITING || fsm == FsmState::IDLE)) {
    return;
  }
  String s = text;
  if (s.isEmpty()) return;
  if (s.length() > 21) s = s.substring(0, 21);
  FaceMood m = FaceMood::Think;
  if (fsm == FsmState::IDLE) m = FaceMood::Idle;
  else if (fsm == FsmState::PLAYING) m = FaceMood::Happy;
  else if (fsm == FsmState::RECORD) m = FaceMood::Listen;
  oledFaceSet(m, s.c_str());
  oledFaceTick(millis());
}

void onCloudReady() {
  Serial.println("[main] cloud READY");
  if (fsm == FsmState::IDLE || fsm == FsmState::ERROR) {
    goIdle();
  }
}

void onCloudDisconnected() {
  Serial.println("[main] cloud disconnected");
  stopPlayback();
  turnActive_ = false;
  if (fsm == FsmState::RECORD || fsm == FsmState::WAITING ||
      fsm == FsmState::PLAYING) {
    fsm = FsmState::IDLE;
  }
  displayState();
}

// ============================================================
// setup / loop
// ============================================================
void setup() {
  Serial.begin(115200);
  Serial.println("========================================");
  Serial.println("TinyBot Firmware booting");
  Serial.println("========================================");

  pinMode(LED_PIN, OUTPUT);

  setupOLED();
  oledFaceSet(FaceMood::Boot, "Booting...");
  oledFaceTick(millis());

  setupI2SMic();
  setupI2SSpeaker();

  connectWiFi();
  if (WiFi.status() != WL_CONNECTED) {
    oledFaceSet(FaceMood::Sad, "WiFi FAIL");
    oledFaceTick(millis());
    return;
  }

  cloud.onSTT(onSTT);
  cloud.onText(onText);
  cloud.onTTS(onTTS);
  cloud.onDone(onDone);
  cloud.onError(onCloudError);
  cloud.onTool(onTool);
  cloud.onStatus(onStatus);
  cloud.onReady(onCloudReady);
  cloud.onDisconnected(onCloudDisconnected);

  if (!cloud.begin()) {
    Serial.println("[main] cloud.begin() failed; will retry in cloud.poll()");
    oledFaceSet(FaceMood::Sleepy, "Reconnecting...");
    oledFaceTick(millis());
  }

  fsm = FsmState::IDLE;
  vadRearmAtMs_ = millis() + TB_VAD_REARM_DELAY_MS;
  displayState();
  playBeep(1000, 200);
  // 蜂鸣后再延后武装，避免余音触发 VAD
  vadRearmAtMs_ = millis() + TB_VAD_REARM_DELAY_MS;
  Serial.println("TinyBot ready, speak anytime (handsfree VAD)");
}

void loop() {
  cloud.poll();
  oledFaceTick(millis());

  // 短暂错误提示结束 → IDLE
  if (fsm == FsmState::ERROR && millis() >= errorUntilMs_) {
    goIdle();
  }

  if (fsm == FsmState::IDLE) {
    if (!cloud.isReady()) {
      delay(5);
      return;
    }
    if (millis() < vadRearmAtMs_) {
      delay(5);
      return;
    }

    int16_t buf[AUDIO_CHUNK_SAMPLES];
    if (!readMicChunk(buf, AUDIO_CHUNK_SAMPLES)) {
      delay(5);
      return;
    }
    const int level = chunkPeakPercent(buf, AUDIO_CHUNK_SAMPLES);
    if (level >= TB_VAD_SPEECH_THRESHOLD) {
      startVadRecord(buf);
    }
  } else if (fsm == FsmState::RECORD) {
    int16_t buf[AUDIO_CHUNK_SAMPLES];
    if (readMicChunk(buf, AUDIO_CHUNK_SAMPLES)) {
      const int level = chunkPeakPercent(buf, AUDIO_CHUNK_SAMPLES);
      if (level >= TB_VAD_SPEECH_THRESHOLD) {
        vadLastLoudMs_ = millis();
      }
      cloud.sendAudio(buf, AUDIO_CHUNK_SAMPLES);
      recordSamplesSent_ += AUDIO_CHUNK_SAMPLES;
    }

    const unsigned long now = millis();
    const unsigned long spoken = now - recordStartMs_;
    const unsigned long silent = now - vadLastLoudMs_;
    if (spoken >= (unsigned long)TB_VAD_MIN_SPEECH_MS &&
        silent >= (unsigned long)TB_VAD_SILENCE_MS) {
      finishVadRecord(false);
    } else if (spoken > (unsigned long)AI_MAX_AUDIO_LENGTH_MS) {
      Serial.println("[main] record timeout");
      finishVadRecord(false);
    }
  }
  // WAITING / PLAYING：不跑 VAD（播报中不打断）

  delay(5);
}
