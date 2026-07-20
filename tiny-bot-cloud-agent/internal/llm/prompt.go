package llm

import (
	"fmt"
	"strings"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/lang"
)

// BuildSystemPrompt 把 SOUL/IDENTITY/AGENT/USER + 事实 + 情节片段 + skill 描述拼成 system prompt。
//
// 顺序：
//  1. SOUL
//  2. IDENTITY
//  3. AGENT
//  4. USER（可被 per-device 覆盖）
//  5. 已知事实（巩固层，不受 lookback 限制）
//  6. 召回的情节记忆片段（按分数从高到低）
//  7. 可用 skill 列表（tool 描述）
//
// 1-4 跨 turn 静态，触发 provider prompt 缓存。
func BuildSystemPrompt(p *PersonaInputs) string {
	var b strings.Builder

	// 1-4
	parts := []string{p.Soul, p.Identity, p.Agent, p.User}
	for _, s := range parts {
		if s == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n---\n\n")
		}
		b.WriteString(s)
	}

	// 5 巩固事实
	if len(p.Facts) > 0 {
		b.WriteString("\n\n---\n\n## 已知事实\n")
		for _, f := range p.Facts {
			if f.Date != "" {
				fmt.Fprintf(&b, "- [%s] %s\n", f.Date, f.Content)
			} else {
				fmt.Fprintf(&b, "- %s\n", f.Content)
			}
		}
	}

	// 6 情节记忆
	if len(p.MemorySnippets) > 0 {
		b.WriteString("\n\n---\n\n## 历史记忆片段（按相关度）\n")
		for _, s := range p.MemorySnippets {
			fmt.Fprintf(&b, "- [%s] %s\n", s.Date, s.Content)
		}
	}

	// 7 skill 列表
	if len(p.Tools) > 0 {
		b.WriteString("\n\n---\n\n## 可用技能\n")
		for _, t := range p.Tools {
			fmt.Fprintf(&b, "- %s: %s\n", t.Name, t.Description)
		}
		b.WriteString("\n需要时调用对应 skill；不必解释调用过程。\n")
	}

	return b.String()
}

// LanguageDirective 根据检测到的语种追加回复语言指令。
func LanguageDirective(code lang.Code) string {
	switch code {
	case lang.Yue:
		return "请用粤语口语回复用户。"
	case lang.En:
		return "Reply to the user in natural English."
	default:
		return "请用普通话口语回复用户。"
	}
}

// PersonaInputs 构造 system prompt 的输入。
type PersonaInputs struct {
	Soul           string
	Identity       string
	Agent          string
	User           string
	Facts          []Fact
	MemorySnippets []Snippet
	Tools          []ToolDef
}

// Fact 简化的巩固事实（避免与 internal/memory 强耦合）。
type Fact struct {
	Date    string
	Content string
}

// Snippet 简化的情节记忆片段（避免与 internal/memory 强耦合）。
type Snippet struct {
	Date    string
	Content string
	Score   float64
}
