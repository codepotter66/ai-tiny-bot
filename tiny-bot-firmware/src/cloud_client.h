/**
 * ============================================================
 *  cloud_client - 与 tiny-bot-cloud-agent 对接的客户端
 * ============================================================
 *
 * 功能：
 *   1. 启动时从 NVS 取 token；如果没有，调用 HTTP /provision 拿
 *   2. 打开 WebSocket 到云端，发 hello
 *   3. 提供 sendAudio / sendEnd / sendInterrupt / sendPing 接口
 *   4. 通过回调通知 STT / LLM / TTS / done / error / tool / status
 *   5. 断线退避重连、ping/pong 空闲检测、AUTH_FAIL force 重配网
 *
 * 外部在 loop() 里周期性调用 poll() 即可。
 *
 * 对接规范见：
 *   ../tiny-bot-cloud-agent/docs/firmware-integration/ws-protocol.schema.json
 *   ../tiny-bot-cloud-agent/docs/human-docs/02-firmware-integration.md
 * ============================================================
 */
#pragma once

#include <Arduino.h>
#include <functional>

class WebSocketsClient;

class CloudClient {
 public:
  using OnSTT = std::function<void(const String& text, const String& lang)>;
  using OnText = std::function<void(const String& delta)>;
  using OnTTS = std::function<void(const int16_t* pcm, size_t samples)>;
  using OnDone = std::function<void()>;
  using OnError = std::function<void(const String& code, const String& msg)>;
  using OnTool = std::function<void(const String& tool, const String& args)>;
  using OnStatus = std::function<void(const String& step, const String& phase, const String& text)>;
  using OnReady = std::function<void()>;
  using OnDisconnected = std::function<void()>;

  CloudClient(const String& http_base,
              const String& ws_host,
              uint16_t ws_port,
              const String& ws_path,
              const String& device_id,
              const String& pairing_code);
  ~CloudClient();

  // 启动：加载/刷新 token 并连 WS；返回 true 表示已发起连接（未必已 READY）
  bool begin();

  // 周期性调用（建议每 5-20ms）
  void poll();

  // 录音期间每拿到一帧 PCM 就调一次。pcm 是 int16 little-endian
  void sendAudio(const int16_t* pcm, size_t samples);

  // 一句录音结束
  void sendEnd();

  // 打断当前 turn（WAITING/PLAYING）
  void sendInterrupt();

  void close();

  bool isReady() const { return state_ == State::READY; }
  bool isReconnecting() const {
    return state_ == State::RECONNECTING || state_ == State::PROVISIONING;
  }
  int negotiatedSampleRate() const { return sampleRate_; }

  void markRecordStart();
  void markRecordEnd();

  void onSTT(OnSTT cb) { onSTT_ = cb; }
  void onText(OnText cb) { onText_ = cb; }
  void onTTS(OnTTS cb) { onTTS_ = cb; }
  void onDone(OnDone cb) { onDone_ = cb; }
  void onError(OnError cb) { onError_ = cb; }
  void onTool(OnTool cb) { onTool_ = cb; }
  void onStatus(OnStatus cb) { onStatus_ = cb; }
  void onReady(OnReady cb) { onReady_ = cb; }
  void onDisconnected(OnDisconnected cb) { onDisconnected_ = cb; }

 private:
  enum class State {
    INIT,
    PROVISIONING,
    CONNECTING,
    READY,
    RECONNECTING,
    ERROR,
  };

  String httpBase_;
  String wsHost_;
  uint16_t wsPort_;
  String wsPath_;
  String deviceId_;
  String pairingCode_;
  String token_;

  State state_ = State::INIT;
  WebSocketsClient* ws_ = nullptr;
  uint32_t audioSeq_ = 0;
  int sampleRate_ = 0;
  String lastErr_;

  unsigned long lastPingMs_ = 0;
  unsigned long lastServerMsgMs_ = 0;
  unsigned long nextRetryMs_ = 0;
  unsigned long backoffMs_ = 1000;
  bool needForceProvision_ = false;
  bool pendingTakenOver_ = false;

  OnSTT onSTT_;
  OnText onText_;
  OnTTS onTTS_;
  OnDone onDone_;
  OnError onError_;
  OnTool onTool_;
  OnStatus onStatus_;
  OnReady onReady_;
  OnDisconnected onDisconnected_;

  bool doProvision(bool force);
  bool loadToken();
  void clearToken();
  void connectWS();
  void disconnectWS();
  void sendHello();
  void sendPing();
  void scheduleReconnect(unsigned long delayMs);
  void tryReconnect();
  void handleAuthFail();
  void noteServerMsg();
  void handleEvent(int type, uint8_t* payload, size_t length);
  void handleTextMessage(const String& msg);
  void setError(const String& code, const String& msg);
  void setState(State s);
  static String base64Encode(const uint8_t* data, size_t len);
  static size_t base64Decode(const String& in, uint8_t* out, size_t outCap);
};
