package audio

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAggregator_FlushOnPeriod(t *testing.T) {
	var out []string
	a := NewAggregator(100, 0, func(s string) { out = append(out, s) })
	a.Push("你好")
	a.Push("，今天")
	a.Push("吃啥")
	a.Push("？")
	assert.Equal(t, []string{"你好，今天吃啥？"}, out)
}

func TestAggregator_MultipleSentences(t *testing.T) {
	var out []string
	a := NewAggregator(100, 0, func(s string) { out = append(out, s) })
	a.Push("第一句。第二句！第三句？")
	assert.Equal(t, []string{"第一句。", "第二句！", "第三句？"}, out)
}

func TestAggregator_ForceFlushOnMaxLen(t *testing.T) {
	var out []string
	a := NewAggregator(5, 0, func(s string) { out = append(out, s) })
	a.Push("abcdefghij") // 10 rune, max 5 → 触发两次 hard flush
	assert.Equal(t, []string{"abcde", "fghij"}, out)
	a.Flush() // 缓冲已空，no-op
	assert.Equal(t, []string{"abcde", "fghij"}, out)
}

func TestAggregator_FlushOnNewline(t *testing.T) {
	var out []string
	a := NewAggregator(100, 0, func(s string) {out = append(out, s) })
	a.Push("第一行\n第二行")
	a.Flush() // 流式结束后需要外部 Flush
	assert.Equal(t, []string{"第一行", "第二行"}, out)
}

func TestAggregator_MinLenSuppressesShort(t *testing.T) {
	var out []string
	a := NewAggregator(100, 3, func(s string) { out = append(out, s) })
	a.Push("好。")
	assert.Empty(t, out) // 短于 MinLen
	a.Flush()
	assert.Equal(t, []string{"好。"}, out)
}

func TestAggregator_NoFlushInsideSentence(t *testing.T) {
	var out []string
	a := NewAggregator(100, 0, func(s string) { out = append(out, s) })
	// 半角逗号/冒号/分号不触发 flush；全角"、，；："也一样
	a.Push("他说: \"你好啊, 小朋友\"")
	assert.Empty(t, out, "should not flush on comma or colon")
	a.Push("，然后呢？")
	assert.Equal(t, []string{"他说: \"你好啊, 小朋友\"，然后呢？"}, out)
}

func TestAggregator_EmptyPushNoop(t *testing.T) {
	var out []string
	a := NewAggregator(100, 0, func(s string) { out = append(out, s) })
	a.Push("")
	a.Push("。")
	assert.Equal(t, []string{"。"}, out)
}

// 基准：每 token 1 ns 级
func BenchmarkAggregator(b *testing.B) {
	var total int
	a := NewAggregator(80, 0, func(s string) { total += len(s) })
	tokens := []string{"你", "好", "，", "我", "是", "小", "陪", "。"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tk := range tokens {
			a.Push(tk)
		}
	}
	_ = strings.Builder{}
	_ = total
}
