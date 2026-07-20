// Package asr 提供 STT（语音转文字）抽象。
//
// 抽象原因：v1 用 mock 实现；后续可接入 Aliyun Paraformer、Whisper、Volc 等，
// 切换时不动 agent 核心代码。
package asr

import (
	"context"
	"errors"
	"io"
)

// PCMStream 上行的 PCM 字节流读取端。
type PCMStream io.Reader

// Result 转写结果。
type Result struct {
	Text     string // 转写文本
	Duration int    // 音频时长（毫秒）
	Language string // "zh" / "en" / "auto"
}

// Transcriber STT 抽象。
type Transcriber interface {
	// Transcribe 把 pcm 流（16 kHz / 16 bit / mono）转成文本。
	Transcribe(ctx context.Context, pcm PCMStream) (*Result, error)
	// Name 返回 provider 名称（用于日志/指标）。
	Name() string
}

// ErrNotImplemented 标识当前 provider 尚未实现。
var ErrNotImplemented = errors.New("asr: provider not implemented")
