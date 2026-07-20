package tts

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMock_EmptyTextShortPCM(t *testing.T) {
	m := NewMock()
	stream, err := m.Synthesize(context.Background(), "", nil)
	require.NoError(t, err)
	defer stream.Close()

	buf, err := io.ReadAll(stream)
	require.NoError(t, err)
	// 最短 0.3s @ 16kHz = 4800 samples × 2 bytes = 9600
	assert.GreaterOrEqual(t, len(buf), 9600)
}

func TestMock_LongerTextLongerPCM(t *testing.T) {
	m := NewMock()
	stream, err := m.Synthesize(context.Background(), "你好世界", nil)
	require.NoError(t, err)
	defer stream.Close()
	buf, err := io.ReadAll(stream)
	require.NoError(t, err)
	// 4 字 / 6 = 0.667s; 实际最小 0.3s 兜底
	// 确保比空文本长
	emptyStream, _ := m.Synthesize(context.Background(), "", nil)
	emptyBuf, _ := io.ReadAll(emptyStream)
	_ = emptyStream.Close()
	assert.Greater(t, len(buf), len(emptyBuf))
}

func TestMock_ContextCancel(t *testing.T) {
	m := &Mock{SampleRate: 16000, FreqHz: 440, Amplitude: 100, CharsPerSec: 6}
	// 让它"长得"足够我们取消
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := m.Synthesize(ctx, strings.Repeat("字", 1000), nil)
	_ = err
	if stream != nil {
		buf := make([]byte, 1024)
		cancel()
		_, _ = stream.Read(buf)
		stream.Close()
	}
}

func TestSilencePCM(t *testing.T) {
	b := SilencePCM(100)
	assert.Len(t, b, 16000*100/1000*2)
}

func TestBytesWithCancel(t *testing.T) {
	b := &bytesWithCancel{buf: []byte("hello"), ctx: context.Background()}
	out, err := io.ReadAll(b)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(out))
}

func TestMock_StreamIsReadCloser(t *testing.T) {
	m := NewMock()
	stream, err := m.Synthesize(context.Background(), "测试", nil)
	require.NoError(t, err)
	_, ok := stream.(io.ReadCloser)
	assert.True(t, ok)
	stream.Close()
	_ = time.Millisecond
	_ = bytes.NewReader(nil)
}
