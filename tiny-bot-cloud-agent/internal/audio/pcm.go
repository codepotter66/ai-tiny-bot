// Package audio 提供 PCM 字节工具与流式句子聚合。
package audio

import (
	"encoding/base64"
	"fmt"
)

// PCM16MonoHeader 是 16 kHz / 16 bit / 单声道 PCM 流的元信息。
// v1 我们只支持这一种，固件侧 I2S 配置就是这套。
type PCM16MonoHeader struct {
	SampleRate int // 16000
	BitDepth   int // 16
	Channels   int // 1
}

// DefaultHeader 返回固件约定的 header。
func DefaultHeader() PCM16MonoHeader {
	return PCM16MonoHeader{SampleRate: 16000, BitDepth: 16, Channels: 1}
}

// BytesToPCM16 把 base64 字符串解码为 PCM 16-bit little-endian 字节切片。
func BytesToPCM16(b64 string) ([]byte, error) {
	if b64 == "" {
		return nil, nil
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("audio: base64 decode: %w", err)
	}
	if len(raw)%2 != 0 {
		return nil, fmt.Errorf("audio: pcm16 byte length %d not even", len(raw))
	}
	return raw, nil
}

// PCM16ToBytes 把 PCM 字节切片编码为 base64 字符串。
func PCM16ToBytes(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// DurationSeconds 返回 PCM16 mono 流的时长（秒）。
func (h PCM16MonoHeader) DurationSeconds(byteLen int) float64 {
	const bytesPerSample = 2
	bytesPerSecond := h.SampleRate * h.Channels * bytesPerSample
	if bytesPerSecond == 0 {
		return 0
	}
	return float64(byteLen) / float64(bytesPerSecond)
}

// FrameBytes 返回指定毫秒时长的 PCM 字节数（用于 chunking）。
func (h PCM16MonoHeader) FrameBytes(ms int) int {
	const bytesPerSample = 2
	return h.SampleRate * h.Channels * bytesPerSample * ms / 1000
}
