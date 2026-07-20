package agent

import (
	"context"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/asr"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/config"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/memory"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/persona"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/tts"
)

// multiSentenceLLM 一次输出多句，触发 Aggregator 多次 flush。
type multiSentenceLLM struct {
	text string
}

func (m *multiSentenceLLM) Name() string { return "multi-sentence" }

func (m *multiSentenceLLM) Chat(_ context.Context, _ []llm.Message, _ []llm.ToolDef, onToken llm.OnToken, result *llm.StreamResult) error {
	for _, r := range []rune(m.text) {
		if onToken != nil {
			if err := onToken(string(r), false); err != nil {
				return err
			}
		}
	}
	if result != nil {
		result.Content = m.text
		result.FinishReason = "stop"
	}
	if onToken != nil {
		return onToken("", true)
	}
	return nil
}

// taggedTTS 每个句子的 PCM chunk 首字节为该句序号，用于检测并发交错。
type taggedTTS struct {
	mu     sync.Mutex
	nextID byte
}

func (t *taggedTTS) Name() string { return "tagged" }

func (t *taggedTTS) Synthesize(_ context.Context, text string, _ *tts.SynthOptions) (tts.PCMStream, error) {
	t.mu.Lock()
	id := t.nextID
	t.nextID++
	t.mu.Unlock()

	// 模拟多 chunk：每句生成若干小块 PCM
	nChunks := 4
	chunkBody := make([]byte, 256)
	for i := range chunkBody {
		chunkBody[i] = id
	}
	data := make([]byte, 0, nChunks*(1+len(chunkBody)))
	for range nChunks {
		data = append(data, id)
		data = append(data, chunkBody...)
	}
	return io.NopCloser(&taggedReader{data: data}), nil
}

type taggedReader struct {
	data []byte
	off  int
}

func (r *taggedReader) Read(p []byte) (int, error) {
	if r.off >= len(r.data) {
		return 0, io.EOF
	}
	const chunkSize = 64
	end := r.off + chunkSize
	if end > len(r.data) {
		end = len(r.data)
	}
	n := copy(p, r.data[r.off:end])
	r.off += n
	return n, nil
}

type pcmCapturingSender struct {
	mu     sync.Mutex
	chunks [][]byte
	done   bool
}

func (s *pcmCapturingSender) SendSTT(string, string)   {}
func (s *pcmCapturingSender) SendText(string)          {}
func (s *pcmCapturingSender) SendTool(string, string)  {}
func (s *pcmCapturingSender) SendDone()                { s.done = true }

func (s *pcmCapturingSender) SendPCMBytes(_ uint32, pcm []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(pcm))
	copy(cp, pcm)
	s.chunks = append(s.chunks, cp)
}

func (s *pcmCapturingSender) snapshot() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]byte, len(s.chunks))
	copy(out, s.chunks)
	return out
}

// sentenceIDsMonotonic 检查 chunk 首字节（句序号）是否单调非递减。
func sentenceIDsMonotonic(chunks [][]byte) bool {
	var cur byte
	for i, chunk := range chunks {
		if len(chunk) == 0 {
			continue
		}
		id := chunk[0]
		if i > 0 && id < cur {
			return false
		}
		cur = id
	}
	return true
}

func TestTurn_TTS_SerialOrder(t *testing.T) {
	ctx := context.Background()
	workspaceRoot := "../../workspace"
	ps, err := persona.NewReloader(workspaceRoot)
	require.NoError(t, err)
	mem := memory.NewMarkdownStore(workspaceRoot)
	sk := mustSkillsReloader(t, workspaceRoot)
	hist := NewHistoryRegistry(5)

	// 三句，每句超过 MinLen=4，且带句末标点触发 softFlush
	llmText := "这是第一句话内容足够长。这是第二句话也要足够长。这是第三句话同样足够长。"

	ag := New(ps, mem, sk, hist, nil,
		asr.NewMock("讲个故事"),
		&multiSentenceLLM{text: llmText},
		&taggedTTS{},
		config.MiniMaxConfig{},
		testAgentCfg(600),
	)

	send := &pcmCapturingSender{}
	require.NoError(t, ag.Turn(ctx, "dev-tts-order", []byte("pcm"), send))
	require.True(t, send.done)

	chunks := send.snapshot()
	require.GreaterOrEqual(t, len(chunks), 6, "expect multiple PCM chunks from 3 sentences")
	assert.True(t, sentenceIDsMonotonic(chunks), "PCM chunks must not interleave across sentences")

	// 至少出现 3 个不同句序号
	seen := map[byte]struct{}{}
	for _, chunk := range chunks {
		if len(chunk) > 0 {
			seen[chunk[0]] = struct{}{}
		}
	}
	assert.GreaterOrEqual(t, len(seen), 3, "expect chunks tagged with 3 sentence IDs")
}
