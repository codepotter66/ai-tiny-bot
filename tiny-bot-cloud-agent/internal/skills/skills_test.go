package skills

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeExampleSkill 在 tmp 下生成一个完整 skill 目录
func makeExampleSkill(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "skills", "weather")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "scripts"), 0o755))
	md := `---
name: weather
description: 查询某城市天气
parameters:
  city:
    type: string
    description: 城市名
    required: true
---
# weather skill body
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "scripts", "run.sh"),
		[]byte("#!/bin/sh\ncat\n"), 0o755))
}

func TestLoadAll_Success(t *testing.T) {
	root := t.TempDir()
	makeExampleSkill(t, root)
	all, err := LoadAll(root)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "weather", all[0].Name)
	assert.Equal(t, "查询某城市天气", all[0].Description)
	assert.True(t, all[0].Parameters["city"].Required)
}

func TestLoadAll_NoSkillsDir(t *testing.T) {
	root := t.TempDir()
	all, err := LoadAll(root)
	require.NoError(t, err)
	assert.Empty(t, all)
}

func TestParseSkillMD_MissingFrontmatter(t *testing.T) {
	_, err := parseSkillMD([]byte("no frontmatter here"))
	assert.Error(t, err)
}

func TestParseSkillMD_NoClosing(t *testing.T) {
	_, err := parseSkillMD([]byte("---\nname: x\nno close"))
	assert.Error(t, err)
}

func TestParseSkillMD_NoName(t *testing.T) {
	_, err := parseSkillMD([]byte("---\ndescription: x\n---\nbody"))
	assert.Error(t, err)
}

func TestRun_ReadsStdinAndReturnsStdout(t *testing.T) {
	root := t.TempDir()
	makeExampleSkill(t, root)
	all, err := LoadAll(root)
	require.NoError(t, err)
	s := all[0]

	res, err := Run(context.Background(), s, `{"city":"杭州"}`)
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
	// run.sh 是 `cat` — 会把 stdin 写到 stdout
	assert.Contains(t, res.Stdout, "city")
}

func TestRun_Timeout(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skills", "slow")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "scripts"), 0o755))
	md := "---\nname: slow\ndescription: a\n---\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "scripts", "run.sh"),
		[]byte("#!/bin/sh\nsleep 60\n"), 0o755))

	all, err := LoadAll(root)
	require.NoError(t, err)
	res, err := Run(context.Background(), all[0], "")
	// 命令被超时 kill，非 0 退出码
	require.NoError(t, err)
	assert.NotEqual(t, 0, res.ExitCode)
}

func TestRegistry_AddGetList(t *testing.T) {
	r := NewRegistry()
	root := t.TempDir()
	makeExampleSkill(t, root)
	all, err := LoadAll(root)
	require.NoError(t, err)
	require.NoError(t, r.Add(all[0]))

	got, ok := r.Get("weather")
	require.True(t, ok)
	assert.Equal(t, "weather", got.Name)

	assert.Equal(t, []string{"weather"}, r.Names())
	defs := r.ToolDefs()
	require.Len(t, defs, 1)
	assert.Equal(t, "weather", defs[0].Name)
}

func TestRegistry_AddDuplicateFails(t *testing.T) {
	r := NewRegistry()
	root := t.TempDir()
	makeExampleSkill(t, root)
	all, _ := LoadAll(root)
	require.NoError(t, r.Add(all[0]))
	err := r.Add(all[0])
	assert.Error(t, err)
}

func TestLoadFromDir(t *testing.T) {
	root := t.TempDir()
	makeExampleSkill(t, root)
	r, err := LoadFromDir(root)
	require.NoError(t, err)
	assert.Equal(t, []string{"weather"}, r.Names())
}

func TestEncodeArgs(t *testing.T) {
	s, err := EncodeArgs(map[string]interface{}{"city": "杭州"})
	require.NoError(t, err)
	assert.Equal(t, `{"city":"杭州"}`, s)
}
