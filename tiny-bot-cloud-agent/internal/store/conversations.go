package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// OpenConversation 找 device 当前进行中的会话（ended_at=0），没有就新建。
func (s *Store) OpenConversation(ctx context.Context, deviceID string) (*Conversation, error) {
	row := s.RO.QueryRowContext(ctx,
		`SELECT id, device_id, started_at, COALESCE(ended_at, 0), turn_count
		 FROM conversations
		 WHERE device_id = ? AND ended_at = 0
		 ORDER BY started_at DESC LIMIT 1`, deviceID)
	var c Conversation
	if err := row.Scan(&c.ID, &c.DeviceID, &c.StartedAt, &c.EndedAt, &c.TurnCount); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	} else {
		return &c, nil
	}
	// 新建
	c.ID = newID()
	c.DeviceID = deviceID
	c.StartedAt = NowMs()
	c.TurnCount = 0
	if _, err := s.RW.ExecContext(ctx,
		`INSERT INTO conversations(id, device_id, started_at, ended_at, turn_count)
		 VALUES (?, ?, ?, 0, 0)`,
		c.ID, c.DeviceID, c.StartedAt); err != nil {
		return nil, fmt.Errorf("insert conversation: %w", err)
	}
	return &c, nil
}

// AppendMessage 写一条消息并把会话 turn_count +1。
func (s *Store) AppendMessage(ctx context.Context, convID string, m *Message) error {
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
		return fmt.Errorf("insert message: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE conversations SET turn_count = turn_count + 1 WHERE id = ?`, convID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("bump turn_count: %w", err)
	}
	return tx.Commit()
}

// ListMessages 拉一个会话的全部消息（按时间升序）。
func (s *Store) ListMessages(ctx context.Context, convID string) ([]*Message, error) {
	rows, err := s.RO.QueryContext(ctx,
		`SELECT id, conversation_id, role, content, COALESCE(tool_name, ''),
		        COALESCE(tool_args, ''), COALESCE(latency_ms, 0),
		        COALESCE(tokens_in, 0), COALESCE(tokens_out, 0), created_at
		 FROM messages WHERE conversation_id = ? ORDER BY created_at ASC`, convID)
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
	return out, rows.Err()
}

// CloseConversation 标记会话结束。
func (s *Store) CloseConversation(ctx context.Context, convID string) error {
	_, err := s.RW.ExecContext(ctx,
		`UPDATE conversations SET ended_at = ? WHERE id = ? AND ended_at = 0`,
		NowMs(), convID)
	return err
}
