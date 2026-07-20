package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupWithWriter_JSON(t *testing.T) {
	var buf bytes.Buffer
	SetupWithWriter(&buf, "info", "json")
	slog := slogDefault()
	slog.Info("hello", "k", "v")

	var out map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	assert.Equal(t, "hello", out["msg"])
	assert.Equal(t, "v", out["k"])
	assert.Equal(t, "INFO", out["level"])
}

func TestSetupWithWriter_Text(t *testing.T) {
	var buf bytes.Buffer
	SetupWithWriter(&buf, "debug", "text")
	slog := slogDefault()
	slog.Debug("dbg")

	out := buf.String()
	assert.True(t, strings.Contains(out, "dbg"), "expected 'dbg' in %q", out)
}

func TestParseLevel(t *testing.T) {
	cases := map[string]int{
		"debug": -4,
		"info":  0,
		"warn":  4,
		"error": 8,
		"":      0,
		"weird": 0,
	}
	for in, want := range cases {
		got := int(parseLevel(in))
		assert.Equal(t, want, got, "input=%q", in)
	}
}
