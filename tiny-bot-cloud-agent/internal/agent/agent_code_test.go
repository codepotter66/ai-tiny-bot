package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/asr"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/config"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/memory"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/persona"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/tts"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/ws"
)

type codeWriteRunLLM struct {
	round int
}

func (m *codeWriteRunLLM) Name() string { return "code-write-run-llm" }

func (m *codeWriteRunLLM) Chat(_ context.Context, _ []llm.Message, _ []llm.ToolDef, _ llm.OnToken, result *llm.StreamResult) error {
	m.round++
	if result == nil {
		return nil
	}
	switch m.round {
	case 1:
		result.FinishReason = "tool_calls"
		result.ToolCalls = []llm.ToolCall{{
			ID: "c1", Name: "code.write",
			Arguments: `{"filename":"sum.py","content":"print(1+2)\n"}`,
		}}
	case 2:
		result.FinishReason = "tool_calls"
		result.ToolCalls = []llm.ToolCall{{
			ID: "c2", Name: "code.run",
			Arguments: `{"filename":"sum.py"}`,
		}}
	default:
		result.FinishReason = "stop"
		result.Content = "一加二等于三。"
	}
	return nil
}

func TestTurn_CodeWriteAndRun(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	ctx := context.Background()
	workspaceRoot, err := filepath.Abs("../../workspace")
	require.NoError(t, err)
	ps, err := persona.NewReloader(workspaceRoot)
	require.NoError(t, err)
	scratchRoot := t.TempDir()
	mem := memory.NewMarkdownStore(scratchRoot)
	sk := mustSkillsReloader(t, workspaceRoot)
	hist := NewHistoryRegistry(5)

	ag := New(ps, mem, sk, hist, nil,
		asr.NewMock("算一下一加二"),
		&codeWriteRunLLM{},
		tts.NewMock(),
		config.MiniMaxConfig{},
		testAgentCfg(600),
	)
	ag.WorkspaceRoot = scratchRoot

	send := &toolSender{}
	require.NoError(t, ag.Turn(ctx, "dev-code", []byte("pcm"), send))
	assert.Equal(t, "code.run", send.toolName)
	assert.Contains(t, send.text, "三")
	assert.True(t, send.done)

	body, err := os.ReadFile(filepath.Join(scratchRoot, "scratch", "dev-code", "sum.py"))
	require.NoError(t, err)
	assert.Contains(t, string(body), "print(1+2)")

	var phases []string
	for _, st := range send.statuses {
		phases = append(phases, st.step+":"+st.phase)
	}
	assert.Contains(t, phases, "code.write:"+ws.StatusStart)
	assert.Contains(t, phases, "code.write:"+ws.StatusDone)
	assert.Contains(t, phases, "code.run:"+ws.StatusStart)
	assert.Contains(t, phases, "code.run:"+ws.StatusDone)
}
