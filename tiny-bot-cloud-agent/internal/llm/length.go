package llm

import "strings"

// LengthMode 回复长度档位。
type LengthMode int

const (
	LengthShort LengthMode = iota
	LengthLong
)

var longReplyKeywords = []string{
	"讲故事", "讲个故事", "讲一个故事", "说个故事", "来个故事",
	"为什么", "怎么回事", "解释一下", "详细", "继续", "然后呢",
	"再说说", "多说点", "展开", "介绍", "教程", "步骤",
}

// DetectLengthMode 根据用户文本判断回复长度档位。
func DetectLengthMode(userText string) LengthMode {
	t := strings.ToLower(strings.TrimSpace(userText))
	if t == "" {
		return LengthShort
	}
	for _, kw := range longReplyKeywords {
		if strings.Contains(t, kw) {
			return LengthLong
		}
	}
	return LengthShort
}

// LengthDirective 返回追加到 system prompt 的长度指令。
func LengthDirective(userText string) string {
	if DetectLengthMode(userText) == LengthLong {
		return "用户需要较长回复：讲故事、解释或「继续/详细说」时，可回复 3–8 句，分段讲述，每 2–3 句自然停顿；不要只给一句确认就停。"
	}
	return "日常闲聊：回复 1–2 句即可，简短口语化，说完留出空隙等对方。"
}
