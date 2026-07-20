package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAICompatConfig OpenAI 兼容协议 LLM 的运行时参数。
// 适用于：MiniMax M3、OpenAI GPT、DeepSeek、Anthropic-via-proxy 等。
type OpenAICompatConfig struct {
	BaseURL   string        // 例如 https://api.minimaxi.com / https://api.openai.com
	APIKey    string        // Bearer token
	Model     string        // 模型名，例如 MiniMax-M3 / MiniMax-M2.5 / gpt-4o-mini
	MaxTokens int           // max_tokens，0 表示不传
	HTTP      *http.Client  // 测试时注入
	Timeout   time.Duration // 单次请求总超时，默认 60s
}

// OpenAICompat 实现 Chat 接口（OpenAI 兼容流式 chat completions）。
type OpenAICompat struct {
	cfg OpenAICompatConfig
}

// NewOpenAICompat 构造 provider。Timeout 缺失时填默认。
func NewOpenAICompat(cfg OpenAICompatConfig) *OpenAICompat {
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: cfg.Timeout}
	}
	return &OpenAICompat{cfg: cfg}
}

func (m *OpenAICompat) Name() string { return "openai_compat" }

// Chat 流式调用 chat completions。每次 delta 通过 onToken 回调。
// stop_reason=end_turn / stop / tool_calls / length → endTurn=true。
func (m *OpenAICompat) Chat(ctx context.Context, msgs []Message, tools []ToolDef, onToken OnToken, result *StreamResult) error {
	if m.cfg.APIKey == "" {
		return fmt.Errorf("%w: openai_compat requires APIKey (set TB_OPENAI_COMPAT_API_KEY)", ErrNotImplemented)
	}
	if m.cfg.BaseURL == "" {
		return fmt.Errorf("%w: openai_compat requires BaseURL (set TB_OPENAI_COMPAT_BASE_URL)", ErrNotImplemented)
	}

	reqBody := map[string]any{
		"model":    m.cfg.Model,
		"messages": toOpenAIMessages(msgs),
		"stream":   true,
		// OpenAI 兼容流式默认不回 usage；显式请求以便日限额统计
		"stream_options": map[string]any{"include_usage": true},
	}
	if m.cfg.MaxTokens > 0 {
		reqBody["max_tokens"] = m.cfg.MaxTokens
	}
	if len(tools) > 0 {
		reqBody["tools"] = toOpenAITools(tools)
	}
	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("openai_compat: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(m.cfg.BaseURL, "/")+"/v1/chat/completions",
		bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("openai_compat: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.cfg.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := m.cfg.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("openai_compat: http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("openai_compat: status=%d body=%s",
			resp.StatusCode, truncateBody(body, 300))
	}

	var acc streamAccumulator
	reader := bufio.NewReader(resp.Body)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("openai_compat: read sse: %w", err)
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		payload := bytes.TrimPrefix(line, []byte("data: "))
		if bytes.Equal(payload, []byte("[DONE]")) {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(payload, &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			acc.tokensIn = chunk.Usage.PromptTokens
			acc.tokensOut = chunk.Usage.CompletionTokens
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		if choice.Delta.Content != "" {
			acc.content.WriteString(choice.Delta.Content)
			if onToken != nil {
				if err := onToken(choice.Delta.Content, false); err != nil {
					return fmt.Errorf("openai_compat: onToken: %w", err)
				}
			}
		}
		for _, tc := range choice.Delta.ToolCalls {
			acc.mergeToolCall(tc.Index, tc.ID, tc.Function.Name, tc.Function.Arguments)
		}
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			acc.finishReason = *choice.FinishReason
		}
	}

	if result != nil {
		result.Content = acc.content.String()
		result.ToolCalls = acc.toolCalls()
		result.FinishReason = acc.finishReason
		if result.FinishReason == "" {
			result.FinishReason = "stop"
		}
		result.TokensIn = acc.tokensIn
		result.TokensOut = acc.tokensOut
	}
	if onToken != nil {
		if err := onToken("", true); err != nil {
			return err
		}
	}
	return nil
}

type streamAccumulator struct {
	content      strings.Builder
	finishReason string
	tokensIn     int
	tokensOut    int
	toolParts    map[int]*ToolCall
}

func (a *streamAccumulator) mergeToolCall(index int, id, name, args string) {
	if a.toolParts == nil {
		a.toolParts = map[int]*ToolCall{}
	}
	tc, ok := a.toolParts[index]
	if !ok {
		tc = &ToolCall{}
		a.toolParts[index] = tc
	}
	if id != "" {
		tc.ID = id
	}
	if name != "" {
		tc.Name = name
	}
	if args != "" {
		tc.Arguments += args
	}
}

func (a *streamAccumulator) toolCalls() []ToolCall {
	if len(a.toolParts) == 0 {
		return nil
	}
	out := make([]ToolCall, 0, len(a.toolParts))
	for i := 0; i < len(a.toolParts)+8; i++ {
		tc, ok := a.toolParts[i]
		if !ok {
			continue
		}
		if tc.Name == "" {
			continue
		}
		out = append(out, *tc)
	}
	return out
}

func toOpenAIMessages(msgs []Message) []map[string]any {
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case RoleTool:
			out = append(out, map[string]any{
				"role":         "tool",
				"tool_call_id": coalesceStr(m.Name, m.ToolID),
				"content":      m.Content,
			})
		case RoleAssistant:
			msg := map[string]any{
				"role":    "assistant",
				"content": m.Content,
			}
			if len(m.ToolCalls) > 0 {
				tcs := make([]map[string]any, 0, len(m.ToolCalls))
				for _, tc := range m.ToolCalls {
					tcs = append(tcs, map[string]any{
						"id":   tc.ID,
						"type": "function",
						"function": map[string]any{
							"name":      tc.Name,
							"arguments": tc.Arguments,
						},
					})
				}
				msg["tool_calls"] = tcs
			}
			out = append(out, msg)
		default:
			out = append(out, map[string]any{
				"role":    string(m.Role),
				"content": m.Content,
			})
		}
	}
	return out
}

func coalesceStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func toOpenAITools(tools []ToolDef) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			},
		})
	}
	return out
}

func truncateBody(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
