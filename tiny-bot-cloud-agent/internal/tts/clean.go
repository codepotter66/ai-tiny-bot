package tts

import (
	"regexp"
	"strings"
)

// bracketGarbage 匹配 LLM 幻觉出的方括号语气乱码，如 [e~[、[e~]、[e[。
// 仅匹配 ASCII 方括号内容，不误伤正常中文。
var bracketGarbage = regexp.MustCompile(`(?:\s*\[[a-zA-Z~^\]]{0,20}\[?\s*)+`)

// trailingBracket 匹配流式截断后残留的孤立 [ 或半个标记。
var trailingBracket = regexp.MustCompile(`\s*\[+[^\]]*$`)

// multiSpace 折叠连续空白。
var multiSpace = regexp.MustCompile(`\s{2,}`)

// CleanText 移除 LLM 幻觉出的方括号语气乱码，保留正常可读文本。
// 官方圆括号语气词如 (laughs) 不会被剥离。
func CleanText(s string) string {
	if s == "" {
		return ""
	}
	s = bracketGarbage.ReplaceAllString(s, "")
	s = trailingBracket.ReplaceAllString(s, "")
	s = multiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
