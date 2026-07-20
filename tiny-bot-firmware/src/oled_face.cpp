/**
 * 程序化矢量表情脸（椭圆眼白 + 瞳孔 + 眉/睑/嘴）。
 * 布局：y0–47 脸部，y52–63 底栏状态。
 */

#include "oled_face.h"

#include <Arduino.h>
#include <string.h>

namespace {

Adafruit_SSD1306* disp_ = nullptr;

FaceMood mood_ = FaceMood::Boot;
char status_[22] = "Booting...";

float gazeX_ = 0.f;
float gazeTarget_ = 0.f;
float blink_ = 0.f;  // 0 open .. 1 closed
uint8_t blinkPhase_ = 0;  // 0 idle, 1 closing, 2 opening
unsigned long nextBlinkAt_ = 0;
unsigned long nextGazeAt_ = 0;
unsigned long lastAnimMs_ = 0;
bool forceRedraw_ = true;

constexpr unsigned long kTickMs = 60;

int16_t clampi(int16_t v, int16_t lo, int16_t hi) {
  if (v < lo) return lo;
  if (v > hi) return hi;
  return v;
}

float clampf(float v, float lo, float hi) {
  if (v < lo) return lo;
  if (v > hi) return hi;
  return v;
}

void scheduleBlink(unsigned long now) {
  unsigned long gap = 1800 + (unsigned long)(esp_random() % 3200);
  if (mood_ == FaceMood::Sleepy) {
    gap = 2800 + (unsigned long)(esp_random() % 2500);
  } else if (mood_ == FaceMood::Happy) {
    gap = 900 + (unsigned long)(esp_random() % 1200);
  }
  nextBlinkAt_ = now + gap;
}

void scheduleGaze(unsigned long now) {
  unsigned long gap = 1400 + (unsigned long)(esp_random() % 2800);
  nextGazeAt_ = now + gap;
  const uint32_t r = esp_random() % 100;
  if (r < 35) {
    gazeTarget_ = -0.85f;
  } else if (r < 70) {
    gazeTarget_ = 0.85f;
  } else {
    gazeTarget_ = 0.f;
  }
}

void drawBrow(int16_t cx, int16_t topY, int16_t halfW, int style) {
  // 0 normal, 1 raised, 2 frown, 3 sad, 4 happy
  int16_t y0 = topY;
  int16_t yL = topY;
  int16_t yR = topY;
  switch (style) {
    case 1:
      y0 = topY - 2;
      yL = topY - 3;
      yR = topY - 3;
      break;
    case 2:
      yL = topY - 1;
      yR = topY + 2;
      break;
    case 3:
      yL = topY + 2;
      yR = topY - 1;
      break;
    case 4:
      y0 = topY - 3;
      yL = topY - 2;
      yR = topY - 2;
      break;
    default:
      break;
  }
  disp_->drawLine(cx - halfW, yL, cx, y0, SSD1306_WHITE);
  disp_->drawLine(cx, y0, cx + halfW, yR, SSD1306_WHITE);
  disp_->drawLine(cx - halfW, yL + 1, cx, y0 + 1, SSD1306_WHITE);
  disp_->drawLine(cx, y0 + 1, cx + halfW, yR + 1, SSD1306_WHITE);
}

void drawMouth(int style, unsigned long now) {
  const int16_t mx = 64;
  const int16_t my = 42;
  if (style == 0) return;
  if (style == 3) {
    disp_->drawLine(mx - 8, my + 2, mx, my - 1, SSD1306_WHITE);
    disp_->drawLine(mx, my - 1, mx + 8, my + 2, SSD1306_WHITE);
    return;
  }
  int16_t bump = 0;
  if (style == 2) {
    bump = (int16_t)((now / 120) % 3);
  }
  const int16_t y = my - bump;
  disp_->drawLine(mx - 10, y, mx - 4, y + 3, SSD1306_WHITE);
  disp_->drawLine(mx - 4, y + 3, mx + 4, y + 3, SSD1306_WHITE);
  disp_->drawLine(mx + 4, y + 3, mx + 10, y, SSD1306_WHITE);
}

void drawOneEye(int16_t cx, int16_t cy, int16_t ew, int16_t eh, float openAmt,
                float gazeX, int browStyle, bool happyCrescent) {
  openAmt = clampf(openAmt, 0.f, 1.f);
  int16_t h = (int16_t)(eh * openAmt + 0.5f);
  if (h < 2) {
    disp_->drawFastHLine(cx - ew / 2, cy, ew, SSD1306_WHITE);
    drawBrow(cx, cy - eh / 2 - 4, ew / 2 - 1, browStyle);
    return;
  }

  if (happyCrescent) {
    disp_->fillRoundRect(cx - ew / 2, cy - h / 2, ew, h, h / 2, SSD1306_WHITE);
    disp_->fillRect(cx - ew / 2 - 1, cy - h / 2 - 1, ew + 2, h / 2 + 1, SSD1306_BLACK);
    drawBrow(cx, cy - eh / 2 - 5, ew / 2 - 1, browStyle);
    return;
  }

  disp_->fillRoundRect(cx - ew / 2, cy - h / 2, ew, h, h / 2, SSD1306_WHITE);

  const int16_t maxPx = (ew / 2) - 4;
  const int16_t maxPy = (h / 2) - 3;
  int16_t px = cx + (int16_t)(gazeX * maxPx);
  int16_t py = cy + (int16_t)(0.15f * maxPy);
  px = clampi(px, cx - maxPx, cx + maxPx);
  if (maxPy > 0) {
    py = clampi(py, cy - maxPy, cy + maxPy);
  }

  int16_t pr = h / 3;
  if (pr < 3) pr = 3;
  if (pr > 7) pr = 7;
  disp_->fillCircle(px, py, pr, SSD1306_BLACK);
  disp_->fillCircle(px - 1, py - 1, 1, SSD1306_WHITE);

  drawBrow(cx, cy - eh / 2 - 4, ew / 2 - 1, browStyle);
}

void render(unsigned long now) {
  if (disp_ == nullptr) return;

  disp_->clearDisplay();

  int16_t ew = 36;
  int16_t eh = 28;
  float baseOpen = 1.f;
  float gaze = gazeX_;
  int brow = 0;
  int mouth = 0;
  bool crescent = false;

  switch (mood_) {
    case FaceMood::Boot:
      baseOpen = 0.85f;
      break;
    case FaceMood::Idle:
      baseOpen = 1.f;
      break;
    case FaceMood::Sleepy:
      baseOpen = 0.45f;
      gaze *= 0.3f;
      break;
    case FaceMood::Listen:
      ew = 40;
      eh = 32;
      baseOpen = 1.f;
      brow = 1;
      gaze = 0.f;
      break;
    case FaceMood::Think:
      baseOpen = 0.75f;
      brow = 2;
      gaze = 0.55f;
      break;
    case FaceMood::Happy:
      baseOpen = 0.55f;
      brow = 4;
      gaze = 0.f;
      mouth = 2;
      crescent = true;
      break;
    case FaceMood::Sad:
      baseOpen = 0.65f;
      brow = 3;
      gaze = 0.f;
      mouth = 3;
      break;
  }

  float openAmt = baseOpen * (1.f - blink_);
  if (openAmt < 0.05f) openAmt = 0.05f;

  const int16_t cy = 22;
  drawOneEye(36, cy, ew, eh, openAmt, gaze, brow, crescent);
  drawOneEye(92, cy, ew, eh, openAmt, gaze, brow, crescent);
  drawMouth(mouth, now);

  disp_->drawFastHLine(0, 49, 128, SSD1306_WHITE);
  disp_->setTextSize(1);
  disp_->setTextColor(SSD1306_WHITE);
  disp_->setCursor(0, 52);
  disp_->print(status_);

  disp_->display();
  forceRedraw_ = false;
}

}  // namespace

void oledFaceBegin(Adafruit_SSD1306* display) {
  disp_ = display;
  mood_ = FaceMood::Boot;
  strncpy(status_, "Booting...", sizeof(status_) - 1);
  status_[sizeof(status_) - 1] = '\0';
  gazeX_ = 0.f;
  gazeTarget_ = 0.f;
  blink_ = 0.f;
  blinkPhase_ = 0;
  forceRedraw_ = true;
  const unsigned long now = millis();
  scheduleBlink(now);
  scheduleGaze(now);
  lastAnimMs_ = now;
}

void oledFaceSet(FaceMood mood, const char* status) {
  const bool moodChanged = (mood_ != mood);
  mood_ = mood;
  if (status != nullptr) {
    strncpy(status_, status, sizeof(status_) - 1);
    status_[sizeof(status_) - 1] = '\0';
  }
  if (moodChanged) {
    if (mood == FaceMood::Listen || mood == FaceMood::Happy ||
        mood == FaceMood::Sad) {
      gazeTarget_ = 0.f;
      gazeX_ = 0.f;
    }
    if (mood == FaceMood::Think) {
      gazeTarget_ = 0.55f;
    }
    blinkPhase_ = 0;
    blink_ = 0.f;
    scheduleBlink(millis());
  }
  forceRedraw_ = true;
}

void oledFaceTick(unsigned long nowMs) {
  if (disp_ == nullptr) return;
  if (!forceRedraw_ && (nowMs - lastAnimMs_ < kTickMs)) return;

  const float dt = (nowMs > lastAnimMs_) ? (nowMs - lastAnimMs_) / 1000.f : 0.06f;
  lastAnimMs_ = nowMs;

  if (mood_ == FaceMood::Idle || mood_ == FaceMood::Sleepy ||
      mood_ == FaceMood::Boot) {
    if (nowMs >= nextGazeAt_ && blinkPhase_ == 0) {
      scheduleGaze(nowMs);
    }
    const float speed = (mood_ == FaceMood::Sleepy) ? 1.2f : 2.4f;
    gazeX_ += (gazeTarget_ - gazeX_) * clampf(speed * dt, 0.f, 1.f);
  } else if (mood_ == FaceMood::Think) {
    gazeX_ += (0.55f - gazeX_) * clampf(3.f * dt, 0.f, 1.f);
  } else {
    gazeX_ += (0.f - gazeX_) * clampf(4.f * dt, 0.f, 1.f);
  }

  if (blinkPhase_ == 0 && nowMs >= nextBlinkAt_) {
    blinkPhase_ = 1;
  }
  if (blinkPhase_ == 1) {
    blink_ += (mood_ == FaceMood::Sleepy ? 5.f : 9.f) * dt;
    if (blink_ >= 1.f) {
      blink_ = 1.f;
      blinkPhase_ = 2;
    }
  } else if (blinkPhase_ == 2) {
    blink_ -= (mood_ == FaceMood::Sleepy ? 4.f : 8.f) * dt;
    if (blink_ <= 0.f) {
      blink_ = 0.f;
      blinkPhase_ = 0;
      scheduleBlink(nowMs);
    }
  } else {
    blink_ = 0.f;
  }

  render(nowMs);
}
