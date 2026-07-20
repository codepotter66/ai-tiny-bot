package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAICompat_ToolCallsSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"weather","arguments":""}}]},"finish_reason":null}]}`+"\n\n")
		flusher.Flush()
		_, _ = fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"杭州\"}"}}]},"finish_reason":null}]}`+"\n\n")
		flusher.Flush()
		_, _ = fmt.Fprint(w, `data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`+"\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	m := NewOpenAICompat(OpenAICompatConfig{
		BaseURL: srv.URL,
		APIKey:  "sk-test",
		Model:   "m-1",
		Timeout: 5 * time.Second,
	})

	var result StreamResult
	err := m.Chat(context.Background(),
		[]Message{{Role: RoleUser, Content: "杭州天气"}},
		[]ToolDef{{Name: "weather", Description: "查天气", Parameters: map[string]any{"type": "object"}}},
		nil, &result)
	require.NoError(t, err)
	assert.Equal(t, "tool_calls", result.FinishReason)
	require.Len(t, result.ToolCalls, 1)
	assert.Equal(t, "call_1", result.ToolCalls[0].ID)
	assert.Equal(t, "weather", result.ToolCalls[0].Name)
	assert.Contains(t, result.ToolCalls[0].Arguments, "杭州")
}

func TestToOpenAIMessages_ToolRoundTrip(t *testing.T) {
	msgs := toOpenAIMessages([]Message{
		{Role: RoleAssistant, Content: "", ToolCalls: []ToolCall{{ID: "c1", Name: "weather", Arguments: `{"city":"杭州"}`}}},
		{Role: RoleTool, Name: "c1", Content: "晴天"},
	})
	require.Len(t, msgs, 2)
	assert.Equal(t, "assistant", msgs[0]["role"])
	assert.NotNil(t, msgs[0]["tool_calls"])
	assert.Equal(t, "tool", msgs[1]["role"])
	assert.Equal(t, "c1", msgs[1]["tool_call_id"])
}
