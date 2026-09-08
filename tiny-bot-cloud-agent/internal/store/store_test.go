package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(context.Background(), filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestOpen_MigratesAndPersists(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "a.db")
	s, err := Open(context.Background(), dbPath)
	require.NoError(t, err)
	_ = s.Close()

	// 重新打开，验证 schema 已应用
	s, err = Open(context.Background(), dbPath)
	require.NoError(t, err)
	defer s.Close()

	row := s.RO.QueryRow("SELECT version FROM schema_version")
	var v int
	require.NoError(t, row.Scan(&v))
	assert.Equal(t, 2, v)
}

func TestUsersAndDevices_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	u := &User{ID: "u-1", DisplayName: "小明"}
	require.NoError(t, s.CreateUser(ctx, u))

	got, err := s.GetUser(ctx, "u-1")
	require.NoError(t, err)
	assert.Equal(t, "小明", got.DisplayName)
	assert.NotZero(t, got.CreatedAt)

	d := &Device{ID: "tinypal-01", UserID: "u-1", PairingCode: "ABCD-1234"}
	require.NoError(t, s.CreateDevice(ctx, d))

	byCode, err := s.GetDeviceByPairingCode(ctx, "ABCD-1234")
	require.NoError(t, err)
	assert.Equal(t, "tinypal-01", byCode.ID)
	assert.Equal(t, "unbound", byCode.Status)

	// Bind
	require.NoError(t, s.BindDevice(ctx, "tinypal-01", "deadbeef"))
	bound, err := s.GetDevice(ctx, "tinypal-01")
	require.NoError(t, err)
	assert.Equal(t, "bound", bound.Status)
	assert.Equal(t, "deadbeef", bound.TokenHash)
	assert.Empty(t, bound.PairingCode)
}

func TestSoulMD_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateUser(ctx, &User{ID: "u-soul", DisplayName: "小明"}))
	require.NoError(t, s.UpdateSoulMD(ctx, "u-soul", "# SOUL\n- 名字: 小星星"))
	require.NoError(t, s.UpdateUserMD(ctx, "u-soul", "# USER\n- 称呼: 小明"))

	got, err := s.GetUser(ctx, "u-soul")
	require.NoError(t, err)
	assert.Contains(t, got.SoulMD, "小星星")
	assert.Contains(t, got.UserMD, "小明")
}

func TestMemory_IndexAndSearch(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	// 用 today / yesterday 而不是 hardcoded 日期（避免 7 天后过期）
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	now := time.Now().UnixMilli()
	require.NoError(t, s.IndexMemoryLine(ctx, "d1", today, 1, now, "小主人喜欢猫和恐龙"))
	require.NoError(t, s.IndexMemoryLine(ctx, "d1", yesterday, 2, now-86_400_000, "今天去公园玩了"))
	hits, err := s.MemorySearch(ctx, "d1", 7)
	require.NoError(t, err)
	assert.Len(t, hits, 2)
}

func TestMemorySearchQuery_FiltersByKeywords(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	today := time.Now().Format("2006-01-02")
	now := time.Now().UnixMilli()
	require.NoError(t, s.IndexMemoryLine(ctx, "d1", today, 1, now, "小主人喜欢猫"))
	require.NoError(t, s.IndexMemoryLine(ctx, "d1", today, 2, now, "今天去公园玩了"))
	hits, err := s.MemorySearchQuery(ctx, "d1", "猫", 7)
	require.NoError(t, err)
	require.Len(t, hits, 1)
	assert.Equal(t, 1, hits[0].Line)
}

func TestExtractKeywords(t *testing.T) {
	cases := map[string]string{
		"小主人喜欢猫和恐龙":    "小 主 人 喜 欢 猫 和 恐 龙",
		"hello, world!":      "hello world",
		"今天去公园 2026年":    "今 天 去 公 园 2026 年",
		"  ":                  "",
	}
	for in, want := range cases {
		got := extractKeywords(in)
		assert.Equal(t, want, got, "input=%q", in)
	}
}
