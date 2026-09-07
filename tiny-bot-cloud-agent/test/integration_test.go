// Package integration 端到端测试：用 httptest 起一个真实服务，模拟设备跑一轮。
package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/agent"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/asr"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/auth"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/config"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/memory"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/persona"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/skills"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/store"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/tts"
)

// fakeTurn 模拟一次 turn：ASR mock 出固定文本 → LLM mock 出固定回复 → TTS mock 出 PCM。
func TestEndToEnd_Turn(t *testing.T) {
	ctx := context.Background()

	// 准备 SQLite
	dir := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(dir, "e2e.db"))
	require.NoError(t, err)
	defer st.Close()

	// 准备 workspace（直接用 repo 内的 workspace 目录）
	workspaceRoot, err := filepath.Abs("../workspace")
	require.NoError(t, err)

	// persona
	ps, err := persona.NewReloader(workspaceRoot)
	require.NoError(t, err)

	// memory
	mem := memory.NewMarkdownStore(workspaceRoot)

	// skills（可能为空，集成测试不强依赖）
	sk, err := skills.NewReloader(workspaceRoot)
	require.NoError(t, err)

	// history
	hist := agent.NewHistoryRegistry(10)

	// provider：固定 ASR 输入（让 LLM 有可消费的内容）
	asrImpl := asr.NewMock("今天天气怎么样")
	llmImpl := &llm.Mock{Prefix: "今天", Suffix: "很好。", PerTokenDelay: 0}
	ttsImpl := tts.NewMock()

	ag := agent.New(ps, mem, sk, hist, st, asrImpl, llmImpl, ttsImpl, config.MiniMaxConfig{}, config.AgentConfig{
		MaxResponseChars: 200, HistoryTurns: 10, MemoryRecallK: 5, MemoryLookbackDays: 7, FactsMax: 80,
		TurnTimeout: 120 * time.Second, MaxToolRounds: 8,
		CodeRunTimeout: 15 * time.Second, CodePythonBin: "python3",
	})
	ag.WorkspaceRoot = t.TempDir()

	// 1) 加一个 user + device
	require.NoError(t, st.CreateUser(ctx, &store.User{ID: "u1", DisplayName: "test"}))
	require.NoError(t, st.CreateDevice(ctx, &store.Device{
		ID: "d1", UserID: "u1", PairingCode: "CODE", Status: "unbound",
	}))

	// 2) provision 拿 token
	authSvc := auth.New(st)
	tok, err := authSvc.Provision(ctx, "d1", "CODE", false)
	require.NoError(t, err)

	// 3) verify token
	ok, err := authSvc.VerifyToken(ctx, "d1", tok)
	require.NoError(t, err)
	assert.True(t, ok)

	// 4) 跑 turn
	rec := &recordingSender{}
	pcm := make([]byte, 10240) // 320ms @ 16k16 mono
	require.NoError(t, ag.Turn(ctx, "d1", pcm, rec))

	// 5) 等 TTS 异步收尾（最长 2s）
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && rec.pcmBytes == 0 {
		time.Sleep(20 * time.Millisecond)
	}

	// 6) 断言
	assert.Equal(t, "今天天气怎么样", rec.stt)
	assert.Equal(t, "zh", rec.sttLang)
	assert.NotEmpty(t, rec.text)        // LLM 至少输出过
	assert.NotEmpty(t, rec.pcmBytes)    // TTS 至少发了一帧
	assert.True(t, rec.done)            // 收尾

	// 6) 记忆里有内容
	hist2 := hist.Get("d1")
	assert.GreaterOrEqual(t, len(hist2.Snapshot()), 2) // user + assistant

	// 7) 记忆文件被写入
	today := mem.TodayPath("d1")
	body, err := readFile(today)
	if err == nil {
		assert.Contains(t, string(body), "今天天气怎么样")
	}
}

type recordingSender struct {
	stt       string
	sttLang   string
	text      string
	pcmBytes  int
	tool      string
	done      bool
}

func (r *recordingSender) SendSTT(s, lang string)              { r.stt = s; r.sttLang = lang }
func (r *recordingSender) SendText(s string)                      { r.text += s }
func (r *recordingSender) SendPCMBytes(seq uint32, b []byte)      { r.pcmBytes += len(b) }
func (r *recordingSender) SendTool(name, args string)             { r.tool = name }
func (r *recordingSender) SendStatus(string, string, string, float64) {}
func (r *recordingSender) SendDone()                              { r.done = true }

func readFile(p string) ([]byte, error) {
	return readFileOS(p)
}
