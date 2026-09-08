package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/asr"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/config"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/memory"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/persona"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/skills"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/store"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/tts"
)

type toolLLM struct {
	round int
}

func (m *toolLLM) Name() string { return "tool-llm" }

func (m *toolLLM) Chat(_ context.Context, _ []llm.Message, _ []llm.ToolDef, _ llm.OnToken, result *llm.StreamResult) error {
	m.round++
	if result == nil {
		return nil
	}
	if m.round == 1 {
		result.FinishReason = "tool_calls"
		result.ToolCalls = []llm.ToolCall{{
			ID: "c1", Name: "weather", Arguments: `{"city":"杭州"}`,
		}}
		return nil
	}
	result.FinishReason = "stop"
	result.Content = "杭州今天多云，二十三度。"
	return nil
}

type memorySaveLLM struct {
	round int
}

func (m *memorySaveLLM) Name() string { return "memory-save-llm" }

func (m *memorySaveLLM) Chat(_ context.Context, _ []llm.Message, _ []llm.ToolDef, _ llm.OnToken, result *llm.StreamResult) error {
	m.round++
	if result == nil {
		return nil
	}
	if m.round == 1 {
		result.FinishReason = "tool_calls"
		result.ToolCalls = []llm.ToolCall{{
			ID: "c1", Name: "memory.save", Arguments: `{"content":"小主人喜欢恐龙"}`,
		}}
		return nil
	}
	result.FinishReason = "stop"
	result.Content = "好的，我记住了。"
	return nil
}

type toolSender struct {
	toolName string
	toolArgs string
	text     string
	done     bool
	statuses []statusEvent
}

type statusEvent struct {
	step     string
	phase    string
	text     string
	progress float64
}

func (s *toolSender) SendSTT(string, string)      {}
func (s *toolSender) SendText(t string)           { s.text += t }
func (s *toolSender) SendTool(name, args string)  { s.toolName = name; s.toolArgs = args }
func (s *toolSender) SendStatus(step, phase, text string, progress float64) {
	s.statuses = append(s.statuses, statusEvent{step: step, phase: phase, text: text, progress: progress})
}
func (s *toolSender) SendPCMBytes(uint32, []byte) {}
func (s *toolSender) SendDone()                   { s.done = true }

func TestTurn_ToolCalling(t *testing.T) {
	ctx := context.Background()
	workspaceRoot, err := filepath.Abs("../../workspace")
	require.NoError(t, err)
	ps, err := persona.NewReloader(workspaceRoot)
	require.NoError(t, err)
	mem := memory.NewMarkdownStore(t.TempDir())
	sk := mustSkillsReloader(t, workspaceRoot)
	hist := NewHistoryRegistry(5)

	ag := New(ps, mem, sk, hist, nil,
		asr.NewMock("杭州天气怎么样"),
		&toolLLM{},
		tts.NewMock(),
		config.MiniMaxConfig{},
		testAgentCfg(600),
	)

	send := &toolSender{}
	require.NoError(t, ag.Turn(ctx, "dev-tools", []byte("pcm"), send))
	assert.Equal(t, "weather", send.toolName)
	assert.Contains(t, send.toolArgs, "杭州")
	assert.Contains(t, send.text, "杭州")
	assert.True(t, send.done)

	require.Len(t, send.statuses, 2)
	assert.Equal(t, "weather", send.statuses[0].step)
	assert.Equal(t, "start", send.statuses[0].phase)
	assert.Equal(t, "查询天气", send.statuses[0].text)
	assert.Equal(t, 0.0, send.statuses[0].progress)
	assert.Equal(t, "weather", send.statuses[1].step)
	assert.Equal(t, "done", send.statuses[1].phase)
	assert.Equal(t, "查询完成", send.statuses[1].text)
	assert.Equal(t, 1.0, send.statuses[1].progress)
}

func TestTurn_MemorySaveWritesFacts(t *testing.T) {
	ctx := context.Background()
	workspaceRoot, err := filepath.Abs("../../workspace")
	require.NoError(t, err)
	ps, err := persona.NewReloader(workspaceRoot)
	require.NoError(t, err)
	memRoot := t.TempDir()
	mem := memory.NewMarkdownStore(memRoot)
	sk := mustSkillsReloader(t, workspaceRoot)
	hist := NewHistoryRegistry(5)

	ag := New(ps, mem, sk, hist, nil,
		asr.NewMock("请记住我喜欢恐龙"),
		&memorySaveLLM{},
		tts.NewMock(),
		config.MiniMaxConfig{},
		testAgentCfg(600),
	)

	send := &toolSender{}
	require.NoError(t, ag.Turn(ctx, "dev-facts", []byte("pcm"), send))
	assert.Equal(t, "memory.save", send.toolName)
	assert.True(t, send.done)

	factsPath := filepath.Join(memRoot, "memory", "dev-facts", "facts.md")
	body, err := os.ReadFile(factsPath)
	require.NoError(t, err)
	assert.Contains(t, string(body), "小主人喜欢恐龙")

	// 不应写入当日情节日志的 [system] 行
	today := time.Now().Format("2006-01-02")
	dayPath := filepath.Join(memRoot, "memory", "dev-facts", today+".md")
	if raw, err := os.ReadFile(dayPath); err == nil {
		assert.NotContains(t, string(raw), "[system]")
	}
}

type personaSaveLLM struct {
	round int
}

func (m *personaSaveLLM) Name() string { return "persona-save-llm" }

func (m *personaSaveLLM) Chat(_ context.Context, _ []llm.Message, _ []llm.ToolDef, _ llm.OnToken, result *llm.StreamResult) error {
	m.round++
	if result == nil {
		return nil
	}
	if m.round == 1 {
		result.FinishReason = "tool_calls"
		result.ToolCalls = []llm.ToolCall{{
			ID: "c1", Name: "persona.save_soul", Arguments: `{"content":"# SOUL\n- 名字: 小星星"}`,
		}}
		return nil
	}
	result.FinishReason = "stop"
	result.Content = "好，以后叫我小星星。"
	return nil
}

func TestTurn_PersonaSaveSoul(t *testing.T) {
	ctx := context.Background()
	workspaceRoot, err := filepath.Abs("../../workspace")
	require.NoError(t, err)
	ps, err := persona.NewReloader(workspaceRoot)
	require.NoError(t, err)
	mem := memory.NewMarkdownStore(t.TempDir())
	sk := mustSkillsReloader(t, workspaceRoot)
	hist := NewHistoryRegistry(5)

	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "persona.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	require.NoError(t, st.CreateUser(ctx, &store.User{ID: "u1", DisplayName: "小明"}))
	require.NoError(t, st.CreateDevice(ctx, &store.Device{
		ID: "dev-persona", UserID: "u1", PairingCode: "TEST-0001", Status: "unbound",
	}))
	require.NoError(t, st.BindDevice(ctx, "dev-persona", "hash"))

	ag := New(ps, mem, sk, hist, st,
		asr.NewMock("以后叫你小星星"),
		&personaSaveLLM{},
		tts.NewMock(),
		config.MiniMaxConfig{},
		testAgentCfg(600),
	)

	send := &toolSender{}
	require.NoError(t, ag.Turn(ctx, "dev-persona", []byte("pcm"), send))
	assert.Equal(t, "persona.save_soul", send.toolName)

	u, err := st.GetUser(ctx, "u1")
	require.NoError(t, err)
	assert.Contains(t, u.SoulMD, "小星星")

	assert.Contains(t, ag.resolveSoulMD(ctx, "dev-persona", "default-soul"), "小星星")
}

type personaSaveUserLLM struct {
	round int
}

func (m *personaSaveUserLLM) Name() string { return "persona-save-user-llm" }

func (m *personaSaveUserLLM) Chat(_ context.Context, _ []llm.Message, _ []llm.ToolDef, _ llm.OnToken, result *llm.StreamResult) error {
	m.round++
	if result == nil {
		return nil
	}
	if m.round == 1 {
		result.FinishReason = "tool_calls"
		result.ToolCalls = []llm.ToolCall{{
			ID: "c1", Name: "persona.save_user", Arguments: `{"content":"# USER\n- 称呼: 小凯"}`,
		}}
		return nil
	}
	result.FinishReason = "stop"
	result.Content = "好，以后叫你小凯。"
	return nil
}

func TestTurn_PersonaSaveUser(t *testing.T) {
	ctx := context.Background()
	workspaceRoot, err := filepath.Abs("../../workspace")
	require.NoError(t, err)
	ps, err := persona.NewReloader(workspaceRoot)
	require.NoError(t, err)
	mem := memory.NewMarkdownStore(t.TempDir())
	sk := mustSkillsReloader(t, workspaceRoot)
	hist := NewHistoryRegistry(5)

	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "user.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	require.NoError(t, st.CreateUser(ctx, &store.User{ID: "u1", DisplayName: "默认"}))
	require.NoError(t, st.CreateDevice(ctx, &store.Device{
		ID: "dev-user", UserID: "u1", PairingCode: "TEST-0002", Status: "unbound",
	}))
	require.NoError(t, st.BindDevice(ctx, "dev-user", "hash"))

	ag := New(ps, mem, sk, hist, st,
		asr.NewMock("我叫小凯，请更新我的画像"),
		&personaSaveUserLLM{},
		tts.NewMock(),
		config.MiniMaxConfig{},
		testAgentCfg(600),
	)

	send := &toolSender{}
	require.NoError(t, ag.Turn(ctx, "dev-user", []byte("pcm"), send))
	assert.Equal(t, "persona.save_user", send.toolName)

	u, err := st.GetUser(ctx, "u1")
	require.NoError(t, err)
	assert.Contains(t, u.UserMD, "小凯")
	assert.Contains(t, ag.resolveUserMD(ctx, "dev-user", "default-user"), "小凯")
}

type soulSkipThenSaveLLM struct {
	round int
}

func (m *soulSkipThenSaveLLM) Name() string { return "soul-skip-llm" }

func (m *soulSkipThenSaveLLM) Chat(_ context.Context, _ []llm.Message, _ []llm.ToolDef, _ llm.OnToken, result *llm.StreamResult) error {
	m.round++
	if result == nil {
		return nil
	}
	switch m.round {
	case 1:
		result.FinishReason = "stop"
		result.Content = "好呀，以后叫小星星。"
	case 2:
		result.FinishReason = "tool_calls"
		result.ToolCalls = []llm.ToolCall{{
			ID: "c1", Name: "persona.save_soul", Arguments: `{"content":"# SOUL\n- 名字: 小星星"}`,
		}}
	default:
		result.FinishReason = "stop"
		result.Content = "已经改好了，我叫小星星。"
	}
	return nil
}

func TestWantsSoulSave(t *testing.T) {
	assert.True(t, wantsSoulSave("以后叫你小星星"))
	assert.True(t, wantsSoulSave("把你的名字改成豆豆"))
	assert.False(t, wantsSoulSave("今天天气怎么样"))
}

func TestTurn_SoulSkipThenRetry(t *testing.T) {
	ctx := context.Background()
	workspaceRoot, err := filepath.Abs("../../workspace")
	require.NoError(t, err)
	ps, err := persona.NewReloader(workspaceRoot)
	require.NoError(t, err)
	mem := memory.NewMarkdownStore(t.TempDir())
	sk := mustSkillsReloader(t, workspaceRoot)
	hist := NewHistoryRegistry(5)

	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "soul-retry.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })
	require.NoError(t, st.CreateUser(ctx, &store.User{ID: "u1", DisplayName: "小明"}))
	require.NoError(t, st.CreateDevice(ctx, &store.Device{
		ID: "dev-retry", UserID: "u1", PairingCode: "TEST-0003", Status: "unbound",
	}))
	require.NoError(t, st.BindDevice(ctx, "dev-retry", "hash"))

	ag := New(ps, mem, sk, hist, st,
		asr.NewMock("以后叫你小星星"),
		&soulSkipThenSaveLLM{},
		tts.NewMock(),
		config.MiniMaxConfig{},
		testAgentCfg(600),
	)

	send := &toolSender{}
	require.NoError(t, ag.Turn(ctx, "dev-retry", []byte("pcm"), send))
	assert.Equal(t, "persona.save_soul", send.toolName)
	assert.Contains(t, send.text, "已经改好了")
	assert.NotContains(t, send.text, "好呀，以后叫小星星")
	u, err := st.GetUser(ctx, "u1")
	require.NoError(t, err)
	assert.Contains(t, u.SoulMD, "小星星")
}

type reminderSetLLM struct {
	round int
}

func (m *reminderSetLLM) Name() string { return "reminder-set-llm" }

func (m *reminderSetLLM) Chat(_ context.Context, _ []llm.Message, _ []llm.ToolDef, _ llm.OnToken, result *llm.StreamResult) error {
	m.round++
	if result == nil {
		return nil
	}
	if m.round == 1 {
		result.FinishReason = "tool_calls"
		result.ToolCalls = []llm.ToolCall{{
			ID: "c1", Name: "reminder.set", Arguments: `{"when":"三点","what":"喝水"}`,
		}}
		return nil
	}
	result.FinishReason = "stop"
	result.Content = "记下了，但我不会到点响喇叭。"
	return nil
}

func TestTurn_ReminderSetWritesFile(t *testing.T) {
	ctx := context.Background()
	workspaceRoot, err := filepath.Abs("../../workspace")
	require.NoError(t, err)
	ps, err := persona.NewReloader(workspaceRoot)
	require.NoError(t, err)
	memRoot := t.TempDir()
	mem := memory.NewMarkdownStore(memRoot)
	sk := mustSkillsReloader(t, workspaceRoot)
	hist := NewHistoryRegistry(5)

	ag := New(ps, mem, sk, hist, nil,
		asr.NewMock("三点提醒我喝水"),
		&reminderSetLLM{},
		tts.NewMock(),
		config.MiniMaxConfig{},
		testAgentCfg(600),
	)
	ag.WorkspaceRoot = memRoot

	send := &toolSender{}
	require.NoError(t, ag.Turn(ctx, "dev-rem", []byte("pcm"), send))
	assert.Equal(t, "reminder.set", send.toolName)

	body, err := os.ReadFile(filepath.Join(memRoot, "memory", "dev-rem", "reminders.md"))
	require.NoError(t, err)
	assert.Contains(t, string(body), "喝水")
	assert.Contains(t, string(body), "三点")
	assert.Contains(t, send.text, "不会到点响喇叭")
	require.GreaterOrEqual(t, len(send.statuses), 2)
	assert.Equal(t, "reminder.set", send.statuses[0].step)
	assert.Equal(t, "记下提醒", send.statuses[0].text)
}

func TestReminderSet_ToolResultSaysNoBuzzer(t *testing.T) {
	ag := &Agent{WorkspaceRoot: t.TempDir()}
	reg := skills.NewRegistry()
	ag.registerReminderBuiltins(reg, "dev-rem", time.Now())
	out, err := reg.Execute(context.Background(), "reminder.set", `{"when":"三点","what":"喝水"}`)
	require.NoError(t, err)
	assert.Contains(t, out, "不会到点响喇叭")
	assert.Contains(t, out, "喝水")
}
