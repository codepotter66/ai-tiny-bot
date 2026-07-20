package persona

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeAll(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"SOUL.md", "IDENTITY.md", "AGENT.md", "USER.md"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("# "+name+"\ncontent "+name), 0o600))
	}
}

func TestLoad_Success(t *testing.T) {
	dir := t.TempDir()
	writeAll(t, dir)
	p, err := Load(dir)
	require.NoError(t, err)
	assert.Contains(t, p.Soul, "SOUL.md")
	assert.Contains(t, p.Identity, "IDENTITY.md")
	assert.Contains(t, p.Agent, "AGENT.md")
	assert.Contains(t, p.User, "USER.md")
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("x"), 0o600))
	_, err := Load(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "IDENTITY.md")
}

func TestSystemPrompt_Order(t *testing.T) {
	dir := t.TempDir()
	writeAll(t, dir)
	p, err := Load(dir)
	require.NoError(t, err)
	s := p.SystemPrompt()
	// 顺序 SOUL → IDENTITY → AGENT → USER
	assert.True(t, strings.Index(s, "SOUL.md") < strings.Index(s, "IDENTITY.md"))
	assert.True(t, strings.Index(s, "IDENTITY.md") < strings.Index(s, "AGENT.md"))
	assert.True(t, strings.Index(s, "AGENT.md") < strings.Index(s, "USER.md"))
}

func TestReloader_Reload(t *testing.T) {
	dir := t.TempDir()
	writeAll(t, dir)
	r, err := NewReloader(dir)
	require.NoError(t, err)

	// 改 SOUL
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("v2"), 0o600))
	require.NoError(t, r.Reload())
	assert.Equal(t, "v2", r.Get().Soul)
}

func TestReloader_ReloadFailure_PreservesOld(t *testing.T) {
	dir := t.TempDir()
	writeAll(t, dir)
	r, err := NewReloader(dir)
	require.NoError(t, err)
	old := r.Get().Soul

	// 删一个文件，Reload 应失败但旧值保留
	require.NoError(t, os.Remove(filepath.Join(dir, "USER.md")))
	err = r.Reload()
	assert.Error(t, err)
	assert.Equal(t, old, r.Get().Soul)
}
