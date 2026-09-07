package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/asr"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/config"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/memory"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/persona"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/tts"
)

type langSender struct {
	stt     string
	sttLang string
	pcm     int
	done    bool
}

func (s *langSender) SendSTT(text, lang string)         { s.stt = text; s.sttLang = lang }
func (s *langSender) SendText(string)                   {}
func (s *langSender) SendTool(string, string)           {}
func (s *langSender) SendStatus(string, string, string, float64) {}
func (s *langSender) SendPCMBytes(uint32, []byte)       { s.pcm++ }
func (s *langSender) SendDone()                         { s.done = true }

func TestTurn_LanguageRouting(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantLang string
	}{
		{"mandarin", "今天天气怎么样", "zh"},
		{"cantonese", "你喺边度呀？", "yue"},
		{"english", "How is the weather?", "en"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			workspaceRoot := "../../workspace"
			ps, err := persona.NewReloader(workspaceRoot)
			require.NoError(t, err)
			mem := memory.NewMarkdownStore(workspaceRoot)
			sk := mustSkillsReloader(t, workspaceRoot)
			hist := NewHistoryRegistry(5)

			ag := New(ps, mem, sk, hist, nil,
				asr.NewMock(tc.input),
				&llm.Mock{Prefix: "ok", Suffix: "。", PerTokenDelay: 0},
				tts.NewMock(),
				config.MiniMaxConfig{},
				testAgentCfg(200),
			)

			send := &langSender{}
			require.NoError(t, ag.Turn(ctx, "dev-lang-test", []byte("pcm"), send))

			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) && !send.done {
				time.Sleep(20 * time.Millisecond)
			}

			assert.Equal(t, tc.input, send.stt)
			assert.Equal(t, tc.wantLang, send.sttLang)
			assert.True(t, send.done)
			assert.Greater(t, send.pcm, 0)
		})
	}
}

func TestTurn_EmptySTT_SkipsLLM(t *testing.T) {
	cases := []struct {
		name string
		asr  asr.Transcriber
	}{
		{"force_empty", &asr.Mock{ForceEmpty: true}},
		{"whitespace", asr.NewMock("   ")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			workspaceRoot := "../../workspace"
			ps, err := persona.NewReloader(workspaceRoot)
			require.NoError(t, err)
			mem := memory.NewMarkdownStore(workspaceRoot)
			sk := mustSkillsReloader(t, workspaceRoot)
			hist := NewHistoryRegistry(5)

			failLLM := &chatCounter{t: t}
			ag := New(ps, mem, sk, hist, nil,
				tc.asr,
				failLLM,
				tts.NewMock(),
				config.MiniMaxConfig{},
				testAgentCfg(200),
			)

			send := &langSender{}
			require.NoError(t, ag.Turn(ctx, "dev-empty", []byte("pcm"), send))
			assert.True(t, send.done)
			assert.Equal(t, 0, send.pcm)
			assert.Equal(t, 0, failLLM.calls)
			assert.Empty(t, hist.Get("dev-empty").Snapshot())
		})
	}
}

type chatCounter struct {
	t     *testing.T
	calls int
}

func (c *chatCounter) Name() string { return "fail-on-chat" }

func (c *chatCounter) Chat(ctx context.Context, msgs []llm.Message, tools []llm.ToolDef, onToken llm.OnToken, _ *llm.StreamResult) error {
	c.calls++
	c.t.Fatal("LLM.Chat should not be called for empty STT")
	return nil
}

func TestTurn_ConcurrentTTSWaits(t *testing.T) {
	ctx := context.Background()
	workspaceRoot := "../../workspace"
	ps, err := persona.NewReloader(workspaceRoot)
	require.NoError(t, err)
	mem := memory.NewMarkdownStore(workspaceRoot)
	sk := mustSkillsReloader(t, workspaceRoot)
	hist := NewHistoryRegistry(5)

	ag := New(ps, mem, sk, hist, nil,
		asr.NewMock("hello"),
		&llm.Mock{Prefix: "Hi", Suffix: " there.", PerTokenDelay: 0},
		tts.NewMock(),
		config.MiniMaxConfig{},
		testAgentCfg(200),
	)

	var wg sync.WaitGroup
	send := &langSender{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = ag.Turn(ctx, "dev", []byte("pcm"), send)
	}()
	wg.Wait()
	assert.True(t, send.done)
}
