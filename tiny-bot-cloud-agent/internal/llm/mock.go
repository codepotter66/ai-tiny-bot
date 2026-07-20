package llm

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Mock 流式回显用户输入 + 套上人设前缀。v1 默认实现。
//
// 行为：把最后一条 user 消息原样回显，前面加 "你说的是："，结尾加 "。"
// 流式：每次 1-2 个字 + 20-30ms 间隔，模拟真实 LLM 的打字节奏。
type Mock struct {
	// Prefix 每次回复前缀（模拟人设语气）
	Prefix string
	// Suffix 每次回复后缀
	Suffix string
	// PerTokenDelay 模拟每个 token 的延迟
	PerTokenDelay time.Duration
}

func NewMock() *Mock {
	return &Mock{
		Prefix:        "嗯，",
		Suffix:        "。",
		PerTokenDelay: 20 * time.Millisecond,
	}
}

func (m *Mock) Name() string { return "mock" }

func (m *Mock) Chat(ctx context.Context, msgs []Message, tools []ToolDef, onToken OnToken, result *StreamResult) error {
	// 取最后一条 user
	var userText string
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == RoleUser {
			userText = msgs[i].Content
			break
		}
	}
	resp := m.Prefix + userText + m.Suffix

	runes := []rune(resp)
	for _, r := range runes {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.PerTokenDelay):
		}
		if onToken != nil {
			if err := onToken(string(r), false); err != nil {
				return err
			}
		}
	}
	if result != nil {
		result.Content = resp
		result.FinishReason = "stop"
	}
	if onToken != nil {
		return onToken("", true)
	}
	return nil
}

// EchoForTest 简单一次性回显（单测用，不流式）。
func EchoForTest(msgs []Message) string {
	var b strings.Builder
	for _, m := range msgs {
		fmt.Fprintf(&b, "[%s] %s\n", m.Role, m.Content)
	}
	return b.String()
}
