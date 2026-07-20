package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveWorkspaceMarkdownPath_OK(t *testing.T) {
	root := t.TempDir()
	abs, err := resolveWorkspaceMarkdownPath(root, "SOUL.md")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "SOUL.md"), abs)

	abs, err = resolveWorkspaceMarkdownPath(root, "memory/d1/facts.md")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "memory", "d1", "facts.md"), abs)

	abs, err = resolveWorkspaceMarkdownPath(root, "skills/weather/SKILL.md")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "skills", "weather", "SKILL.md"), abs)
}

func TestResolveWorkspaceMarkdownPath_Reject(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		"../etc/passwd",
		"SOUL.txt",
		"skills/x/run.sh",
		"skills/x/y/SKILL.md",
		"secret.md",
		"",
		"memory/../../SOUL.md",
	}
	for _, c := range cases {
		_, err := resolveWorkspaceMarkdownPath(root, c)
		assert.Error(t, err, "path=%q", c)
	}
}

func TestListAndWriteWorkspaceMarkdown(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "SOUL.md"), []byte("# soul"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "memory", "d1"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "memory", "d1", "facts.md"), []byte("- a"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "skills", "weather"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "skills", "weather", "SKILL.md"), []byte("# skill"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "secret.md"), []byte("no"), 0o644))

	files, err := listWorkspaceMarkdown(root)
	require.NoError(t, err)
	assert.Contains(t, files, "SOUL.md")
	assert.Contains(t, files, "memory/d1/facts.md")
	assert.Contains(t, files, "skills/weather/SKILL.md")
	assert.NotContains(t, files, "secret.md")

	abs, err := resolveWorkspaceMarkdownPath(root, "AGENT.md")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(abs, []byte("# agent\n"), 0o644))
	body, err := os.ReadFile(abs)
	require.NoError(t, err)
	assert.Contains(t, string(body), "# agent")
}
