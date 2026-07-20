// Package llm 提供 LLM 抽象与 system prompt 组合。
package llm

import (
	"context"
	"errors"
)

// Role 消息角色。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall LLM 请求调用的工具。
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Message 一条对话消息。
type Message struct {
	Role      Role       `json:"role"`
	Content   string     `json:"content"`
	Name      string     `json:"name,omitempty"`       // tool 消息时 = tool_call id
	ToolID    string     `json:"tool_id,omitempty"`    // tool 消息时回填
	ToolName  string     `json:"tool_name,omitempty"`  // assistant 消息：tool_use
	ToolArgs  string     `json:"tool_args,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"` // assistant 消息：tool_calls
}

// StreamResult 流式 Chat 结束后的汇总（result 可为 nil，调用方不关心时跳过）。
type StreamResult struct {
	Content      string
	ToolCalls    []ToolCall
	FinishReason string // stop | tool_calls | length
	TokensIn     int
	TokensOut    int
}

// ToolDef 工具描述（OpenAI 兼容协议 tools[i]）。
type ToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// OnToken token 流式回调。endTurn=true 时表示流结束（stop_reason=end_turn）。
type OnToken func(token string, endTurn bool) error

// Chat LLM 抽象。
type Chat interface {
	// Chat 流式返回 token。每次 token 通过 onToken 回调，回调中 endTurn=true 时
	// 表示模型输出完成（end_turn / tool_use / max_tokens）。
	// result 非 nil 时写入完整汇总（含 tool_calls、usage）。
	Chat(ctx context.Context, msgs []Message, tools []ToolDef, onToken OnToken, result *StreamResult) error
	// Name provider 名称。
	Name() string
}

// ErrNotImplemented 标识当前 provider 尚未实现。
var ErrNotImplemented = errors.New("llm: provider not implemented")
