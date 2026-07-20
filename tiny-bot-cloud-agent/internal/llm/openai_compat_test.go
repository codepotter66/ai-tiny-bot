package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAICompat_Name(t *testing.T) {
	m := NewOpenAICompat(OpenAICompatConfig{BaseURL: "https://x", APIKey: "k", Model: "m"})
	assert.Equal(t, "openai_compat", m.Name())
}

func TestOpenAICompat_MissingAPIKey(t *testing.T) {
	m := NewOpenAICompat(OpenAICompatConfig{BaseURL: "https://x", Model: "m"})
	err := m.Chat(context.Background(), nil, nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "APIKey")
}

func TestOpenAICompat_MissingBaseURL(t *testing.T) {
	m := NewOpenAICompat(OpenAICompatConfig{APIKey: "k", Model: "m"})
	err := m.Chat(context.Background(), nil, nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BaseURL")
}

func TestOpenAICompat_StreamsSSE(t *testing.T) {
	// 模拟 OpenAI chat completions 流式响应
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1) 验证请求
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

		// 2) 验证 body 包含 model + messages + stream
		body := make([]byte, 4096)
		n, _ := r.Body.Read(body)
		s := string(body[:n])
		assert.Contains(t, s, `"model":"m-1"`)
		assert.Contains(t, s, `"stream":true`)
		assert.Contains(t, s, `"role":"user"`)
		assert.Contains(t, s, `"content":"hi"`)

		// 3) 返回 SSE 流
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for _, piece := range []string{"你", "好", "世界"} {
			_, _ = fmt.Fprintf(w, `data: {"choices":[{"delta":{"content":"%s"},"finish_reason":null}]}`+"\n\n", piece)
			flusher.Flush()
		}
		_, _ = fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`+"\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	wsURL := strings.TrimPrefix(srv.URL, "http://")
	m := NewOpenAICompat(OpenAICompatConfig{
		BaseURL: "http://" + wsURL,
		APIKey:  "sk-test",
		Model:   "m-1",
		Timeout: 5 * time.Second,
	})

	var got strings.Builder
	var endTurnSeen bool
	err := m.Chat(context.Background(),
		[]Message{{Role: RoleUser, Content: "hi"}},
		nil,
		func(token string, endTurn bool) error {
			if endTurn {
				endTurnSeen = true
				return nil
			}
			got.WriteString(token)
			return nil
		}, nil)
	require.NoError(t, err)
	assert.Equal(t, "你好世界", got.String())
	assert.True(t, endTurnSeen, "should signal endTurn at the end of stream")
}

func TestOpenAICompat_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()
	wsURL := strings.TrimPrefix(srv.URL, "http://")
	m := NewOpenAICompat(OpenAICompatConfig{
		BaseURL: "http://" + wsURL,
		APIKey:  "sk-bad",
		Model:   "m",
		Timeout: 5 * time.Second,
	})
	err := m.Chat(context.Background(), []Message{{Role: RoleUser, Content: "x"}}, nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status=401")
}

// TestOpenAICompat_CtxCancel 不在单测里：httptest.Server.Close() 等所有 handler 返回，
// 写阻塞 handler 会让 server.Close 死锁（go test 默认 10min 超时）。
// 生产环境 ctx cancel 走 http.Client 的 request context，会立刻返回 error。
// 单测覆盖：HTTP 4xx（TestOpenAICompat_HTTPError）已经能验证错误路径。
