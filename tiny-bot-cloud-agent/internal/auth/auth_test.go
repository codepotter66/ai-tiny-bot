package auth

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/store"
)

func newSvc(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(context.Background(), filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	return New(st)
}

func setupDevice(t *testing.T, s *Service) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, s.st.CreateUser(ctx, &store.User{ID: "u1", DisplayName: "x"}))
	require.NoError(t, s.st.CreateDevice(ctx, &store.Device{
		ID: "tinypal-01", UserID: "u1", PairingCode: "ABC-123", Status: "unbound",
	}))
}

func TestProvision_Success(t *testing.T) {
	s := newSvc(t)
	setupDevice(t, s)
	tok, err := s.Provision(context.Background(), "tinypal-01", "ABC-123", false)
	require.NoError(t, err)
	assert.NotEmpty(t, tok)
}

func TestProvision_WrongCode(t *testing.T) {
	s := newSvc(t)
	setupDevice(t, s)
	_, err := s.Provision(context.Background(), "tinypal-01", "WRONG", false)
	assert.ErrorIs(t, err, ErrInvalidCode)
}

func TestProvision_UnknownDevice(t *testing.T) {
	s := newSvc(t)
	_, err := s.Provision(context.Background(), "nope", "x", false)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestProvision_AlreadyBound(t *testing.T) {
	s := newSvc(t)
	setupDevice(t, s)
	_, err := s.Provision(context.Background(), "tinypal-01", "ABC-123", false)
	require.NoError(t, err)
	_, err = s.Provision(context.Background(), "tinypal-01", "ABC-123", false)
	assert.ErrorIs(t, err, ErrAlreadyBound)
}

func TestProvision_ForceOnUnbound(t *testing.T) {
	// 固件 TB_PROVISION_FORCE=1 时，首次配网也会走 force → ReBindDevice
	s := newSvc(t)
	setupDevice(t, s)

	tok, err := s.Provision(context.Background(), "tinypal-01", "ABC-123", true)
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	ok, err := s.VerifyToken(context.Background(), "tinypal-01", tok)
	require.NoError(t, err)
	assert.True(t, ok, "unbound + force provision 后 token 必须能通过 VerifyToken")
}

func TestProvision_ForceReProvision(t *testing.T) {
	// 模拟"硬件 factory reset 后重配对"场景
	s := newSvc(t)
	setupDevice(t, s)
	tok1, err := s.Provision(context.Background(), "tinypal-01", "ABC-123", false)
	require.NoError(t, err)

	// 默认拒绝
	_, err = s.Provision(context.Background(), "tinypal-01", "ABC-123", false)
	assert.ErrorIs(t, err, ErrAlreadyBound)

	// force=true 重新签发，旧 token 失效
	tok2, err := s.Provision(context.Background(), "tinypal-01", "ABC-123", true)
	require.NoError(t, err)
	assert.NotEqual(t, tok1, tok2, "新 token 应该跟旧的不一样")

	// 新 token 有效
	ok, err := s.VerifyToken(context.Background(), "tinypal-01", tok2)
	require.NoError(t, err)
	assert.True(t, ok)

	// 旧 token 失效
	ok, err = s.VerifyToken(context.Background(), "tinypal-01", tok1)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestVerifyToken_RoundTrip(t *testing.T) {
	s := newSvc(t)
	setupDevice(t, s)
	tok, err := s.Provision(context.Background(), "tinypal-01", "ABC-123", false)
	require.NoError(t, err)

	ok, err := s.VerifyToken(context.Background(), "tinypal-01", tok)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = s.VerifyToken(context.Background(), "tinypal-01", "wrong-token")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestSubtleEqual(t *testing.T) {
	assert.True(t, subtleEqual("abc", "abc"))
	assert.False(t, subtleEqual("abc", "abd"))
	assert.False(t, subtleEqual("abc", "abcd"))
}
