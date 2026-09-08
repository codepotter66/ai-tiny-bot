package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/codejail"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/logging"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/skills"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/store"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/tts"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/ws"
)

const defaultMaxToolRounds = 8

type toolExecutor struct {
	agent    *Agent
	deviceID string
}

func (a *Agent) maxToolRounds() int {
	if a.AgentCfg.MaxToolRounds > 0 {
		return a.AgentCfg.MaxToolRounds
	}
	return defaultMaxToolRounds
}

func (a *Agent) runToolLoop(
	ctx context.Context,
	msgs []llm.Message,
	tools []llm.ToolDef,
	send Sender,
	deviceID string,
	now time.Time,
	onVisible func(string) error,
	userText string,
) (string, int, int, error) {
	exec := &toolExecutor{agent: a, deviceID: deviceID}
	var totalIn, totalOut int
	maxRounds := a.maxToolRounds()
	usedSoul := false
	soulNudged := false

	for round := 0; round < maxRounds; round++ {
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
				if tc.Name == "persona.save_soul" {
					usedSoul = true
				}
				send.SendStatus(tc.Name, ws.StatusStart, statusStartText(tc.Name), 0)
				send.SendTool(tc.Name, tc.Arguments)
				out, execErr := exec.runOne(ctx, tc)
				if execErr != nil {
					send.SendStatus(tc.Name, ws.StatusError, statusErrorText(tc.Name), -1)
				} else {
					send.SendStatus(tc.Name, ws.StatusDone, statusDoneText(tc.Name), 1)
				}
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

		if wantsSoulSave(userText) && !usedSoul && !soulNudged {
			soulNudged = true
			if result.Content != "" {
				msgs = append(msgs, llm.Message{Role: llm.RoleAssistant, Content: result.Content})
			}
			msgs = append(msgs, llm.Message{
				Role:    llm.RoleSystem,
				Content: "用户在改你的名字或性格。必须先调用 persona.save_soul 写入短版完整 SOUL Markdown，禁止只口头答应。",
			})
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
	return "", totalIn, totalOut, fmt.Errorf("tool loop exceeded %d rounds", maxRounds)
}

func (e *toolExecutor) runOne(ctx context.Context, tc llm.ToolCall) (string, error) {
	logger := logging.FromContext(ctx)
	reg := e.agent.skills()
	out, err := reg.Execute(ctx, tc.Name, tc.Arguments)
	if err != nil {
		logger.Warn("tool failed", "tool", tc.Name, "err", err)
		return fmt.Sprintf("工具 %s 执行失败: %s", tc.Name, err.Error()), err
	}
	return strings.TrimSpace(out), nil
}

func statusStartText(toolName string) string {
	switch toolName {
	case "memory.save":
		return "记住中"
	case "memory.recall":
		return "回忆中"
	case "persona.save_soul":
		return "更新人设"
	case "persona.save_user":
		return "更新画像"
	case "reminder.set":
		return "记下提醒"
	case "reminder.list":
		return "查看提醒"
	case "weather":
		return "查询天气"
	case "code.write":
		return "写代码"
	case "code.run":
		return "运行中"
	default:
		return truncateOLED(toolName)
	}
}

func statusDoneText(toolName string) string {
	switch toolName {
	case "memory.save":
		return "已记住"
	case "memory.recall":
		return "回忆完成"
	case "persona.save_soul", "persona.save_user":
		return "已更新"
	case "reminder.set":
		return "已记下"
	case "reminder.list":
		return "查完了"
	case "weather":
		return "查询完成"
	case "code.write":
		return "已写好"
	case "code.run":
		return "运行完成"
	default:
		return "完成"
	}
}

func statusErrorText(toolName string) string {
	switch toolName {
	case "code.run":
		return "运行失败"
	default:
		base := statusStartText(toolName)
		return truncateOLED(base + "失败")
	}
}

func truncateOLED(s string) string {
	const maxRunes = 21
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes])
}

func wantsSoulSave(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	needles := []string{
		"以后叫你", "以后叫我", "改成你叫", "改名字", "换个名字", "把你的名字",
		"改性格", "性格改", "换人设", "更新你的灵魂", "更新人设",
	}
	for _, n := range needles {
		if strings.Contains(t, n) {
			return true
		}
	}
	return false
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

func (a *Agent) registerReminderBuiltins(reg *skills.Registry, deviceID string, now time.Time) {
	_ = reg.AddBuiltin(&skills.Builtin{
		Name:        "reminder.set",
		Description: "记下用户要提醒的事。本设备不会到点响喇叭或闹钟；必须如实告诉用户只会记在本子上。",
		Parameters: map[string]skills.Param{
			"when": {Type: "string", Description: "时间描述，例如三点、明天早上", Required: true},
			"what": {Type: "string", Description: "要提醒的内容", Required: true},
		},
		Handler: func(_ context.Context, args map[string]interface{}) (string, error) {
			when, _ := args["when"].(string)
			what, _ := args["what"].(string)
			when = strings.TrimSpace(when)
			what = strings.TrimSpace(what)
			if when == "" || what == "" {
				return "", fmt.Errorf("when and what required")
			}
			if err := appendReminder(a.WorkspaceRoot, deviceID, now, when, what); err != nil {
				return "", err
			}
			return fmt.Sprintf("已记下：%s %s。我不会到点响喇叭，到时候还得你再叫我。", when, what), nil
		},
	})
	_ = reg.AddBuiltin(&skills.Builtin{
		Name:        "reminder.list",
		Description: "列出该设备已记下的提醒（不会自动响铃）。",
		Parameters:  map[string]skills.Param{},
		Handler: func(_ context.Context, _ map[string]interface{}) (string, error) {
			body, err := readReminders(a.WorkspaceRoot, deviceID)
			if err != nil {
				return "", err
			}
			if strings.TrimSpace(body) == "" {
				return "还没有记下的提醒。", nil
			}
			return strings.TrimSpace(body), nil
		},
	})
}

func reminderPath(root, deviceID string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("workspace root not configured")
	}
	if strings.TrimSpace(deviceID) == "" {
		return "", fmt.Errorf("deviceID empty")
	}
	return filepath.Join(root, "memory", deviceID, "reminders.md"), nil
}

func appendReminder(root, deviceID string, now time.Time, when, what string) error {
	path, err := reminderPath(root, deviceID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	exists := false
	if _, statErr := os.Stat(path); statErr == nil {
		exists = true
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if !exists {
		if _, err := fmt.Fprintf(f, "# Reminders\n"); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(f, "- [%s] %s：%s\n", now.Format("2006-01-02"), when, what)
	return err
}

func readReminders(root, deviceID string) (string, error) {
	path, err := reminderPath(root, deviceID)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

func (a *Agent) codeJail() *codejail.Jail {
	bin := a.AgentCfg.CodePythonBin
	if strings.TrimSpace(bin) == "" {
		bin = codejail.DefaultPython
	}
	timeout := a.AgentCfg.CodeRunTimeout
	if timeout <= 0 {
		timeout = codejail.DefaultTimeout
	}
	return &codejail.Jail{
		WorkspaceRoot: a.WorkspaceRoot,
		PythonBin:     bin,
		Timeout:       timeout,
		MaxFileBytes:  codejail.MaxFileBytes,
	}
}

func (a *Agent) registerCodeBuiltins(reg *skills.Registry, deviceID string) {
	jail := a.codeJail()
	_ = reg.AddBuiltin(&skills.Builtin{
		Name:        "code.write",
		Description: "把 Python 源码写入本设备受限 scratch 目录（仅 stdlib，文件名须为 *.py，无路径）。需要运行时再调 code.run。",
		Parameters: map[string]skills.Param{
			"filename": {Type: "string", Description: "文件名，例如 calc.py（不可含 / 或 ..）", Required: true},
			"content":  {Type: "string", Description: "完整 Python 源码", Required: true},
		},
		Handler: func(_ context.Context, args map[string]interface{}) (string, error) {
			filename, _ := args["filename"].(string)
			content, _ := args["content"].(string)
			filename = strings.TrimSpace(filename)
			if filename == "" {
				return "", fmt.Errorf("filename required")
			}
			if a.WorkspaceRoot == "" {
				return "", fmt.Errorf("workspace root not configured")
			}
			name, err := jail.Write(deviceID, filename, content)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("已写入 %s（%d 字节）。用 code.run 执行。", name, len(content)), nil
		},
	})
	_ = reg.AddBuiltin(&skills.Builtin{
		Name:        "code.run",
		Description: "在受限目录执行先前 code.write 写入的 Python 文件（仅 stdlib，超时杀掉，不继承服务器密钥）。",
		Parameters: map[string]skills.Param{
			"filename": {Type: "string", Description: "要执行的 *.py 文件名", Required: true},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (string, error) {
			filename, _ := args["filename"].(string)
			filename = strings.TrimSpace(filename)
			if filename == "" {
				return "", fmt.Errorf("filename required")
			}
			if a.WorkspaceRoot == "" {
				return "", fmt.Errorf("workspace root not configured")
			}
			res, err := jail.Run(ctx, deviceID, filename)
			if err != nil {
				return "", err
			}
			var b strings.Builder
			if res.TimedOut {
				fmt.Fprintf(&b, "超时（已杀掉）。\n")
			} else {
				fmt.Fprintf(&b, "exit_code=%d\n", res.ExitCode)
			}
			if res.Stdout != "" {
				fmt.Fprintf(&b, "stdout:\n%s\n", res.Stdout)
			}
			if res.Stderr != "" {
				fmt.Fprintf(&b, "stderr:\n%s\n", res.Stderr)
			}
			out := strings.TrimSpace(b.String())
			if out == "" {
				out = "exit_code=0（无输出）"
			}
			// 非零退出仍把结果交给 LLM（不返回 error），便于它解释报错
			return out, nil
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
