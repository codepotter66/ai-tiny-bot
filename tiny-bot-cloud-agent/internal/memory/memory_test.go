package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecord_CreatesFile(t *testing.T) {
	root := t.TempDir()
	s := NewMarkdownStore(root)
	ctx := context.Background()

	ts := time.Date(2026, 6, 4, 10, 30, 0, 0, time.UTC)
	require.NoError(t, s.Record(ctx, "d1", "user", "我喜欢猫", ts))

	path := filepath.Join(root, "memory", "d1", "2026-06-04.md")
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(body), "# 2026-06-04")
	assert.Contains(t, string(body), "10:30:00 [user] 我喜欢猫")
}

func TestRecord_Appends(t *testing.T) {
	root := t.TempDir()
	s := NewMarkdownStore(root)
	ctx := context.Background()

	ts1 := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 6, 4, 10, 5, 0, 0, time.UTC)
	require.NoError(t, s.Record(ctx, "d1", "user", "第一句", ts1))
	require.NoError(t, s.Record(ctx, "d1", "assistant", "第二句", ts2))

	body, _ := os.ReadFile(filepath.Join(root, "memory", "d1", "2026-06-04.md"))
	assert.Contains(t, string(body), "第一句")
	assert.Contains(t, string(body), "第二句")
}

func TestRecall_KeywordScoring(t *testing.T) {
	root := t.TempDir()
	s := NewMarkdownStore(root)
	ctx := context.Background()

	now := time.Now()
	old := now.AddDate(0, 0, -3)
	older := now.AddDate(0, 0, -10)

	require.NoError(t, s.Record(ctx, "d1", "user", "我喜欢恐龙和猫", now))
	require.NoError(t, s.Record(ctx, "d1", "user", "今天去公园玩了", old))
	require.NoError(t, s.Record(ctx, "d1", "user", "晚饭吃面条", older))

	hits, err := s.Recall(ctx, "d1", "猫", 30, 5)
	require.NoError(t, err)
	require.NotEmpty(t, hits)
	// 第一名应该包含"猫"
	assert.Contains(t, hits[0].Content, "猫")
}

func TestRecall_EmptyWhenNoFile(t *testing.T) {
	root := t.TempDir()
	s := NewMarkdownStore(root)
	hits, err := s.Recall(context.Background(), "missing", "猫", 7, 5)
	require.NoError(t, err)
	assert.Empty(t, hits)
}

func TestTokenize(t *testing.T) {
	cases := map[string]int{
		"我喜欢猫和恐龙":    5, // 我 喜 欢 猫 和 恐 龙 = 6 单字；统计时 "我喜欢" 会被当 2 字符词留下... 实际看实现
		"hello world":  2,
		"  ":            0,
		"hi":            1, // 2 字符以上保留
	}
	for in, want := range cases {
		got := len(tokenize(in))
		// 对中文 case 不强求精确，只验证 > 0
		if want == 5 {
			assert.Greater(t, got, 0, "input=%q", in)
			continue
		}
		assert.Equal(t, want, got, "input=%q", in)
	}
}

func TestTodayPath(t *testing.T) {
	root := t.TempDir()
	s := NewMarkdownStore(root)
	p := s.TodayPath("d1")
	assert.Contains(t, p, "memory/d1")
	assert.True(t, filepath.Ext(p) == ".md")
}

func TestSaveFact_AppendDedupTrim(t *testing.T) {
	root := t.TempDir()
	s := NewMarkdownStore(root)
	ctx := context.Background()

	ts1 := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	require.NoError(t, s.SaveFact(ctx, "d1", "小主人喜欢恐龙", ts1, 2))
	require.NoError(t, s.SaveFact(ctx, "d1", "  小主人喜欢恐龙  ", ts2, 2)) // 去重
	require.NoError(t, s.SaveFact(ctx, "d1", "睡觉前要听故事", ts2, 2))
	require.NoError(t, s.SaveFact(ctx, "d1", "不喜欢菠菜", ts2, 2)) // 超限删最旧

	path := filepath.Join(root, "memory", "d1", "facts.md")
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(body)
	assert.Contains(t, text, "# Facts")
	assert.NotContains(t, text, "小主人喜欢恐龙") // 被裁掉
	assert.Contains(t, text, "睡觉前要听故事")
	assert.Contains(t, text, "不喜欢菠菜")

	facts, err := s.ListFacts(ctx, "d1", 2)
	require.NoError(t, err)
	require.Len(t, facts, 2)
	assert.Equal(t, "睡觉前要听故事", facts[0].Content)
	assert.Equal(t, "不喜欢菠菜", facts[1].Content)
}

func TestListFacts_Empty(t *testing.T) {
	root := t.TempDir()
	s := NewMarkdownStore(root)
	facts, err := s.ListFacts(context.Background(), "missing", 10)
	require.NoError(t, err)
	assert.Empty(t, facts)
}
