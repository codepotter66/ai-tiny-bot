package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/audio"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/logging"
)

// Session 每条 WebSocket 连接的状态机。
//
// 生命周期：
//  1. readLoop 收 hello → 鉴权 → 发 hello.ok
//  2. 收 audio chunks → 累加到 audioBuf
//  3. 收 end → audioBuf 交给 ASR → 触发 turn
//  4. 收 interrupt → 取消当前 turn
//  5. 关闭 → cancel 所有 goroutine
type Session struct {
	conn    *websocket.Conn
	device  string
	turnCtx context.Context
	cancel  context.CancelFunc

	// TurnTimeout 单轮 turn 超时；≤0 时用默认 120s。
	TurnTimeout time.Duration

	// 写入互斥（coder/websocket 是并发安全读但写不是）
	writeMu sync.Mutex

	// 当前 turn 的音频缓冲
	audioMu  sync.Mutex
	audioBuf []byte

	// 注入的回调（agent 包实现）
	OnHello     func(ctx context.Context, deviceID, token string) (sampleRate int, proto int, errCode string)
	OnTurn      func(ctx context.Context, deviceID string, pcm []byte) error
	OnInterrupt func(ctx context.Context, deviceID string)
	OnClose     func(ctx context.Context, deviceID string)

	turnMu     sync.Mutex
	turnCancel context.CancelFunc
}

// NewSession 构造 session。
func NewSession(conn *websocket.Conn) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{conn: conn, turnCtx: ctx, cancel: cancel}
}

// Run 阻塞直到连接关闭。
func (s *Session) Run() {
	defer s.cancel()
	defer func() {
		if s.OnClose != nil && s.device != "" {
			s.OnClose(s.turnCtx, s.device)
		}
	}()
	defer s.conn.Close(websocket.StatusNormalClosure, "bye")

	// readLoop
	for {
		_, data, err := s.conn.Read(context.Background())
		if err != nil {
			// 正常关闭或网络错误都退出
			return
		}
		var msg ClientMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			s.sendError(ErrInternal, "bad json")
			continue
		}
		switch msg.Type {
		case MsgHello:
			s.handleHello(msg)
		case MsgAudio:
			s.handleAudio(msg)
		case MsgEnd:
			s.handleEnd()
		case MsgInterrupt:
			s.handleInterrupt()
		case MsgPing:
			s.send(ServerMsg{Type: MsgPong})
		default:
			s.sendError(ErrProto, "unknown type "+msg.Type)
		}
	}
}

func (s *Session) handleHello(msg ClientMsg) {
	if s.OnHello == nil {
		s.sendError(ErrInternal, "no hello handler")
		return
	}
	sr, proto, errCode := s.OnHello(s.turnCtx, msg.DeviceID, msg.Token)
	if errCode != "" {
		s.sendError(errCode, "auth failed")
		_ = s.conn.Close(websocket.StatusPolicyViolation, errCode)
		return
	}
	s.device = msg.DeviceID
	// 把带 device_id 的 logger 挂到 ctx
	s.turnCtx = logging.WithDevice(s.turnCtx, msg.DeviceID)
	s.send(ServerMsg{
		Type:       MsgHelloOK,
		SampleRate: sr,
		Proto:      proto,
	})
	slog.Info("ws: hello ok", "device", msg.DeviceID, "sample_rate", sr)
}

func (s *Session) handleAudio(msg ClientMsg) {
	pcm, err := audio.BytesToPCM16(msg.Data)
	if err != nil {
		slog.Debug("audio decode failed", "device", s.device, "err", err, "b64_len", len(msg.Data))
		s.sendError(ErrInternal, "bad pcm")
		return
	}
	s.audioMu.Lock()
	s.audioBuf = append(s.audioBuf, pcm...)
	total := len(s.audioBuf)
	s.audioMu.Unlock()
	slog.Debug("audio chunk", "device", s.device, "seq", msg.Seq, "bytes", len(pcm), "total_buf", total)
}

func (s *Session) handleEnd() {
	if s.OnTurn == nil {
		return
	}
	s.audioMu.Lock()
	pcm := s.audioBuf
	s.audioBuf = nil
	s.audioMu.Unlock()

	slog.Info("turn start", "device", s.device, "pcm_bytes", len(pcm))

	// v1.1.2 修复：即使 buffer 为空也发 done，避免客户端死锁卡在「处理中」
	// （客户端期望收到 done 才能切回「已连接」状态）
	if len(pcm) == 0 {
		slog.Info("turn skipped: empty audio buffer", "device", s.device)
		s.sendError(ErrProto, "empty audio")
		s.send(ServerMsg{Type: MsgDone})
		return
	}
	// 异步跑 turn，不阻塞 read loop
	go func() {
		timeout := s.TurnTimeout
		if timeout <= 0 {
			timeout = 120 * time.Second
		}
		turnCtx, turnCancel := context.WithTimeout(s.turnCtx, timeout)
		s.turnMu.Lock()
		s.turnCancel = turnCancel
		s.turnMu.Unlock()
		defer func() {
			s.turnMu.Lock()
			s.turnCancel = nil
			s.turnMu.Unlock()
			turnCancel()
		}()
		if err := s.OnTurn(turnCtx, s.device, pcm); err != nil {
			if turnCtx.Err() != nil {
				slog.Info("turn cancelled", "device", s.device)
				return
			}
			slog.Error("turn failed", "device", s.device, "err", err)
			s.sendError(turnErrorCode(err), err.Error())
			s.send(ServerMsg{Type: MsgDone})
		}
	}()
}

func (s *Session) handleInterrupt() {
	s.turnMu.Lock()
	cancel := s.turnCancel
	s.turnMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if s.OnInterrupt != nil {
		s.OnInterrupt(s.turnCtx, s.device)
	}
}

// SendPCMBytes 把 PCM 字节块发到客户端（base64 包装）。
func (s *Session) SendPCMBytes(seq uint32, pcm []byte) {
	s.send(ServerMsg{Type: MsgAudioDn, Seq: seq, Data: audio.PCM16ToBytes(pcm)})
}

// SendText 发一段中间文本（用于 debug 模式展示）。
func (s *Session) SendText(text string) {
	s.send(ServerMsg{Type: MsgText, Text: text})
}

// SendSTT 发 STT 结果。
func (s *Session) SendSTT(text, lang string) {
	s.send(ServerMsg{Type: MsgSTT, Text: text, Lang: lang})
}

// SendTool 发工具调用事件。
func (s *Session) SendTool(name, args string) {
	s.send(ServerMsg{Type: MsgTool, Tool: name, Args: args})
}

// SendStatus 发任务步骤进度（静默，不进 TTS）。
// progress < 0 时省略 progress 字段；≥0 时写入 0.0–1.0。
func (s *Session) SendStatus(step, phase, text string, progress float64) {
	m := ServerMsg{Type: MsgStatus, Step: step, Phase: phase, Text: text}
	if progress >= 0 {
		p := progress
		m.Progress = &p
	}
	s.send(m)
}

// SendDone 发 turn 结束。
func (s *Session) SendDone() {
	s.send(ServerMsg{Type: MsgDone})
}

func (s *Session) sendError(code, msg string) {
	s.send(ServerMsg{Type: MsgError, Err: code + ":" + msg})
}

func (s *Session) send(m ServerMsg) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.conn.Write(ctx, websocket.MessageText, mustJSON(m)); err != nil {
		// 写失败基本意味着连接已断
		slog.Debug("ws write failed", "err", err)
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"type":"error","error":"marshal"}`)
	}
	return b
}

// ErrSessionClosed 表示 session 已关闭。
var ErrSessionClosed = errors.New("session closed")

func turnErrorCode(err error) string {
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "asr:"):
		return ErrSTT
	case strings.HasPrefix(msg, "tts:"):
		return ErrTTS
	default:
		return ErrLLM
	}
}
