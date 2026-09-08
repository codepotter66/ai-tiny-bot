/**
 * ============================================================
 *  cloud_client 实现
 * ============================================================
 */
#include "cloud_client.h"
#include "config.h"

#include <HTTPClient.h>
#include <Preferences.h>
#include <WebSocketsClient.h>
#include <ArduinoJson.h>
#include <WiFiClientSecure.h>

#ifndef TB_USE_TLS
#define TB_USE_TLS 0
#endif

#ifndef TB_PROVISION_FORCE
#define TB_PROVISION_FORCE 0
#endif

#ifndef TB_PING_INTERVAL_MS
#define TB_PING_INTERVAL_MS 15000
#endif

#ifndef TB_SERVER_IDLE_MS
#define TB_SERVER_IDLE_MS 30000
#endif

#ifndef TB_RECONNECT_MAX_MS
#define TB_RECONNECT_MAX_MS 30000
#endif

#ifndef TB_TAKEN_OVER_WAIT_MS
#define TB_TAKEN_OVER_WAIT_MS 5000
#endif

// ============================================================
// base64 helpers（标准 RFC 4648，带 + / =）
// ============================================================
namespace {

constexpr char B64_TABLE[] =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

int b64Idx(char c) {
  if (c >= 'A' && c <= 'Z') return c - 'A';
  if (c >= 'a' && c <= 'z') return c - 'a' + 26;
  if (c >= '0' && c <= '9') return c - '0' + 52;
  if (c == '+') return 62;
  if (c == '/') return 63;
  return -1;
}

}  // namespace

String CloudClient::base64Encode(const uint8_t* data, size_t len) {
  String out;
  out.reserve(((len + 2) / 3) * 4);
  size_t i = 0;
  while (i + 3 <= len) {
    uint32_t v = (uint32_t)data[i] << 16 | (uint32_t)data[i + 1] << 8 | data[i + 2];
    out += B64_TABLE[(v >> 18) & 0x3F];
    out += B64_TABLE[(v >> 12) & 0x3F];
    out += B64_TABLE[(v >> 6) & 0x3F];
    out += B64_TABLE[v & 0x3F];
    i += 3;
  }
  if (i < len) {
    uint32_t v = (uint32_t)data[i] << 16;
    if (i + 1 < len) v |= (uint32_t)data[i + 1] << 8;
    out += B64_TABLE[(v >> 18) & 0x3F];
    out += B64_TABLE[(v >> 12) & 0x3F];
    out += (i + 1 < len) ? B64_TABLE[(v >> 6) & 0x3F] : '=';
    out += '=';
  }
  return out;
}

size_t CloudClient::base64Decode(const String& in, uint8_t* out, size_t outCap) {
  size_t written = 0;
  size_t i = 0;
  uint32_t buf = 0;
  int bits = 0;
  while (i < in.length()) {
    char c = in.charAt(i++);
    if (c == '=' || c == '\n' || c == '\r' || c == ' ') continue;
    int v = b64Idx(c);
    if (v < 0) continue;
    buf = (buf << 6) | (uint32_t)v;
    bits += 6;
    if (bits >= 8) {
      bits -= 8;
      if (written >= outCap) return written;
      out[written++] = (uint8_t)((buf >> bits) & 0xFF);
    }
  }
  return written;
}

// ============================================================
// 构造/析构
// ============================================================
CloudClient::CloudClient(const String& http_base,
                         const String& ws_host,
                         uint16_t ws_port,
                         const String& ws_path,
                         const String& device_id,
                         const String& pairing_code)
    : httpBase_(http_base),
      wsHost_(ws_host),
      wsPort_(ws_port),
      wsPath_(ws_path),
      deviceId_(device_id),
      pairingCode_(pairing_code) {}

CloudClient::~CloudClient() {
  disconnectWS();
}

void CloudClient::setState(State s) { state_ = s; }

void CloudClient::setError(const String& code, const String& msg) {
  lastErr_ = code + ":" + msg;
  if (onError_) onError_(code, msg);
}

void CloudClient::noteServerMsg() {
  lastServerMsgMs_ = millis();
}

// ============================================================
// NVS 持久化 token
// ============================================================
bool CloudClient::loadToken() {
  Preferences prefs;
  if (!prefs.begin("tinybot", true)) {
    return false;
  }
  String t = prefs.getString("token", "");
  prefs.end();
  if (t.length() == 64) {
    token_ = t;
    Serial.printf("[cloud] token loaded from NVS (%u chars)\n", t.length());
    return true;
  }
  return false;
}

void CloudClient::clearToken() {
  Preferences prefs;
  if (prefs.begin("tinybot", false)) {
    prefs.remove("token");
    prefs.end();
  }
  token_ = "";
  Serial.println("[cloud] token cleared from NVS");
}

// ============================================================
// HTTP /provision
// ============================================================
bool CloudClient::doProvision(bool force) {
  setState(State::PROVISIONING);
  bool useForce = force || (TB_PROVISION_FORCE != 0);
  String url = httpBase_ + "/provision?device_id=" + deviceId_ +
               "&code=" + pairingCode_;
  if (useForce) {
    url += "&force=1";
  }
  Serial.printf("[cloud] POST %s\n", url.c_str());

  HTTPClient http;
#if TB_USE_TLS
  WiFiClientSecure secure;
  secure.setInsecure();  // 无 pinning；生产可改为 setCACert
  if (!http.begin(secure, url)) {
    Serial.println("[cloud] provision https begin failed");
    setError("PROV_NET", "https begin failed");
    setState(State::ERROR);
    return false;
  }
#else
  if (!http.begin(url)) {
    Serial.println("[cloud] provision http begin failed");
    setError("PROV_NET", "http begin failed");
    setState(State::ERROR);
    return false;
  }
#endif
  http.setTimeout(5000);
  int code = http.POST("");
  if (code != 200) {
    Serial.printf("[cloud] provision failed: HTTP %d\n", code);
    if (code == 401) {
      setError("PROV_AUTH", "pairing code wrong or device already bound");
    } else {
      setError("PROV_NET", "HTTP " + String(code));
    }
    http.end();
    setState(State::ERROR);
    return false;
  }
  String body = http.getString();
  http.end();

  JsonDocument doc;
  DeserializationError err = deserializeJson(doc, body);
  if (err) {
    Serial.printf("[cloud] provision bad json: %s\n", err.c_str());
    setError("PROV_BAD", "json parse");
    setState(State::ERROR);
    return false;
  }
  String token = doc["token"].as<String>();
  if (token.length() != 64) {
    setError("PROV_BAD", "token length");
    setState(State::ERROR);
    return false;
  }

  Preferences prefs;
  if (prefs.begin("tinybot", false)) {
    prefs.putString("token", token);
    prefs.end();
  }
  token_ = token;
  needForceProvision_ = false;
  Serial.println("[cloud] provisioned, token saved to NVS");
  return true;
}

// ============================================================
// WebSocket
// ============================================================
void CloudClient::disconnectWS() {
  if (ws_) {
    ws_->disconnect();
    delete ws_;
    ws_ = nullptr;
  }
}

void CloudClient::connectWS() {
  disconnectWS();
  ws_ = new WebSocketsClient();
  ws_->onEvent([this](WStype_t type, uint8_t* payload, size_t length) {
    handleEvent((int)type, payload, length);
  });
  setState(State::CONNECTING);
  Serial.printf("[cloud] WS connect %s:%u%s tls=%d\n",
                wsHost_.c_str(), wsPort_, wsPath_.c_str(), TB_USE_TLS);
#if TB_USE_TLS
  ws_->beginSSL(wsHost_.c_str(), wsPort_, wsPath_.c_str());
#else
  ws_->begin(wsHost_.c_str(), wsPort_, wsPath_.c_str());
#endif
}

void CloudClient::sendHello() {
  if (ws_ == nullptr) return;
  JsonDocument doc;
  doc["type"] = "hello";
  doc["device_id"] = deviceId_;
  doc["token"] = token_;
  doc["proto"] = TB_PROTOCOL_VERSION;
  String out;
  serializeJson(doc, out);
  ws_->sendTXT(out);
  Serial.printf("[cloud] >> hello proto=%d (%u bytes)\n",
                TB_PROTOCOL_VERSION, out.length());
}

void CloudClient::sendPing() {
  if (ws_ == nullptr || state_ != State::READY) return;
  ws_->sendTXT("{\"type\":\"ping\"}");
  lastPingMs_ = millis();
}

void CloudClient::close() {
  disconnectWS();
  setState(State::INIT);
}

void CloudClient::scheduleReconnect(unsigned long delayMs) {
  nextRetryMs_ = millis() + delayMs;
  setState(State::RECONNECTING);
  Serial.printf("[cloud] reconnect scheduled in %lu ms\n", delayMs);
  if (onDisconnected_) onDisconnected_();
}

void CloudClient::tryReconnect() {
  unsigned long now = millis();
  if (now < nextRetryMs_) return;

  if (pendingTakenOver_) {
    pendingTakenOver_ = false;
  }

  Serial.println("[cloud] attempting reconnect");
  if (needForceProvision_ || token_.length() != 64) {
    if (!doProvision(true)) {
      backoffMs_ = backoffMs_ < 1000 ? 1000 : backoffMs_ * 2;
      if (backoffMs_ > TB_RECONNECT_MAX_MS) backoffMs_ = TB_RECONNECT_MAX_MS;
      scheduleReconnect(backoffMs_);
      return;
    }
  }
  backoffMs_ = 1000;
  connectWS();
}

void CloudClient::handleAuthFail() {
  clearToken();
  needForceProvision_ = true;
  disconnectWS();
  backoffMs_ = 1000;
  scheduleReconnect(500);
}

// ============================================================
// 出站消息
// ============================================================
void CloudClient::sendAudio(const int16_t* pcm, size_t samples) {
  if (state_ != State::READY || ws_ == nullptr) return;
  size_t n = samples * sizeof(int16_t);
  String b64 = base64Encode(reinterpret_cast<const uint8_t*>(pcm), n);
  JsonDocument doc;
  doc["type"] = "audio";
  doc["seq"] = audioSeq_++;
  doc["data"] = b64;
  String out;
  serializeJson(doc, out);
  if (out.length() > WS_MAX_FRAME_BYTES) {
    Serial.printf("[cloud] WARN: audio frame %u > %d bytes, will likely be dropped\n",
                  out.length(), WS_MAX_FRAME_BYTES);
  }
  ws_->sendTXT(out);
}

void CloudClient::sendEnd() {
  if (state_ != State::READY || ws_ == nullptr) return;
  ws_->sendTXT("{\"type\":\"end\"}");
  Serial.println("[cloud] >> end");
}

void CloudClient::sendInterrupt() {
  if (state_ != State::READY || ws_ == nullptr) return;
  ws_->sendTXT("{\"type\":\"interrupt\"}");
  Serial.println("[cloud] >> interrupt");
}

void CloudClient::markRecordStart() {
  audioSeq_ = 0;
  Serial.println("[cloud] record start");
}

void CloudClient::markRecordEnd() {
  Serial.println("[cloud] record end");
}

// ============================================================
// 入站消息
// ============================================================
void CloudClient::handleEvent(int type, uint8_t* payload, size_t length) {
  switch (type) {
    case WStype_DISCONNECTED:
      Serial.println("[cloud] WS disconnected");
      if (state_ == State::READY || state_ == State::CONNECTING) {
        backoffMs_ = backoffMs_ < 1000 ? 1000 : backoffMs_ * 2;
        if (backoffMs_ > TB_RECONNECT_MAX_MS) backoffMs_ = TB_RECONNECT_MAX_MS;
        scheduleReconnect(backoffMs_);
      }
      break;
    case WStype_CONNECTED:
      Serial.println("[cloud] WS connected, sending hello");
      noteServerMsg();
      sendHello();
      break;
    case WStype_TEXT: {
      String msg(reinterpret_cast<const char*>(payload), length);
      noteServerMsg();
      handleTextMessage(msg);
      break;
    }
    case WStype_ERROR:
      Serial.printf("[cloud] WS error: %s\n",
                    length > 0 ? reinterpret_cast<const char*>(payload) : "(none)");
      break;
    default:
      break;
  }
}

void CloudClient::handleTextMessage(const String& msg) {
  JsonDocument doc;
  DeserializationError err = deserializeJson(doc, msg);
  if (err) {
    Serial.printf("[cloud] bad json: %s | raw: %.80s\n", err.c_str(), msg.c_str());
    return;
  }
  const char* type = doc["type"] | "";
  if (strcmp(type, "hello.ok") == 0) {
    sampleRate_ = doc["sample_rate"] | 0;
    int proto = doc["proto"] | 0;
    Serial.printf("[cloud] << hello.ok sample_rate=%d proto=%d\n",
                  sampleRate_, proto);
    if (sampleRate_ > 0 && sampleRate_ != I2S_SPK_SAMPLE_RATE) {
      Serial.printf("[cloud] WARN: hello.ok sample_rate=%d != I2S_SPK_SAMPLE_RATE=%d\n",
                    sampleRate_, I2S_SPK_SAMPLE_RATE);
    }
    backoffMs_ = 1000;
    lastPingMs_ = millis();
    setState(State::READY);
    if (onReady_) onReady_();
  } else if (strcmp(type, "stt") == 0) {
    String t = doc["text"] | "";
    String lang = doc["lang"] | "";
    Serial.printf("[cloud] << stt lang=%s: %s\n", lang.c_str(), t.c_str());
    if (onSTT_) onSTT_(t, lang);
  } else if (strcmp(type, "text") == 0) {
    String t = doc["text"] | "";
    if (onText_) onText_(t);
  } else if (strcmp(type, "audio") == 0) {
    String b64 = doc["data"] | "";
    if (b64.isEmpty()) return;
    size_t maxSamples = (b64.length() / 4) * 3 / 2 + 1;
    int16_t* pcm = (int16_t*)malloc(maxSamples * sizeof(int16_t));
    if (!pcm) return;
    size_t bytes = base64Decode(b64, (uint8_t*)pcm, maxSamples * 2);
    size_t samples = bytes / 2;
    if (onTTS_ && samples > 0) onTTS_(pcm, samples);
    free(pcm);
  } else if (strcmp(type, "done") == 0) {
    Serial.println("[cloud] << done");
    if (onDone_) onDone_();
  } else if (strcmp(type, "error") == 0) {
    String e = doc["error"] | "";
    Serial.printf("[cloud] << error: %s\n", e.c_str());
    int colon = e.indexOf(':');
    String code = colon > 0 ? e.substring(0, colon) : e;
    String rest = colon > 0 ? e.substring(colon + 1) : "";
    if (onError_) onError_(code, rest);
    if (code == "AUTH_FAIL") {
      handleAuthFail();
    } else if (code == "TAKEN_OVER") {
      pendingTakenOver_ = true;
      disconnectWS();
      scheduleReconnect(TB_TAKEN_OVER_WAIT_MS);
    }
    // 其它 *_FAIL / INTERNAL：连接保持，由应用层回 IDLE
  } else if (strcmp(type, "tool") == 0) {
    String tool = doc["tool"] | "";
    String args;
    if (doc["args"].is<JsonObject>() || doc["args"].is<JsonArray>()) {
      serializeJson(doc["args"], args);
    } else {
      args = doc["args"] | "";
    }
    Serial.printf("[cloud] << tool: %s args=%s\n", tool.c_str(), args.c_str());
    if (onTool_) onTool_(tool, args);
  } else if (strcmp(type, "status") == 0) {
    String step = doc["step"] | "";
    String phase = doc["phase"] | "";
    String text = doc["text"] | "";
    Serial.printf("[cloud] << status step=%s phase=%s text=%s\n",
                  step.c_str(), phase.c_str(), text.c_str());
    if (onStatus_) onStatus_(step, phase, text);
  } else if (strcmp(type, "pong") == 0) {
    // noteServerMsg already called
  } else {
    Serial.printf("[cloud] << unknown type: %s\n", type);
  }
}

// ============================================================
// 启动 + 主循环
// ============================================================
bool CloudClient::begin() {
  needForceProvision_ = false;
  backoffMs_ = 1000;
  if (!loadToken()) {
    if (!doProvision(TB_PROVISION_FORCE != 0)) {
      scheduleReconnect(backoffMs_);
      return false;
    }
  }
  connectWS();
  return true;
}

void CloudClient::poll() {
  if (ws_) ws_->loop();

  unsigned long now = millis();

  if (state_ == State::READY) {
    if (now - lastPingMs_ >= TB_PING_INTERVAL_MS) {
      sendPing();
    }
    if (lastServerMsgMs_ > 0 && now - lastServerMsgMs_ >= TB_SERVER_IDLE_MS) {
      Serial.println("[cloud] server idle timeout, reconnecting");
      disconnectWS();
      backoffMs_ = 1000;
      scheduleReconnect(500);
    }
  }

  if (state_ == State::RECONNECTING || state_ == State::ERROR ||
      state_ == State::INIT) {
    tryReconnect();
  }
}
