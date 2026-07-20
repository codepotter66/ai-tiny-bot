package tts

import (
	"context"
	"io"
	"math"
	"strings"
	"sync"
	"time"
)

// Mock 把文字长度映射到一段固定时长的正弦波 PCM。
// 用途：跑通 pipeline 而不消耗真实 TTS 配额。
type Mock struct {
	SampleRate   int     // 默认 16000
	FreqHz       float64 // 默认 440
	Amplitude    int16   // 默认 12000
	CharsPerSec  float64 // 默认 6（中文字符/秒），1 秒约 6 个字
}

func NewMock() *Mock { return &Mock{SampleRate: 16000, FreqHz: 440, Amplitude: 12000, CharsPerSec: 6} }

func (m *Mock) Name() string { return "mock" }

// Synthesize 返回一个 io.ReadCloser，读取完整 PCM 流。
// 字节长度 = SampleRate * 2 * duration_seconds
func (m *Mock) Synthesize(ctx context.Context, text string, _ *SynthOptions) (PCMStream, error) {
	text = strings.TrimSpace(text)
	dur := 0.5
	if n := len([]rune(text)); n > 0 {
		dur = float64(n) / m.CharsPerSec
		if dur < 0.3 {
			dur = 0.3
		}
		if dur > 8.0 {
			dur = 8.0
		}
	}
	samples := int(float64(m.SampleRate) * dur)
	buf := make([]byte, samples*2) // int16 little-endian

	for i := 0; i < samples; i++ {
		t := float64(i) / float64(m.SampleRate)
		// 简单正弦 + 轻微包络避免咔哒声
		env := 1.0
		if i < 200 {
			env = float64(i) / 200
		} else if i > samples-200 {
			env = float64(samples-i) / 200
		}
		v := int16(float64(m.Amplitude) * env * math.Sin(2*math.Pi*m.FreqHz*t))
		// little-endian
		buf[2*i] = byte(v)
		buf[2*i+1] = byte(v >> 8)
	}

	return &pcmReadCloser{
		Reader:   &bytesWithCancel{buf: buf, ctx: ctx},
		closeFn:  func() error { return nil },
		consumed: &sync.Once{},
	}, nil
}

// pcmReadCloser 包了 ReadCloser 行为，支持 ctx 取消
type pcmReadCloser struct {
	io.Reader
	closeFn  func() error
	consumed *sync.Once
}

func (p *pcmReadCloser) Close() error {
	var err error
	p.consumed.Do(func() { err = p.closeFn() })
	return err
}

// bytesWithCancel 支持 ctx 取消的 byte reader
type bytesWithCancel struct {
	buf []byte
	off int
	ctx context.Context
}

func (b *bytesWithCancel) Read(p []byte) (int, error) {
	if b.ctx != nil {
		select {
		case <-b.ctx.Done():
			return 0, b.ctx.Err()
		default:
		}
	}
	if b.off >= len(b.buf) {
		return 0, io.EOF
	}
	n := copy(p, b.buf[b.off:])
	b.off += n
	return n, nil
}

// 静音一段（占位用）
func SilencePCM(ms int) []byte {
	samples := 16000 * ms / 1000
	return make([]byte, samples*2)
}

// Sleep 短延时（用于测试节奏）
var Sleep = func(d time.Duration) { time.Sleep(d) }
