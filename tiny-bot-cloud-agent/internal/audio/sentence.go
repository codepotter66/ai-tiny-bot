package audio

import (
	"strings"
	"unicode/utf8"
)

// Aggregator 把 LLM 的 token 流聚合成完整句子。
//
// 触发 flush 的条件（任一）：
//   - 遇到终结标点 。！？!? 之一
//   - 遇到换行
//   - 缓冲长度超过 MaxLen
//
// 句中遇到 ,;:;，；、 等"软"标点不清空，等真正的终结。
type Aggregator struct {
	MaxLen int          // 缓冲上限（rune 数），超过强制 flush
	MinLen int          // 最小 flush 长度（rune 数），避免单独发一个"嗯"
	flush  func(string) // 句完成回调
	buf    strings.Builder
}

// NewAggregator 构造一个聚合器。flush 不可为 nil。
func NewAggregator(maxLen, minLen int, flush func(string)) *Aggregator {
	if maxLen <= 0 {
		maxLen = 80
	}
	if minLen < 0 {
		minLen = 0
	}
	if flush == nil {
		flush = func(string) {}
	}
	return &Aggregator{MaxLen: maxLen, MinLen: minLen, flush: flush}
}

// Push 追加一段 token（可能不是完整 UTF-8 安全；内部按 rune 处理）。
func (a *Aggregator) Push(token string) {
	for _, r := range token {
		a.buf.WriteRune(r)
		if isSentenceEnd(r) || r == '\n' {
			a.softFlush()
			continue
		}
		if utf8.RuneCountInString(a.buf.String()) >= a.MaxLen {
			a.hardFlush()
		}
	}
}

// Flush 强制把残余缓冲输出（不补句号），忽略 MinLen。
func (a *Aggregator) Flush() {
	a.hardFlush()
}

// hardFlush 强制输出，忽略 MinLen。用于 MaxLen 溢出与外部 Flush()。
func (a *Aggregator) hardFlush() {
	s := strings.TrimSpace(a.buf.String())
	a.buf.Reset()
	if s == "" {
		return
	}
	a.flush(s)
}

// softFlush 在遇到句末标点时调用。缓冲短于 MinLen 则不输出（保留到下次 flush）。
func (a *Aggregator) softFlush() {
	s := strings.TrimSpace(a.buf.String())
	if s == "" {
		a.buf.Reset()
		return
	}
	if utf8.RuneCountInString(s) < a.MinLen {
		// 太短，保留在缓冲里等下一段
		return
	}
	a.buf.Reset()
	a.flush(s)
}

// isSentenceEnd 判断是否句末标点。
func isSentenceEnd(r rune) bool {
	switch r {
	case '。', '！', '？', '.', '!', '?':
		return true
	}
	return false
}
