package tts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanText_UserExample(t *testing.T) {
	garbage := strings.Repeat(" [e~[", 50)
	input := "好啦好啦，小陪不说啦～ 那小主人想听什么故事呀～" + garbage
	want := "好啦好啦，小陪不说啦～ 那小主人想听什么故事呀～"
	assert.Equal(t, want, CleanText(input))
}

func TestCleanText_PureGarbage(t *testing.T) {
	assert.Equal(t, "", CleanText("[e~[ [e~[ [e~["))
}

func TestCleanText_NormalChinese(t *testing.T) {
	input := "你好呀～今天想听什么故事？"
	assert.Equal(t, input, CleanText(input))
}

func TestCleanText_KeepsOfficialParenTags(t *testing.T) {
	input := "真好笑(laughs)，我们去玩吧。"
	assert.Equal(t, input, CleanText(input))
}

func TestCleanText_TrailingPartialBracket(t *testing.T) {
	assert.Equal(t, "故事呀～", CleanText("故事呀～ ["))
	assert.Equal(t, "故事呀～", CleanText("故事呀～ [e"))
}

func TestCleanText_Empty(t *testing.T) {
	assert.Equal(t, "", CleanText(""))
	assert.Equal(t, "", CleanText("   "))
}
