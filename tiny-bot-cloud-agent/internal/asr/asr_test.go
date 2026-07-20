package asr

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMock_DefaultReply(t *testing.T) {
	m := NewMock("")
	r, err := m.Transcribe(context.Background(), bytes.NewReader([]byte{1, 2, 3, 4}))
	require.NoError(t, err)
	assert.Contains(t, r.Text, "mock")
	assert.Contains(t, r.Text, "4 字节")
}

func TestMock_FixedReply(t *testing.T) {
	m := NewMock("你好")
	r, err := m.Transcribe(context.Background(), bytes.NewReader(nil))
	require.NoError(t, err)
	assert.Equal(t, "你好", r.Text)
}

func TestMock_CtxCancel(t *testing.T) {
	m := &Mock{Reply: "x", EchoSeconds: 5}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := m.Transcribe(ctx, bytes.NewReader(nil))
	assert.Error(t, err)
}
