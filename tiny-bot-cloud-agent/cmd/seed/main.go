// Package main 是 seed 工具：在云端 SQLite 预登记用户/设备（device_id + pairing_code）。
//
// seed 是「入场登记」，不是让 ESP32 立刻在线。板子上电后仍需：
//   WiFi → POST /provision（同一 ID/码）→ 拿 token → WebSocket。
// 未 seed 时 provision 会 device not found（HTTP 401）。
//
// 用法（配置走 .env，seed 只需要 cli 参数）：
//
//	./bin/seed
//	./bin/seed -device-id tinypal-01 -display-name 小明 -pairing-code ABCD-1234
//	make seed                 # 本地 DB
//	make seed-remote          # 本机 SSH 到服务器容器内执行（线上）
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/config"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/logging"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/store"
)

func main() {
	var (
		deviceID     = flag.String("device-id", "tinypal-01", "device id")
		displayName  = flag.String("display-name", "小主人", "user display name")
		pairingCode  = flag.String("pairing-code", "", "pairing code (empty = auto-generate)")
		userID       = flag.String("user-id", "u-default", "user id")
		userMD       = flag.String("user-md", "", "optional per-user USER.md override (markdown text or @file path)")
		soulMD       = flag.String("soul-md", "", "optional per-user SOUL.md override (markdown text or @file path)")
	)
	flag.Parse()

	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(2)
	}
	logging.Setup(cfg.Logging.Level, cfg.Logging.Format)

	ctx := context.Background()
	s, err := store.Open(ctx, cfg.Storage.DBPath)
	if err != nil {
		slog.Error("open store", "err", err)
		os.Exit(1)
	}
	defer s.Close()

	// upsert user
	userMDText, err := resolveMDFlag(*userMD)
	if err != nil {
		slog.Error("read user-md", "err", err)
		os.Exit(1)
	}
	soulMDText, err := resolveMDFlag(*soulMD)
	if err != nil {
		slog.Error("read soul-md", "err", err)
		os.Exit(1)
	}
	if _, err := s.GetUser(ctx, *userID); err != nil {
		if err == store.ErrNotFound {
			u := &store.User{ID: *userID, DisplayName: *displayName, UserMD: userMDText, SoulMD: soulMDText}
			if err := s.CreateUser(ctx, u); err != nil {
				slog.Error("create user", "err", err)
				os.Exit(1)
			}
			slog.Info("user created", "id", u.ID, "name", u.DisplayName)
		} else {
			slog.Error("get user", "err", err)
			os.Exit(1)
		}
	} else {
		if userMDText != "" {
			if err := s.UpdateUserMD(ctx, *userID, userMDText); err != nil {
				slog.Error("update user_md", "err", err)
				os.Exit(1)
			}
			slog.Info("user_md updated", "id", *userID)
		}
		if soulMDText != "" {
			if err := s.UpdateSoulMD(ctx, *userID, soulMDText); err != nil {
				slog.Error("update soul_md", "err", err)
				os.Exit(1)
			}
			slog.Info("soul_md updated", "id", *userID)
		}
		if userMDText == "" && soulMDText == "" {
			slog.Info("user exists, skip", "id", *userID)
		}
	}

	// upsert device
	existing, err := s.GetDevice(ctx, *deviceID)
	if err != nil && err != store.ErrNotFound {
		slog.Error("get device", "err", err)
		os.Exit(1)
	}
	if existing != nil {
		slog.Info("device exists, skip",
			"id", existing.ID, "status", existing.Status,
			"pairing_code", existing.PairingCode)
		return
	}

	code := *pairingCode
	if code == "" {
		code = randomCode()
	}
	d := &store.Device{
		ID:          *deviceID,
		UserID:      *userID,
		PairingCode: code,
		Status:      "unbound",
	}
	if err := s.CreateDevice(ctx, d); err != nil {
		slog.Error("create device", "err", err)
		os.Exit(1)
	}
	slog.Info("device created",
		"id", d.ID, "pairing_code", d.PairingCode,
		"hint", "use this code with POST /provision?device_id="+d.ID+"&code="+d.PairingCode)
}

// randomCode 生成 XXXX-XXXX 形式的大写字母+数字配对码。
func randomCode() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	hex1 := hex.EncodeToString(b)
	_, _ = rand.Read(b)
	hex2 := hex.EncodeToString(b)
	return hex1[:4] + "-" + hex2[:4]
}

func resolveMDFlag(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", nil
	}
	if strings.HasPrefix(v, "@") {
		raw, err := os.ReadFile(strings.TrimPrefix(v, "@"))
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(raw)), nil
	}
	return v, nil
}
