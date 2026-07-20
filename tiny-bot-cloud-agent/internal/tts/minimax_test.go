package tts

import (
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiniMax_Name(t *testing.T) {
	m := NewMiniMax(MiniMaxConfig{APIKey: "sk-test"})
	assert.Equal(t, "minimax", m.Name())
}

func TestMiniMax_MissingAPIKey(t *testing.T) {
	m := NewMiniMax(MiniMaxConfig{})
	_, err := m.Synthesize(context.Background(), "hello", nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNotImplemented)
}

func TestMiniMax_EmptyText(t *testing.T) {
	m := NewMiniMax(MiniMaxConfig{APIKey: "sk-test"})
	rc, err := m.Synthesize(context.Background(), "   ", nil)
	require.NoError(t, err)
	defer rc.Close()
	buf := make([]byte, 1024)
	n, err := rc.Read(buf)
	assert.NoError(t, err)
	assert.Greater(t, n, 0) // 静音 ~32KB
}

func TestMiniMax_WebSocketConnect(t *testing.T) {
	// 起一个真的 wss test server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1) 验证 Authorization header
		auth := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer sk-test", auth)

		// 2) 升级到 WebSocket
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("accept: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		// 3) 读 task_start
		_, msg, err := conn.Read(context.Background())
		if err != nil {
			t.Logf("read task_start: %v", err)
			return
		}
		assert.Contains(t, string(msg), `"event":"task_start"`)
		assert.Contains(t, string(msg), `"model":"speech-2.8-turbo"`)
		assert.Contains(t, string(msg), `"voice_id":"male-qn-qingse"`)
		assert.Contains(t, string(msg), `"format":"pcm"`)
		assert.Contains(t, string(msg), `"sample_rate":16000`)

		// 4) 回 task_started
		_ = conn.Write(context.Background(), websocket.MessageText, []byte(`{"event":"task_started"}`))

		// 5) 读 task_continue
		_, msg2, _ := conn.Read(context.Background())
		assert.Contains(t, string(msg2), `"text":"hello"`)
		assert.Contains(t, string(msg2), `"event":"task_continue"`)

		// 6) 回一个 audio chunk + is_final
		pcm := []byte("fake-pcm-bytes-here")
		resp := `{"event":"task_continue","data":{"audio":"` + hex.EncodeToString(pcm) + `"},"is_final":true}`
		_ = conn.Write(context.Background(), websocket.MessageText, []byte(resp))

		// 7) 收 task_finish
		_, msg3, _ := conn.Read(context.Background())
		assert.Contains(t, string(msg3), `"event":"task_finish"`)
	}))
	defer srv.Close()

	// httptest 是 HTTP，客户端要连 ws://（不是 wss://）
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	m := &MiniMax{cfg: MiniMaxConfig{
		APIKey:     "sk-test",
		WSURL:      wsURL,
		Format:     "pcm",
		SampleRate: 16000,
		Model:      "speech-2.8-turbo",
		Voice:      "male-qn-qingse",
		Timeout:    5 * time.Second,
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := make(chan []byte, 4)
	errCh := make(chan error, 1)
	go func() {
		defer close(ch)
		errCh <- m.runSession(ctx, wsURL, "hello", nil, ch)
	}()

	var got []byte
	for chunk := range ch {
		got = append(got, chunk...)
	}
	assert.Equal(t, "fake-pcm-bytes-here", string(got))
	assert.NoError(t, <-errCh)
}

func TestMiniMax_TaskStartLanguageBoost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := websocket.Accept(w, r, nil)
		defer conn.Close(websocket.StatusNormalClosure, "")
		_, msg, _ := conn.Read(context.Background())
		body := string(msg)
		assert.Contains(t, body, `"language_boost":"Chinese,Yue"`)
		assert.Contains(t, body, `"voice_id":"Cantonese_GentleLady"`)
		_ = conn.Write(context.Background(), websocket.MessageText, []byte(`{"event":"task_started"}`))
		_, _, _ = conn.Read(context.Background())
		_ = conn.Write(context.Background(), websocket.MessageText, []byte(`{"is_final":true,"data":{}}`))
	}))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http://")

	m := NewMiniMax(MiniMaxConfig{
		APIKey:  "sk-test",
		WSURL:   wsURL,
		Timeout: 5 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rc, err := m.Synthesize(ctx, "你好", &SynthOptions{
		Voice:         "Cantonese_GentleLady",
		LanguageBoost: "Chinese,Yue",
	})
	require.NoError(t, err)
	defer rc.Close()
	_, _ = io.Copy(io.Discard, rc)
}

func TestMiniMax_CtxCancel(t *testing.T) {
	// 慢 server：1s 后才回 audio
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := websocket.Accept(w, r, nil)
		defer conn.Close(websocket.StatusNormalClosure, "")
		_ = conn.Write(context.Background(), websocket.MessageText, []byte(`{"event":"task_started"}`))
		_, _, _ = conn.Read(context.Background()) // task_continue
		time.Sleep(1 * time.Second)
		_ = conn.Write(context.Background(), websocket.MessageText, []byte(`{"is_final":true,"data":{}}`))
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	m := NewMiniMax(MiniMaxConfig{
		APIKey:  "sk-test",
		WSURL:   wsURL,
		Timeout: 5 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	rc, err := m.Synthesize(ctx, "hello", nil)
	require.NoError(t, err)
	defer rc.Close()

	// Read 应该立即返回 ctx error
	buf := make([]byte, 1024)
	_, err = rc.Read(buf)
	assert.Error(t, err)
}

func TestMiniMax_TaskFinishThenEOF(t *testing.T) {
	// server 收完 task_continue 就回 is_final=true + 空 audio
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _ := websocket.Accept(w, r, nil)
		defer conn.Close(websocket.StatusNormalClosure, "")
		_ = conn.Write(context.Background(), websocket.MessageText, []byte(`{"event":"task_started"}`))
		_, _, _ = conn.Read(context.Background()) // task_continue
		_ = conn.Write(context.Background(), websocket.MessageText, []byte(`{"is_final":true,"data":{}}`))
		_, _, _ = conn.Read(context.Background()) // task_finish
	}))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	m := NewMiniMax(MiniMaxConfig{
		APIKey:  "sk-test",
		WSURL:   wsURL,
		Timeout: 5 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rc, err := m.Synthesize(ctx, "hi", nil)
	require.NoError(t, err)
	defer rc.Close()

	buf := make([]byte, 1024)
	_, err = rc.Read(buf)
	assert.Equal(t, io.EOF, err)
}
