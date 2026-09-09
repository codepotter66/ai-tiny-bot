package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound 仓储查不到记录的统一错误。
var ErrNotFound = errors.New("store: not found")

// CreateUser 新建用户。
func (s *Store) CreateUser(ctx context.Context, u *User) error {
	if u.CreatedAt == 0 {
		u.CreatedAt = NowMs()
	}
	u.UpdatedAt = NowMs()
	_, err := s.RW.ExecContext(ctx,
		`INSERT INTO users(id, display_name, user_md, soul_md, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.DisplayName, u.UserMD, u.SoulMD, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

// GetUser 取用户。
func (s *Store) GetUser(ctx context.Context, id string) (*User, error) {
	row := s.RO.QueryRowContext(ctx,
		`SELECT id, display_name, COALESCE(user_md, ''), COALESCE(soul_md, ''), created_at, updated_at
		 FROM users WHERE id = ?`, id)
	var u User
	if err := row.Scan(&u.ID, &u.DisplayName, &u.UserMD, &u.SoulMD, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// CreateDevice 新建设备（配网阶段用，pairing_code 必填，token_hash 为空）。
func (s *Store) CreateDevice(ctx context.Context, d *Device) error {
	if d.CreatedAt == 0 {
		d.CreatedAt = NowMs()
	}
	if d.Status == "" {
		d.Status = "unbound"
	}
	_, err := s.RW.ExecContext(ctx,
		`INSERT INTO devices(id, user_id, pairing_code, token_hash, status, hardware_rev, last_seen_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.UserID, d.PairingCode, d.TokenHash, d.Status, d.HardwareRev, d.LastSeenAt, d.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert device: %w", err)
	}
	return nil
}

// GetDevice 按 ID 取设备。
func (s *Store) GetDevice(ctx context.Context, id string) (*Device, error) {
	row := s.RO.QueryRowContext(ctx,
		`SELECT id, user_id, COALESCE(pairing_code, ''), COALESCE(token_hash, ''),
		        status, COALESCE(hardware_rev, ''), COALESCE(last_seen_at, 0), created_at
		 FROM devices WHERE id = ?`, id)
	var d Device
	if err := row.Scan(&d.ID, &d.UserID, &d.PairingCode, &d.TokenHash, &d.Status, &d.HardwareRev, &d.LastSeenAt, &d.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

// GetDeviceByPairingCode 配网阶段查找。
func (s *Store) GetDeviceByPairingCode(ctx context.Context, code string) (*Device, error) {
	row := s.RO.QueryRowContext(ctx,
		`SELECT id, user_id, COALESCE(pairing_code, ''), COALESCE(token_hash, ''),
		        status, COALESCE(hardware_rev, ''), COALESCE(last_seen_at, 0), created_at
		 FROM devices WHERE pairing_code = ?`, code)
	var d Device
	if err := row.Scan(&d.ID, &d.UserID, &d.PairingCode, &d.TokenHash, &d.Status, &d.HardwareRev, &d.LastSeenAt, &d.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

// BindDevice 配网完成：写入 token_hash，清空 pairing_code，置 status=bound。
func (s *Store) BindDevice(ctx context.Context, id, tokenHash string) error {
	res, err := s.RW.ExecContext(ctx,
		`UPDATE devices
		 SET token_hash = ?, pairing_code = NULL, status = 'bound', last_seen_at = ?
		 WHERE id = ? AND status = 'unbound'`,
		tokenHash, NowMs(), id)
	if err != nil {
		return fmt.Errorf("bind device: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("device %q not in unbound state", id)
	}
	return nil
}

// ReBindDevice 强制重新签发 token（force provision）。
// 不限制原 status：覆盖 token_hash，并置 status=bound。
// （若只更新 hash 不改 status，unbound + force=1 会导致 VerifyToken 一直失败。）
func (s *Store) ReBindDevice(ctx context.Context, id, tokenHash string) error {
	res, err := s.RW.ExecContext(ctx,
		`UPDATE devices
		 SET token_hash = ?, status = 'bound', last_seen_at = ?
		 WHERE id = ?`,
		tokenHash, NowMs(), id)
	if err != nil {
		return fmt.Errorf("rebind device: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("device %q not found", id)
	}
	return nil
}

// 网页 demo 专用账号/设备。与硬件 device_id 分开，避免 force 配网抢走音箱 token。
const (
	DemoUserID      = "u-demo"
	DemoDisplayName = "网页调试"
	DemoDeviceID    = "tinypal-demo"
	DemoPairingCode = "DEMO-1234"
)

// EnsureUserAndDevice 若用户或设备不存在则创建；已存在的设备一行都不改（含 pairing_code / token）。
func (s *Store) EnsureUserAndDevice(ctx context.Context, userID, displayName, deviceID, pairingCode string) error {
	if userID == "" || deviceID == "" {
		return fmt.Errorf("userID and deviceID required")
	}
	if _, err := s.GetUser(ctx, userID); err != nil {
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		if displayName == "" {
			displayName = userID
		}
		if err := s.CreateUser(ctx, &User{ID: userID, DisplayName: displayName}); err != nil {
			return err
		}
	}
	existing, err := s.GetDevice(ctx, deviceID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if existing != nil {
		return nil
	}
	if pairingCode == "" {
		return fmt.Errorf("pairingCode required for new device")
	}
	return s.CreateDevice(ctx, &Device{
		ID:          deviceID,
		UserID:      userID,
		PairingCode: pairingCode,
		Status:      "unbound",
	})
}

// EnsureDemoDevice 登记网页调试设备 tinypal-demo（幂等，不碰其它设备）。
func (s *Store) EnsureDemoDevice(ctx context.Context) error {
	return s.EnsureUserAndDevice(ctx, DemoUserID, DemoDisplayName, DemoDeviceID, DemoPairingCode)
}

// TouchDevice 更新 last_seen_at。
func (s *Store) TouchDevice(ctx context.Context, id string) error {
	_, err := s.RW.ExecContext(ctx,
		`UPDATE devices SET last_seen_at = ? WHERE id = ?`, NowMs(), id)
	return err
}
