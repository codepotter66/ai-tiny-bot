package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectLengthMode_Long(t *testing.T) {
	cases := []string{
		"你能讲个故事吗",
		"为什么天是蓝的",
		"详细说一下",
		"然后呢",
		"继续讲",
	}
	for _, c := range cases {
		assert.Equal(t, LengthLong, DetectLengthMode(c), c)
	}
}

func TestDetectLengthMode_Short(t *testing.T) {
	cases := []string{
		"你在干什么",
		"你好",
		"",
	}
	for _, c := range cases {
		assert.Equal(t, LengthShort, DetectLengthMode(c), c)
	}
}

func TestLengthDirective(t *testing.T) {
	assert.Contains(t, LengthDirective("讲个故事"), "3–8 句")
	assert.Contains(t, LengthDirective("你好"), "1–2 句")
}
