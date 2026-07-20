/**
 * OLED 表情脸：128×64 上半屏矢量眼睛，底栏一行状态。
 */
#pragma once

#include <Adafruit_SSD1306.h>

enum class FaceMood {
  Boot,
  Idle,
  Sleepy,
  Listen,
  Think,
  Happy,
  Sad,
};

void oledFaceBegin(Adafruit_SSD1306* display);
void oledFaceSet(FaceMood mood, const char* status);
void oledFaceTick(unsigned long nowMs);
