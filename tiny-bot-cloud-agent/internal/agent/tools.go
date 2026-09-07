package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/logging"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/skills"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/store"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/tts"
)

const maxToolRounds = 3

type toolExecutor struct {
	agent    *Agent
	deviceID string
}

func (a *Agent) runToolLoop(
	ctx context.Context,
	msgs []llm.Message,
	tools []llm.ToolDef,
	send Sender,
	deviceID string,
	now time.Time,
	onVisible func(string) error,
) (string, int, int, error) {
	exec := &toolExecutor{agent: a, deviceID: deviceID}
	var totalIn, totalOut int

	for round := 0; round < maxToolRounds; round++ {
		var result llm.StreamResult
		filter := llm.NewStreamThinkingFilter()
		streamed := false
		err := a.LLM.Chat(ctx, msgs, tools, func(tok string, end bool) error {
			if end {
				if streamed {
					if tail := tts.CleanText(filter.Flush()); tail != "" {
						return onVisible(tail)
					}
				}
				return nil
			}
			// tool_calls 轮次 content 通常为空；有 content 时先缓冲到 result，结束时再决定
			visible := tts.CleanText(filter.Feed(tok))
			if visible == "" {
				return nil
			}
			streamed = true
			return onVisible(visible)
		}, &result)
		if err != nil {
			return "", totalIn, totalOut, err
		}
		totalIn += result.TokensIn
		totalOut += result.TokensOut

		if result.FinishReason == "tool_calls" && len(result.ToolCalls) > 0 {
			assistant := llm.Message{
				Role:      llm.RoleAssistant,
				Content:   result.Content,
				ToolCalls: result.ToolCalls,
			}
			msgs = append(msgs, assistant)
			for _, tc := range result.ToolCalls {
				send.SendTool(tc.Name, tc.Arguments)
				out, _ := exec.runOne(ctx, tc)
				msgs = append(msgs, llm.Message{
					Role:    llm.RoleTool,
					Name:    tc.ID,
					ToolID:  tc.ID,
					Content: out,
				})
				a.persistToolMessage(ctx, deviceID, tc, out)
			}
			continue
		}

		// 非流式 provider（只写 result、不回调 onToken）时补一次
		if !streamed && result.Content != "" {
			visible := tts.CleanText(filter.Feed(result.Content) + filter.Flush())
			if visible != "" {
				if err := onVisible(visible); err != nil {
					return "", totalIn, totalOut, err
				}
			}
			return strings.TrimSpace(visible), totalIn, totalOut, nil
		}
		return strings.TrimSpace(result.Content), totalIn, totalOut, nil
	}
	return "", totalIn, totalOut, fmt.Errorf("tool loop exceeded %d rounds", maxToolRounds)
}

func (e *toolExecutor) runOne(ctx context.Context, tc llm.ToolCall) (string, error) {
	logger := logging.FromContext(ctx)
	reg := e.agent.skills()
	out, err := reg.Execute(ctx, tc.Name, tc.Arguments)
	if err != nil {
		logger.Warn("tool failed", "tool", tc.Name, "err", err)
		return fmt.Sprintf("工具 %s 执行失败: %s", tc.Name, err.Error()), nil
	}
	return strings.TrimSpace(out), nil
}

func (a *Agent) registerMemoryBuiltins(reg *skills.Registry, deviceID string, now time.Time) {
	_ = reg.AddBuiltin(&skills.Builtin{
		Name:        "memory.save",
		Description: "把用户明确要求记住的信息写入长期事实记忆。",
		Parameters: map[string]skills.Param{
			"content": {Type: "string", Description: "要记住的内容", Required: true},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			content, _ := args["content"].(string)
			content = strings.TrimSpace(content)
			if content == "" {
				return "", fmt.Errorf("content required")
			}
			if err := a.Memory.SaveFact(ctx, deviceID, content, now, a.AgentCfg.FactsMax); err != nil {
				return "", err
			}
			return "已记住。", nil
		},
	})
	_ = reg.AddBuiltin(&skills.Builtin{
		Name:        "memory.recall",
		Description: "按关键词召回该设备的历史情节记忆片段。",
		Parameters: map[string]skills.Param{
			"query": {Type: "string", Description: "召回关键词或问题", Required: true},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			query, _ := args["query"].(string)
			query = strings.TrimSpace(query)
			if query == "" {
				return "", fmt.Errorf("query required")
			}
			snippets, err := a.Memory.Recall(ctx, deviceID, query, a.AgentCfg.MemoryLookbackDays, a.AgentCfg.MemoryRecallK)
			if err != nil {
				return "", err
			}
			if len(snippets) == 0 {
				return "没有找到相关记忆。", nil
			}
			var b strings.Builder
			for _, s := range snippets {
				fmt.Fprintf(&b, "- [%s] %s\n", s.Date, s.Content)
			}
			return strings.TrimSpace(b.String()), nil
		},
	})
}

func (a *Agent) registerPersonaBuiltins(reg *skills.Registry, deviceID string) {
	_ = reg.AddBuiltin(&skills.Builtin{
		Name:        "persona.save_soul",
		Description: "整段覆盖该用户的 SOUL（名字、语气、性格）。用户要求改机器人名字/性格/语气时必须调用；content 用短版完整 Markdown（# SOUL + 几条要点即可），不要只口头答应。",
		Parameters: map[string]skills.Param{
			"content": {Type: "string", Description: "完整 SOUL Markdown 正文（可短）", Required: true},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			content, _ := args["content"].(string)
			content = strings.TrimSpace(content)
			if content == "" {
				return "", fmt.Errorf("content required")
			}
			userID, err := a.userIDForDevice(ctx, deviceID)
			if err != nil {
				return "", err
			}
			if err := a.Store.UpdateSoulMD(ctx, userID, content); err != nil {
				return "", err
			}
			return "人设已更新。", nil
		},
	})
	_ = reg.AddBuiltin(&skills.Builtin{
		Name:        "persona.save_user",
		Description: "整段覆盖该用户的 USER 画像（称呼、年龄、偏好）。用户明确介绍自己或要求改称呼时调用；传入完整 Markdown。零散偏好仍用 memory.save。",
		Parameters: map[string]skills.Param{
			"content": {Type: "string", Description: "完整 USER Markdown 正文", Required: true},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			content, _ := args["content"].(string)
			content = strings.TrimSpace(content)
			if content == "" {
				return "", fmt.Errorf("content required")
			}
			userID, err := a.userIDForDevice(ctx, deviceID)
			if err != nil {
				return "", err
			}
			if err := a.Store.UpdateUserMD(ctx, userID, content); err != nil {
				return "", err
			}
			return "用户画像已更新。", nil
		},
	})
}

func (a *Agent) persistToolMessage(ctx context.Context, deviceID string, tc llm.ToolCall, out string) {
	if a.Store == nil {
		return
	}
	conv, err := a.Store.OpenConversation(ctx, deviceID)
	if err != nil {
		return
	}
	_ = a.Store.AppendMessageWithTokens(ctx, conv.ID, &store.Message{
		ID: store.NewID(), Role: "tool", Content: out,
		ToolName: tc.Name, ToolArgs: tc.Arguments,
	})
}
