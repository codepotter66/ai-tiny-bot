package tts

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAliyunTTS_Name(t *testing.T) {
	a := NewAliyun(AliyunConfig{Key: "ak", Secret: "sk", AppKey: "app"})
	assert.Equal(t, "aliyun", a.Name())
}

func TestAliyunTTS_MissingCreds(t *testing.T) {
	cases := []AliyunConfig{
		{Secret: "sk", AppKey: "app"},
		{Key: "ak", AppKey: "app"},
		{Key: "ak", Secret: "sk"},
	}
	for _, c := range cases {
		a := NewAliyun(c)
		_, err := a.Synthesize(context.Background(), "你好", nil)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrNotImplemented)
	}
}

func TestAliyunTTS_EmptyText(t *testing.T) {
	// 空文本：返回静音流，不发 HTTP 请求
	a := NewAliyun(AliyunConfig{Key: "ak", Secret: "sk", AppKey: "app"})
	stream, err := a.Synthesize(context.Background(), "   \n  ", nil)
	require.NoError(t, err)
	defer stream.Close()

	data, err := io.ReadAll(stream)
	require.NoError(t, err)
	// 静音 PCM：~32 KB
	assert.Greater(t, len(data), 30_000)
	assert.Equal(t, byte(0), data[0])
	assert.Equal(t, byte(0), data[len(data)-1])
}

func TestAliyunTTS_HTTPTest(t *testing.T) {
	// 假装 Aliyun TTS 返回固定 PCM bytes
	expectedPCM := []byte("fake pcm audio data here")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1) path
		assert.Equal(t, "/stream/v1/tts", r.URL.Path)
		// 2) query
		assert.Equal(t, "pcm", r.URL.Query().Get("format"))
		assert.Equal(t, "16000", r.URL.Query().Get("sample_rate"))
		assert.Equal(t, "test-app", r.URL.Query().Get("appkey"))
		assert.Equal(t, "你好世界", r.URL.Query().Get("text"))
		assert.NotEmpty(t, r.URL.Query().Get("Signature"))
		// 3) 返回 PCM 字节流
		w.Header().Set("Content-Type", "audio/pcm")
		_, _ = w.Write(expectedPCM)
	}))
	defer srv.Close()

	srvURL := strings.TrimPrefix(srv.URL, "http://")
	a := NewAliyun(AliyunConfig{
		Key: "testak", Secret: "testsk", AppKey: "test-app",
		Host: "http://" + srvURL,
		HTTP: srv.Client(),
	})
	stream, err := a.Synthesize(context.Background(), "你好世界", nil)
	require.NoError(t, err)
	defer stream.Close()

	data, err := io.ReadAll(stream)
	require.NoError(t, err)
	assert.Equal(t, expectedPCM, data, "TTS 流应该原样返回 server 给的 PCM bytes")
}

func TestAliyunTTS_CtxCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 等 ctx cancel
		<-r.Context().Done()
	}))
	defer srv.Close()

	srvURL := strings.TrimPrefix(srv.URL, "http://")
	a := NewAliyun(AliyunConfig{
		Key: "ak", Secret: "sk", AppKey: "app",
		Host: "http://" + srvURL,
		HTTP: srv.Client(),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := a.Synthesize(ctx, "你好", nil)
	assert.Error(t, err)
}

func TestAliyunTTS_HTTPStatusNot2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`internal error`))
	}))
	defer srv.Close()

	srvURL := strings.TrimPrefix(srv.URL, "http://")
	a := NewAliyun(AliyunConfig{
		Key: "ak", Secret: "sk", AppKey: "app",
		Host: "http://" + srvURL,
		HTTP: srv.Client(),
	})
	_, err := a.Synthesize(context.Background(), "你好", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
