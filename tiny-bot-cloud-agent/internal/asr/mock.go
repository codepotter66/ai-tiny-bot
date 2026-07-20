package asr

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/lang"
)

// Mock 是一个固定回显的 Transcriber，把 PCM 全部读完，返回预设文本。
// 用于 v1 联调，不做真正的语音识别。
type Mock struct {
	// Reply 固定返回的文本。空时按下列优先级：
	//   1. 环境变量 TB_FAKE_ASR_TEXT
	//   2. fmt 拼的 "(mock 听到了 N 字节)"
	Reply string
	// ForceEmpty 为 true 时强制返回空文本（用于测试空 STT 短路）。
	ForceEmpty bool
	// EchoSeconds 模拟处理耗时（秒），用于压测 pipeline
	EchoSeconds float64
}

// NewMock 构造一个返回固定文本的 mock。
func NewMock(reply string) *Mock {
	return &Mock{Reply: reply}
}

func (m *Mock) Name() string { return "mock" }

func (m *Mock) Transcribe(ctx context.Context, pcm PCMStream) (*Result, error) {
	// 模拟读流：把全部字节丢弃并统计大小
	n, _ := io.Copy(io.Discard, pcm)

	if m.EchoSeconds > 0 {
		select {
		case <-time.After(time.Duration(m.EchoSeconds * float64(time.Second))):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	reply := m.Reply
	if m.ForceEmpty {
		reply = ""
	} else if reply == "" {
		if v := os.Getenv("TB_FAKE_ASR_TEXT"); v != "" {
			reply = v
		} else {
			reply = fmt.Sprintf("（mock 听到了 %d 字节的音频）", n)
		}
	}
	return &Result{Text: reply, Duration: 0, Language: lang.Detect(reply).String()}, nil
}

// TrimSilence 工具：去掉首尾空白
func TrimSilence(s string) string { return strings.TrimSpace(s) }
