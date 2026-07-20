package llm

import (
	"strings"
)

type thinkingBlock struct {
	open  string
	close string
}

const (
	tagRedactedOpen  = "<" + "redacted_thinking" + ">"
	tagRedactedClose = "</" + "redacted_thinking" + ">"
	tagThinkOpen     = "<" + "think" + ">"
	tagThinkClose    = "</" + "think" + ">"
)

// 推理模型可能在 content 里混入的思考块标签。
var thinkingBlocks = []thinkingBlock{
	{open: tagRedactedOpen, close: tagRedactedClose},
	{open: tagThinkOpen, close: tagThinkClose},
}

// StreamThinkingFilter 从 LLM 流式 token 中剥掉 thinking 块，只保留可见回复。
type StreamThinkingFilter struct {
	buf        string
	inThinking bool
	active     thinkingBlock
}

// NewStreamThinkingFilter 构造流式 thinking 过滤器。
func NewStreamThinkingFilter() *StreamThinkingFilter {
	return &StreamThinkingFilter{}
}

// Feed 输入原始 token，返回可展示给 UI/TTS 的文本片段（可能为空）。
func (f *StreamThinkingFilter) Feed(chunk string) string {
	if chunk == "" {
		return ""
	}
	f.buf += chunk
	var out strings.Builder
	for f.buf != "" {
		if f.inThinking {
			idx := strings.Index(f.buf, f.active.close)
			if idx >= 0 {
				f.buf = f.buf[idx+len(f.active.close):]
				f.inThinking = false
				f.active = thinkingBlock{}
				continue
			}
			hold := holdSuffixForAnyClose(f.buf)
			if hold > 0 {
				f.buf = f.buf[:len(f.buf)-hold]
			} else {
				f.buf = ""
			}
			break
		}

		openIdx, block := findEarliestOpen(f.buf)
		if openIdx >= 0 {
			out.WriteString(f.buf[:openIdx])
			f.buf = f.buf[openIdx+len(block.open):]
			f.inThinking = true
			f.active = block
			continue
		}

		hold := holdSuffixForAnyOpen(f.buf)
		if hold > 0 {
			out.WriteString(f.buf[:len(f.buf)-hold])
			f.buf = f.buf[len(f.buf)-hold:]
			break
		}
		out.WriteString(f.buf)
		f.buf = ""
		break
	}
	return out.String()
}

// Flush 在流结束时吐出剩余可见文本；未闭合的 thinking 尾巴丢弃。
func (f *StreamThinkingFilter) Flush() string {
	if f.inThinking {
		f.buf = ""
		f.inThinking = false
		f.active = thinkingBlock{}
		return ""
	}
	rest := f.buf
	f.buf = ""
	return rest
}

// StripThinking 从完整字符串中移除所有 thinking 块（非流式辅助）。
func StripThinking(s string) string {
	f := NewStreamThinkingFilter()
	var out strings.Builder
	out.WriteString(f.Feed(s))
	out.WriteString(f.Flush())
	return out.String()
}

func findEarliestOpen(s string) (int, thinkingBlock) {
	best := -1
	var block thinkingBlock
	for _, b := range thinkingBlocks {
		if i := strings.Index(s, b.open); i >= 0 && (best < 0 || i < best) {
			best = i
			block = b
		}
	}
	return best, block
}

func holdSuffixForAnyOpen(s string) int {
	best := 0
	for _, b := range thinkingBlocks {
		if strings.HasPrefix(b.open, s) && len(s) < len(b.open) {
			if len(s) > best {
				best = len(s)
			}
		}
		if n := partialPrefixLen(s, b.open); n > best {
			best = n
		}
	}
	if best > 0 {
		return best
	}
	return 0
}

func holdSuffixForAnyClose(s string) int {
	best := 0
	for _, b := range thinkingBlocks {
		if strings.HasPrefix(b.close, s) && len(s) < len(b.close) {
			if len(s) > best {
				best = len(s)
			}
		}
		if n := partialPrefixLen(s, b.close); n > best {
			best = n
		}
	}
	if best > 0 {
		return best
	}
	return 0
}

// partialPrefixLen 返回 s 末尾与 target 前缀匹配的长度（不含完整 target）。
func partialPrefixLen(s, target string) int {
	max := min(len(s), len(target)-1)
	for n := max; n > 0; n-- {
		if strings.HasSuffix(s, target[:n]) {
			return n
		}
	}
	return 0
}
