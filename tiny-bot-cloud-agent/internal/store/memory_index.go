package store

import (
	"context"
	"fmt"
	"strings"
)

// IndexMemoryLine 把一行记忆写入索引表。
//  - deviceID: 设备 ID
//  - date: YYYY-MM-DD
//  - lineNo: 文件内行号（从 1 开始）
//  - tsMs: 该行时间戳
//  - content: 行原文（内部会分词去停用词）
func (s *Store) IndexMemoryLine(ctx context.Context, deviceID, date string, lineNo int, tsMs int64, content string) error {
	kw := extractKeywords(content)
	if kw == "" {
		return nil
	}
	_, err := s.RW.ExecContext(ctx,
		`INSERT OR REPLACE INTO memory_index(device_id, date, line_no, ts_ms, kw)
		 VALUES (?, ?, ?, ?, ?)`,
		deviceID, date, lineNo, tsMs, kw)
	if err != nil {
		return fmt.Errorf("index memory line: %w", err)
	}
	return nil
}

// RecallKeywords 取 device 最近 N 天内、与 query 共享关键词的行。
// 返回 (date, line_no, ts_ms, kw) 元组。
type MemoryHit struct {
	Date  string
	Line  int
	TSMs  int64
	KW    string
	Score float64
}

// MemorySearch 简单 LIKE 查询，召回候选；最终打分由 recall.go 决定。
func (s *Store) MemorySearch(ctx context.Context, deviceID string, lookbackDays int) ([]MemoryHit, error) {
	rows, err := s.RO.QueryContext(ctx,
		`SELECT date, line_no, ts_ms, kw
		 FROM memory_index
		 WHERE device_id = ?
		   AND date >= date('now', ?)
		 ORDER BY ts_ms DESC`,
		deviceID, fmt.Sprintf("-%d days", lookbackDays))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemoryHit
	for rows.Next() {
		var h MemoryHit
		if err := rows.Scan(&h.Date, &h.Line, &h.TSMs, &h.KW); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// MemorySearchQuery 在 lookback 内按 query 关键词过滤 memory_index。
func (s *Store) MemorySearchQuery(ctx context.Context, deviceID, query string, lookbackDays int) ([]MemoryHit, error) {
	all, err := s.MemorySearch(ctx, deviceID, lookbackDays)
	if err != nil {
		return nil, err
	}
	q := strings.Fields(extractKeywords(query))
	if len(q) == 0 {
		return nil, nil
	}
	var out []MemoryHit
	for _, h := range all {
		kwPad := " " + h.KW + " "
		for _, tok := range q {
			if tok == "" {
				continue
			}
			if strings.Contains(kwPad, " "+tok+" ") {
				out = append(out, h)
				break
			}
		}
	}
	return out, nil
}

// extractKeywords 取中文 + 英文 + 数字 token，2 字符以上。
// 极简实现：空格、标点分词；中文按字符切。
func extractKeywords(s string) string {
	s = strings.ToLower(s)
	// 中英标点替换为空格
	repl := strings.NewReplacer(
		"、", " ", "，", " ", "。", " ", "！", " ", "？", " ",
		"；", " ", "：", " ", "（", " ", "）", " ", "《", " ", "》", " ",
		"\"", " ", "'", " ", "\n", " ", "\t", " ",
		",", " ", ".", " ", ";", " ", ":", " ", "!", " ", "?", " ",
		"(", " ", ")", " ", "[", " ", "]", " ",
	)
	s = repl.Replace(s)
	// 拆字为单字（中文字符）+ 英文数字单词
	var tokens []string
	var buf []rune
	flush := func() {
		if len(buf) > 0 {
			t := string(buf)
			if len([]rune(t)) >= 2 {
				tokens = append(tokens, t)
			}
			buf = buf[:0]
		}
	}
	for _, r := range s {
		switch {
		case r >= 0x4e00 && r <= 0x9fff: // CJK Unified Ideographs
			flush()
			tokens = append(tokens, string(r)) // 单字
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			buf = append(buf, r)
		default:
			flush()
		}
	}
	flush()
	return strings.Join(tokens, " ")
}
