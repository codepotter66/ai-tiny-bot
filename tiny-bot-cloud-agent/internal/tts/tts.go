// Package tts 提供 TTS（文字转语音）抽象。
package tts

import (
	"context"
	"errors"
	"io"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/config"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/lang"
)

// PCMStream 下行的 PCM 字节流（16 kHz / 16 bit / mono）。
type PCMStream io.ReadCloser

// SynthOptions 单次合成的可选覆盖参数。
type SynthOptions struct {
	Voice               string // 覆盖 voice_id，空则用 provider 默认
	LanguageBoost       string // 覆盖 language_boost
	EnglishNormalization bool  // 英语文本规范化
}

// SynthOptionsFromProfile 把配置里的 VoiceProfile 转成 SynthOptions。
func SynthOptionsFromProfile(p config.VoiceProfile, code lang.Code) *SynthOptions {
	return &SynthOptions{
		Voice:                p.VoiceID,
		LanguageBoost:        p.LanguageBoost,
		EnglishNormalization: code == lang.En,
	}
}

// Synthesizer TTS 抽象。
type Synthesizer interface {
	// Synthesize 合成一段文本为 PCM 流。opts 为 nil 时使用 provider 默认配置。
	Synthesize(ctx context.Context, text string, opts *SynthOptions) (PCMStream, error)
	// Name 返回 provider 名称。
	Name() string
}

// ErrNotImplemented 标识当前 provider 尚未实现。
var ErrNotImplemented = errors.New("tts: provider not implemented")
