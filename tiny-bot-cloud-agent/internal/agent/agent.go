package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/asr"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/audio"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/config"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/lang"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/logging"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/memory"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/persona"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/skills"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/store"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/tts"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/ws"
)

// Sender 把 turn 产物发回 client 的抽象（ws.Session 实现）。
type Sender interface {
	SendSTT(text, lang string)
	SendText(string)
	SendTool(name, args string)
	SendStatus(step, phase, text string, progress float64)
	SendPCMBytes(seq uint32, pcm []byte)
	SendDone()
}

// Agent 把所有依赖组合起来，跑 turn。
type Agent struct {
	ASR           asr.Transcriber
	LLM           llm.Chat
	TTS           tts.Synthesizer
	Persona       *persona.Reloader
	Memory        memory.Store
	Skills        *skills.Reloader
	History       *HistoryRegistry
	Store         *store.Store
	VoiceProfiles config.MiniMaxConfig
	AgentCfg      config.AgentConfig
	WorkspaceRoot string // workspace 根；code.write/run 的 scratch 在此下

	SeqCounter *uint32 // 单调递增的 audio chunk 序号
	seqMu      sync.Mutex
}

// New 构造 Agent。SeqCounter 在内部创建。
func New(
	personaR *persona.Reloader,
	mem memory.Store,
	sk *skills.Reloader,
	hist *HistoryRegistry,
	st *store.Store,
	asrImpl asr.Transcriber,
	llmImpl llm.Chat,
	ttsImpl tts.Synthesizer,
	voiceProfiles config.MiniMaxConfig,
	agentCfg config.AgentConfig,
) *Agent {
	return &Agent{
		ASR:           asrImpl,
		LLM:           llmImpl,
		TTS:           ttsImpl,
		Persona:       personaR,
		Memory:        mem,
		Skills:        sk,
		History:       hist,
		Store:         st,
		VoiceProfiles: voiceProfiles,
		AgentCfg:      agentCfg,
		SeqCounter:    new(uint32),
	}
}

func (a *Agent) skills() *skills.Registry {
	if a.Skills == nil {
		return skills.NewRegistry()
	}
	return a.Skills.Get()
}

// Turn 跑一轮：audio → STT → LLM(stream) → sentence → TTS → SendPCM。
func (a *Agent) Turn(ctx context.Context, deviceID string, pcm []byte, send Sender) error {
	logger := logging.FromContext(ctx)
	logger.Info("turn start", "pcm_bytes", len(pcm))

	if a.Store != nil && a.AgentCfg.DailyTokenCapPerDevice > 0 {
		tin, tout, err := a.Store.SumDeviceTokensToday(ctx, deviceID)
		if err == nil && tin+tout >= a.AgentCfg.DailyTokenCapPerDevice {
			logger.Warn("daily token cap reached", "tokens_in", tin, "tokens_out", tout, "cap", a.AgentCfg.DailyTokenCapPerDevice)
			send.SendText("今天聊太久啦，明天再找我吧。")
			a.synthAndSend(ctx, "今天聊太久啦，明天再找我吧。", send)
			send.SendDone()
			return nil
		}
	}

	sttRes, err := a.ASR.Transcribe(ctx, bytes.NewReader(pcm))
	if err != nil {
		return fmt.Errorf("asr: %w", err)
	}
	userText := strings.TrimSpace(sttRes.Text)
	inputLang := lang.Detect(userText)
	if sttRes.Language != "" && sttRes.Language != "auto" {
		inputLang = lang.Code(sttRes.Language)
	}
	logger.Info("stt", "text", userText, "lang", inputLang)
	send.SendSTT(userText, inputLang.String())

	if userText == "" {
		send.SendDone()
		logger.Info("turn skipped: empty stt")
		return nil
	}

	now := time.Now()
	_ = a.Memory.Record(ctx, deviceID, "user", userText, now)

	var convID string
	if a.Store != nil {
		conv, err := a.Store.OpenConversation(ctx, deviceID)
		if err == nil {
			convID = conv.ID
			_ = a.Store.AppendMessageWithTokens(ctx, convID, &store.Message{
				ID: store.NewID(), Role: "user", Content: userText,
			})
		}
	}

	hist := a.History.Get(deviceID)
	hist.Append(llm.Message{Role: llm.RoleUser, Content: userText})

	snippets, _ := a.Memory.Recall(ctx, deviceID, userText, a.AgentCfg.MemoryLookbackDays, a.AgentCfg.MemoryRecallK)
	llmSnips := make([]llm.Snippet, 0, len(snippets))
	for _, s := range snippets {
		llmSnips = append(llmSnips, llm.Snippet{Date: s.Date, Content: s.Content, Score: s.Score})
	}

	facts, _ := a.Memory.ListFacts(ctx, deviceID, a.AgentCfg.FactsMax)
	llmFacts := make([]llm.Fact, 0, len(facts))
	for _, f := range facts {
		llmFacts = append(llmFacts, llm.Fact{Date: f.Date, Content: f.Content})
	}

	ps := a.Persona.Get()
	soulMD := a.resolveSoulMD(ctx, deviceID, ps.Soul)
	userMD := a.resolveUserMD(ctx, deviceID, ps.User)
	reg := a.skills()
	a.registerMemoryBuiltins(reg, deviceID, now)
	a.registerPersonaBuiltins(reg, deviceID)
	a.registerCodeBuiltins(reg, deviceID)
	tools := reg.ToolDefs()

	sys := llm.BuildSystemPrompt(&llm.PersonaInputs{
		Soul: soulMD, Identity: ps.Identity, Agent: ps.Agent, User: userMD,
		Facts: llmFacts, MemorySnippets: llmSnips, Tools: tools,
	})
	sys += "\n\n---\n\n" + llm.LanguageDirective(inputLang)
	sys += "\n\n" + llm.LengthDirective(userText)
	msgs := []llm.Message{{Role: llm.RoleSystem, Content: sys}}
	msgs = append(msgs, hist.Snapshot()...)

	var fullText bytes.Buffer
	ttsQueue := make(chan string, 8)
	var ttsWg sync.WaitGroup
	ttsWg.Add(1)
	go func() {
		defer ttsWg.Done()
		for text := range ttsQueue {
			if ctx.Err() != nil {
				return
			}
			a.synthAndSend(ctx, text, send)
		}
	}()
	agg := audio.NewAggregator(80, 4, func(sent string) {
		fullText.WriteString(sent)
		fullText.WriteString(" ")
		logger.Info("sentence", "text", sent)
		ttsQueue <- sent
	})

	var respChars int
	truncated := false
	onVisible := func(visible string) error {
		if truncated {
			return nil
		}
		visible = tts.CleanText(visible)
		if visible == "" {
			return nil
		}
		maxChars := a.AgentCfg.MaxResponseChars
		if maxChars > 0 && respChars+len([]rune(visible)) > maxChars {
			remain := maxChars - respChars
			if remain > 0 {
				runes := []rune(visible)
				visible = string(runes[:remain])
			} else {
				visible = ""
			}
			if visible != "" {
				send.SendText(visible)
				agg.Push(visible)
				respChars += len([]rune(visible))
			}
			agg.Flush()
			truncated = true
			logger.Info("response truncated", "max_chars", maxChars)
			return nil
		}
		respChars += len([]rune(visible))
		send.SendText(visible)
		agg.Push(visible)
		return nil
	}

	text, tokensIn, tokensOut, err := a.runToolLoop(ctx, msgs, tools, send, deviceID, now, onVisible)
	if err != nil {
		if ctx.Err() != nil {
			close(ttsQueue)
			ttsWg.Wait()
			return ctx.Err()
		}
		return fmt.Errorf("llm: %w", err)
	}
	agg.Flush()

	reply := strings.TrimSpace(fullText.String())
	if reply == "" {
		reply = strings.TrimSpace(text)
	}
	hist.Append(llm.Message{Role: llm.RoleAssistant, Content: reply})
	_ = a.Memory.Record(ctx, deviceID, "assistant", reply, now)

	if a.Store != nil && convID != "" {
		_ = a.Store.AppendMessageWithTokens(ctx, convID, &store.Message{
			ID: store.NewID(), Role: "assistant", Content: reply,
			TokensIn: tokensIn, TokensOut: tokensOut,
		})
	}

	close(ttsQueue)
	ttsWg.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	send.SendDone()
	logger.Info("turn done", "chars", len(reply), "tokens_in", tokensIn, "tokens_out", tokensOut)
	return nil
}

func (a *Agent) resolveUserMD(ctx context.Context, deviceID, defaultUser string) string {
	if a.Store == nil || deviceID == "" {
		return defaultUser
	}
	dev, err := a.Store.GetDevice(ctx, deviceID)
	if err != nil {
		return defaultUser
	}
	user, err := a.Store.GetUser(ctx, dev.UserID)
	if err != nil || strings.TrimSpace(user.UserMD) == "" {
		return defaultUser
	}
	return user.UserMD
}

func (a *Agent) resolveSoulMD(ctx context.Context, deviceID, defaultSoul string) string {
	if a.Store == nil || deviceID == "" {
		return defaultSoul
	}
	dev, err := a.Store.GetDevice(ctx, deviceID)
	if err != nil {
		return defaultSoul
	}
	user, err := a.Store.GetUser(ctx, dev.UserID)
	if err != nil || strings.TrimSpace(user.SoulMD) == "" {
		return defaultSoul
	}
	return user.SoulMD
}

func (a *Agent) userIDForDevice(ctx context.Context, deviceID string) (string, error) {
	if a.Store == nil {
		return "", fmt.Errorf("store unavailable")
	}
	if deviceID == "" {
		return "", fmt.Errorf("device_id empty")
	}
	dev, err := a.Store.GetDevice(ctx, deviceID)
	if err != nil {
		return "", fmt.Errorf("device not found")
	}
	if strings.TrimSpace(dev.UserID) == "" {
		return "", fmt.Errorf("device has no user")
	}
	return dev.UserID, nil
}

func (a *Agent) restoreHistory(ctx context.Context, deviceID string) {
	if a.Store == nil || deviceID == "" {
		return
	}
	limit := a.AgentCfg.HistoryTurns * 2
	msgs, err := a.Store.ListRecentMessages(ctx, deviceID, limit)
	if err != nil || len(msgs) == 0 {
		return
	}
	out := make([]llm.Message, 0, len(msgs))
	for _, m := range msgs {
		role := llm.Role(m.Role)
		switch role {
		case llm.RoleUser, llm.RoleAssistant:
			out = append(out, llm.Message{Role: role, Content: m.Content})
		default:
		}
	}
	a.History.Get(deviceID).Load(out)
}

func (a *Agent) synthAndSend(ctx context.Context, text string, send Sender) {
	logger := logging.FromContext(ctx)
	text = tts.CleanText(text)
	if text == "" {
		return
	}
	sentenceLang := lang.Detect(text)
	profile := a.VoiceProfiles.VoiceProfileFor(sentenceLang)
	opts := tts.SynthOptionsFromProfile(profile, sentenceLang)
	logger.Info("tts route", "lang", sentenceLang, "voice", profile.VoiceID, "boost", profile.LanguageBoost)
	stream, err := a.TTS.Synthesize(ctx, text, opts)
	if err != nil {
		logger.Error("tts synth", "err", err, "text", text)
		return
	}
	defer stream.Close()

	buf := make([]byte, 32*1024)
	for {
		if ctx.Err() != nil {
			return
		}
		n, err := stream.Read(buf)
		if n > 0 {
			a.seqMu.Lock()
			*a.SeqCounter++
			seq := *a.SeqCounter
			a.seqMu.Unlock()
			send.SendPCMBytes(seq, buf[:n])
		}
		if err != nil {
			if err != io.EOF {
				logger.Debug("tts read", "err", err)
			}
			return
		}
	}
}

// Bind 把 session 跟 agent 绑起来（设置 OnHello/OnTurn/OnInterrupt/OnClose）。
func (a *Agent) Bind(s *ws.Session, authVerify func(ctx context.Context, deviceID, token string) (bool, error)) {
	s.OnHello = func(ctx context.Context, deviceID, token string) (int, int, string) {
		if deviceID == "" {
			return 0, 0, ws.ErrProto
		}
		ok, err := authVerify(ctx, deviceID, token)
		if err != nil {
			return 0, 0, ws.ErrInternal
		}
		if !ok {
			return 0, 0, ws.ErrAuth
		}
		a.restoreHistory(ctx, deviceID)
		_ = slog.Default()
		return 16000, 1, ""
	}
	s.OnTurn = func(ctx context.Context, deviceID string, pcm []byte) error {
		return a.Turn(ctx, deviceID, pcm, s)
	}
	s.OnInterrupt = func(ctx context.Context, deviceID string) {
		logger := logging.FromContext(ctx).With("device_id", deviceID)
		logger.Info("interrupt")
	}
	s.OnClose = func(ctx context.Context, deviceID string) {
		if a.Store == nil || deviceID == "" {
			return
		}
		convID, err := a.Store.GetOpenConversationID(ctx, deviceID)
		if err != nil || convID == "" {
			return
		}
		_ = a.Store.CloseConversation(ctx, convID)
	}
}
