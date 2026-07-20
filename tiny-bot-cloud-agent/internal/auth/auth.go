// Package auth 处理设备鉴权与首次配网。
//
// v1 设计：
//   - devices 表里 pairing_code 一栏一码（未签发 token 时）
//   - 配网：POST /provision?device_id=xxx&code=yyy → 返回 32 字节随机 token（明文）
//   - WebSocket hello：携带该 token，sha256 哈希与 devices.token_hash 比对
//   - 长期 token 存固件 NVS，重启后用同一 token 即可
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/store"
)

// TokenBytes token 随机字节数（256 bit = 32 字节）。
const TokenBytes = 32

// ErrNotFound 设备不存在。
var ErrNotFound = errors.New("auth: device not found")

// ErrInvalidCode 配对码错误。
var ErrInvalidCode = errors.New("auth: invalid pairing code")

// ErrInvalidToken token 错误。
var ErrInvalidToken = errors.New("auth: invalid token")

// ErrAlreadyBound 设备已绑（不能再次用 pairing code 配网，除非 force）。
var ErrAlreadyBound = errors.New("auth: device already bound")

// Service auth 服务。
type Service struct {
	st *store.Store
}

// New 构造 auth service。
func New(st *store.Store) *Service { return &Service{st: st} }

// Provision 用 pairing code 换 token，并把 device 标记为 bound。
//
// 行为：
//   - unbound 设备：必须配对码正确，签发 token，标记 bound
//   - bound 设备：默认拒绝（ErrAlreadyBound）
//   - bound 设备 + force=true：跳过 pairing_code 检查（设备丢 token 后重发）
//
// force 场景：硬件 factory reset / localStorage 清空 / 调试。
// 注意：force=true 意味着"知道 device_id 就能重发 token"，对单 VPS 自部署足够安全。
func (s *Service) Provision(ctx context.Context, deviceID, code string, force bool) (string, error) {
	d, err := s.st.GetDevice(ctx, deviceID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	if d.Status == "bound" && !force {
		return "", ErrAlreadyBound
	}
	if !force && d.PairingCode != code {
		return "", ErrInvalidCode
	}
	raw := make([]byte, TokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	token := hex.EncodeToString(raw)
	hash := hashToken(token)
	if force {
		if err := s.st.ReBindDevice(ctx, deviceID, hash); err != nil {
			return "", err
		}
	} else {
		if err := s.st.BindDevice(ctx, deviceID, hash); err != nil {
			return "", err
		}
	}
	return token, nil
}

// VerifyToken 校验 device 的 token 是否有效。
// 返回 true 表示通过，false 表示失败。
func (s *Service) VerifyToken(ctx context.Context, deviceID, token string) (bool, error) {
	if deviceID == "" || token == "" {
		return false, nil
	}
	d, err := s.st.GetDevice(ctx, deviceID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	if d.Status != "bound" || d.TokenHash == "" {
		return false, nil
	}
	return subtleEqual(d.TokenHash, hashToken(token)), nil
}

// hashToken 把 token 做 SHA-256 摘要（hex）。
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// subtleEqual 防止时序攻击。
func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// EnsureLoaded helper：在 WebSocket hello 之前确保 device 已被加载（避免冷启动慢）。
func (s *Service) EnsureLoaded(ctx context.Context, deviceID string) error {
	_, err := s.st.GetDevice(ctx, deviceID)
	if err != nil {
		return err
	}
	time.Sleep(0) // 占位，便于将来加 cache
	return nil
}
