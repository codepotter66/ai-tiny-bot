package tts

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// MiniMaxConfig 是 MiniMax 流式 TTS 所需的运行时参数。
//
// 协议来源：https://platform.minimaxi.com/docs/mcp  → 同步语音合成
// 端点：wss://api.minimaxi.com/ws/v1/t2a_v2
// 鉴权：Authorization: Bearer <api_key>（与 TB_OPENAI_COMPAT_API_KEY 共用）
type MiniMaxConfig struct {
	APIKey     string        // TB_MINIMAX_API_KEY
	Model      string        // 默认 speech-2.8-turbo
	Voice      string        // 默认 male-qn-qingse
	Format     string        // 默认 pcm
	SampleRate int           // 默认 16000
	WSURL      string        // 可选：测试时覆盖（如 ws://127.0.0.1:12345）
	Timeout    time.Duration // 单次连接超时，默认 15s
	// 测试用：注入 HTTP 客户端（影响 TLS 跳过等）。nil 时用默认。
	HTTPClient *http.Client
}

// MiniMax 是 tts.Synthesizer 的真实实现。
// MiniMax TTS 协议：WebSocket JSON 事件流，音频通过 data.audio 字段（hex 编码 PCM）回传。
type MiniMax struct {
	cfg MiniMaxConfig
}

// NewMiniMax 构造 provider。Model / Voice / Format / SampleRate / Timeout 缺失时填默认值。
func NewMiniMax(cfg MiniMaxConfig) *MiniMax {
	if cfg.Model == "" {
		cfg.Model = "speech-2.8-turbo"
	}
	if cfg.Voice == "" {
		cfg.Voice = "male-qn-qingse"
	}
	if cfg.Format == "" {
		cfg.Format = "pcm"
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 16000
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	return &MiniMax{cfg: cfg}
}

func (m *MiniMax) Name() string { return "minimax" }

func (m *MiniMax) Synthesize(ctx context.Context, text string, opts *SynthOptions) (PCMStream, error) {
	if m.cfg.APIKey == "" {
		return nil, fmt.Errorf("%w: minimax tts requires APIKey (set TB_MINIMAX_API_KEY or TB_OPENAI_COMPAT_API_KEY)", ErrNotImplemented)
	}
	text = strings.TrimSpace(text)
	text = CleanText(text)
	if text == "" {
		// 空文本：返回 ~300ms 静音（PCM 16k16mono = 32000 字节）
		return io.NopCloser(strings.NewReader(string(make([]byte, 32_000)))), nil
	}

	wsURL := m.cfg.WSURL
	if wsURL == "" {
		wsURL = "wss://api.minimaxi.com/ws/v1/t2a_v2"
	}

	ch := make(chan []byte, 32)
	errCh := make(chan error, 1)
	go func() {
		defer close(ch)
		if err := m.runSession(ctx, wsURL, text, opts, ch); err != nil {
			errCh <- err
		}
	}()

	return &chanPCMStream{ch: ch, errCh: errCh, ctx: ctx}, nil
}

// runSession 跑一次 WebSocket TTS 协议。
// 流程：连接 → task_start → 等 task_started → task_continue(text) → 收 audio chunks → task_finish
func (m *MiniMax) runSession(ctx context.Context, wsURL, text string, synthOpts *SynthOptions, out chan<- []byte) error {
	// 1) Dial
	dialCtx, cancel := context.WithTimeout(ctx, m.cfg.Timeout)
	defer cancel()

	dialOpts := &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": []string{"Bearer " + m.cfg.APIKey},
		},
	}
	if m.cfg.HTTPClient != nil {
		dialOpts.HTTPClient = m.cfg.HTTPClient
	}
	conn, _, err := websocket.Dial(dialCtx, wsURL, dialOpts)
	if err != nil {
		return fmt.Errorf("tts minimax: dial: %w", err)
	}
	// 整体 conn 生命周期绑 ctx
	go func() {
		<-ctx.Done()
		_ = conn.Close(websocket.StatusNormalClosure, "ctx done")
	}()
	defer conn.Close(websocket.StatusNormalClosure, "done")

	voiceID := m.cfg.Voice
	languageBoost := ""
	englishNorm := false
	if synthOpts != nil {
		if synthOpts.Voice != "" {
			voiceID = synthOpts.Voice
		}
		languageBoost = synthOpts.LanguageBoost
		englishNorm = synthOpts.EnglishNormalization
	}

	// 2) task_start
	start := map[string]any{
		"event": "task_start",
		"model": m.cfg.Model,
		"voice_setting": map[string]any{
			"voice_id":                voiceID,
			"speed":                   1,
			"vol":                     1,
			"pitch":                   0,
			"english_normalization":   englishNorm,
			"emotion":                 nil,
		},
		"audio_setting": map[string]any{
			"sample_rate": m.cfg.SampleRate,
			"bitrate":     128000,
			"format":      m.cfg.Format,
			"channel":     1,
		},
	}
	if languageBoost != "" {
		start["language_boost"] = languageBoost
	}
	if err := writeJSON(ctx, conn, start); err != nil {
		return fmt.Errorf("tts minimax: send task_start: %w", err)
	}

	// 3) 等 task_started
	if err := waitForEvent(ctx, conn, "task_started"); err != nil {
		return fmt.Errorf("tts minimax: wait task_started: %w", err)
	}

	// 4) task_continue(text)
	cont := map[string]any{
		"event": "task_continue",
		"text":  text,
	}
	if err := writeJSON(ctx, conn, cont); err != nil {
		return fmt.Errorf("tts minimax: send task_continue: %w", err)
	}

	// 5) 收 audio chunks
	for {
		// 检查 ctx
		if err := ctx.Err(); err != nil {
			return err
		}

		_, msg, err := conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("tts minimax: read: %w", err)
		}
		var resp struct {
			Event   string `json:"event"`
			IsFinal bool   `json:"is_final"`
			Data    struct {
				Audio  string `json:"audio"`
				Status int    `json:"status"`
			} `json:"data"`
		}
		if err := json.Unmarshal(msg, &resp); err != nil {
			return fmt.Errorf("tts minimax: decode: %w body=%s", err, truncateBody(msg, 200))
		}

		// 收到 audio → hex 解码 → 推给 channel
		if resp.Data.Audio != "" {
			pcm, err := hex.DecodeString(resp.Data.Audio)
			if err != nil {
				return fmt.Errorf("tts minimax: hex decode: %w", err)
			}
			select {
			case out <- pcm:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// is_final=true 表示这段文本合成完
		if resp.IsFinal {
			break
		}
	}

	// 6) task_finish（礼貌关闭）
	finishCtx, finishCancel := context.WithTimeout(ctx, 2*time.Second)
	defer finishCancel()
	_ = writeJSON(finishCtx, conn, map[string]any{"event": "task_finish"})

	return nil
}

// writeJSON 写一个 JSON 文本帧。
func writeJSON(ctx context.Context, conn *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

// waitForEvent 读取直到收到指定 event 类型。
func waitForEvent(ctx context.Context, conn *websocket.Conn, want string) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, msg, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		var resp struct {
			Event string `json:"event"`
		}
		if err := json.Unmarshal(msg, &resp); err != nil {
			return fmt.Errorf("decode: %w", err)
		}
		if resp.Event == want {
			return nil
		}
	}
}

// chanPCMStream 是 io.ReadCloser，从 chan []byte 读字节。
// 行为：
//   - 优先吐 pending 里残留
//   - 没残留时从 ch 读一块
//   - ctx.Done() → 立即返回 ctx.Err()
//   - ch close → EOF
//   - errCh 有错 → 立即返回错
type chanPCMStream struct {
	ch      <-chan []byte
	errCh   <-chan error
	ctx     context.Context
	mu      sync.Mutex
	pending []byte
	done    bool
}

func (s *chanPCMStream) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.done {
		return 0, io.EOF
	}

	// 1) 优先吐 pending
	if len(s.pending) > 0 {
		n := copy(p, s.pending)
		s.pending = s.pending[n:]
		return n, nil
	}

	// 2) 等下一个 chunk 或结束
	select {
	case chunk, ok := <-s.ch:
		if !ok {
			// ch 关闭，再查 errCh
			select {
			case err := <-s.errCh:
				if err != nil {
					s.done = true
					return 0, err
				}
			default:
			}
			s.done = true
			return 0, io.EOF
		}
		n := copy(p, chunk)
		if n < len(chunk) {
			// chunk 没全吐出去，存到 pending
			s.pending = append(s.pending, chunk[n:]...)
		}
		return n, nil
	case err := <-s.errCh:
		s.done = true
		return 0, err
	case <-s.ctx.Done():
		s.done = true
		return 0, s.ctx.Err()
	}
}

func (s *chanPCMStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return nil
	}
	s.done = true
	// 不主动 close ch（goroutine 自己会关）；只取消
	return nil
}

// 确保 errors 引入（不写也行，defer 中可能要用）
var _ = errors.New
