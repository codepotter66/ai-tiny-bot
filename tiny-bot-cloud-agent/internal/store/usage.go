package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// SumDeviceTokensToday 统计设备今日 messages 表的 token 用量。
func (s *Store) SumDeviceTokensToday(ctx context.Context, deviceID string) (tokensIn, tokensOut int, err error) {
	start := startOfDayMs()
	row := s.RO.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(m.tokens_in), 0), COALESCE(SUM(m.tokens_out), 0)
		 FROM messages m
		 JOIN conversations c ON m.conversation_id = c.id
		 WHERE c.device_id = ? AND m.created_at >= ?`,
		deviceID, start)
	if err := row.Scan(&tokensIn, &tokensOut); err != nil {
		return 0, 0, err
	}
	return tokensIn, tokensOut, nil
}

// UpdateUserMD 更新用户画像 Markdown（覆盖 USER.md）。
func (s *Store) UpdateUserMD(ctx context.Context, userID, userMD string) error {
	_, err := s.RW.ExecContext(ctx,
		`UPDATE users SET user_md = ?, updated_at = ? WHERE id = ?`,
		userMD, NowMs(), userID)
	return err
}

// UpdateSoulMD 更新人设 Markdown（覆盖 SOUL.md）。
func (s *Store) UpdateSoulMD(ctx context.Context, userID, soulMD string) error {
	_, err := s.RW.ExecContext(ctx,
		`UPDATE users SET soul_md = ?, updated_at = ? WHERE id = ?`,
		soulMD, NowMs(), userID)
	return err
}

// ListRecentMessages 拉设备当前 open conversation 的最近 N 条消息（时间升序）。
func (s *Store) ListRecentMessages(ctx context.Context, deviceID string, limit int) ([]*Message, error) {
	conv, err := s.OpenConversation(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	rows, err := s.RO.QueryContext(ctx,
		`SELECT id, conversation_id, role, content, COALESCE(tool_name, ''),
		        COALESCE(tool_args, ''), COALESCE(latency_ms, 0),
		        COALESCE(tokens_in, 0), COALESCE(tokens_out, 0), created_at
		 FROM messages WHERE conversation_id = ?
		 ORDER BY created_at DESC LIMIT ?`, conv.ID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.ToolName, &m.ToolArgs, &m.LatencyMs, &m.TokensIn, &m.TokensOut, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// reverse to ascending
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// AppendMessageWithTokens 写消息并记录 token 用量。
func (s *Store) AppendMessageWithTokens(ctx context.Context, convID string, m *Message) error {
	if m.ID == "" {
		m.ID = newID()
	}
	if m.CreatedAt == 0 {
		m.CreatedAt = NowMs()
	}
	tx, err := s.RW.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO messages(id, conversation_id, role, content, tool_name, tool_args, latency_ms, tokens_in, tokens_out, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, convID, m.Role, m.Content, m.ToolName, m.ToolArgs, m.LatencyMs, m.TokensIn, m.TokensOut, m.CreatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE conversations SET turn_count = turn_count + 1 WHERE id = ?`, convID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// GetOpenConversationID 返回设备当前 open conversation id，没有则空。
func (s *Store) GetOpenConversationID(ctx context.Context, deviceID string) (string, error) {
	row := s.RO.QueryRowContext(ctx,
		`SELECT id FROM conversations WHERE device_id = ? AND ended_at = 0 ORDER BY started_at DESC LIMIT 1`,
		deviceID)
	var id string
	if err := row.Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return id, nil
}

func startOfDayMs() int64 {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return start.UnixMilli()
}
