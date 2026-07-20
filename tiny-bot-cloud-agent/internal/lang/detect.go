// Package lang 提供基于文本的语种检测（普通话 / 粤语 / 英语）。
package lang

import (
	"strings"
	"unicode"
)

// Code 是标准化语种代码。
type Code string

const (
	Zh  Code = "zh"  // 普通话
	Yue Code = "yue" // 粤语
	En  Code = "en"  // 英语
)

// String 返回代码字面量。
func (c Code) String() string { return string(c) }

// Label 返回面向用户的语种名称。
func (c Code) Label() string {
	switch c {
	case Zh:
		return "普通话"
	case Yue:
		return "粤语"
	case En:
		return "English"
	default:
		return string(c)
	}
}

// cantoneseMarkers 粤语特征词/助词（长词优先匹配由调用方逐词扫描）。
var cantoneseMarkers = []string{
	"点呀", "点啊", "咩事", "边个", "几时", "点解", "做咩", "好耐", "咁样", "系咪", "唔系",
	"嘅", "喺", "佢", "咗", "啲", "咁", "噉", "乜", "冇", "嚟", "睇", "唔",
}

// Detect 根据转写文本推断语种。
// 优先级：英语 > 粤语 > 普通话。
func Detect(text string) Code {
	text = strings.TrimSpace(text)
	if text == "" {
		return Zh
	}

	latin, cjk, other := countScript(text)
	total := latin + cjk + other
	if total == 0 {
		return Zh
	}

	// 基本为拉丁文本 → 英语
	if latin > 0 && cjk == 0 && float64(latin)/float64(total) >= 0.6 {
		return En
	}
	if isMostlyASCII(text) {
		return En
	}

	yueHits := countMarkers(text, cantoneseMarkers)
	if yueHits >= 1 {
		return Yue
	}

	if cjk > 0 {
		return Zh
	}
	if latin > 0 {
		return En
	}
	return Zh
}

func countScript(text string) (latin, cjk, other int) {
	for _, r := range text {
		switch {
		case isLatin(r):
			latin++
		case isCJK(r):
			cjk++
		case !unicode.IsSpace(r):
			other++
		}
	}
	return latin, cjk, other
}

func isLatin(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r)
}

func isMostlyASCII(text string) bool {
	if text == "" {
		return false
	}
	ascii := 0
	total := 0
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		total++
		if r < 128 && (isLatin(r) || unicode.IsDigit(r) || unicode.IsPunct(r)) {
			ascii++
		}
	}
	return total > 0 && float64(ascii)/float64(total) >= 0.85
}

func countMarkers(text string, markers []string) int {
	n := 0
	for _, m := range markers {
		if strings.Contains(text, m) {
			n++
		}
	}
	return n
}
