package memory

import (
	"strings"
	"time"
	"unicode"
)

// tokenize 与 store.extractKeywords 同策略（中文单字 + 英文/数字词）。
func tokenize(s string) map[string]struct{} {
	s = strings.ToLower(s)
	// 中英标点替换为空格
	repl := strings.NewReplacer(
		"、", " ", "，", " ", "。", " ", "！", " ", "？", " ",
		"；", " ", "：", " ", "（", " ", "）", " ", "《", " ", "》", " ",
		"\"", " ", "'", " ", "\n", " ", "\t", " ",
		",", " ", ".", " ", ";", " ", ":", " ", "!", " ", "?", " ",
		"(", " ", ")", " ", "[", " ", "]", " ",
	)
	s = repl.Replace(s)

	out := make(map[string]struct{})
	var buf []rune
	flush := func() {
		if len(buf) > 0 {
			t := string(buf)
			if len([]rune(t)) >= 2 {
				out[t] = struct{}{}
			}
			buf = buf[:0]
		}
	}
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Han, r):
			flush()
			out[string(r)] = struct{}{} // CJK 单字
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			buf = append(buf, r)
		default:
			flush()
		}
	}
	flush()
	return out
}

// scoreLine 返回该行与 query 的相关度。
//   - 关键词重合度 × 0.6
//   - 时间衰减（30 天线性） × 0.4
func scoreLine(queryKW map[string]struct{}, line string, now time.Time) float64 {
	if len(queryKW) == 0 {
		return 0
	}
	lineKW := tokenize(line)
	if len(lineKW) == 0 {
		return 0
	}
	hits := 0
	for k := range queryKW {
		if _, ok := lineKW[k]; ok {
			hits++
		}
	}
	kwScore := float64(hits) / float64(len(queryKW))
	if kwScore == 0 {
		return 0
	}

	// 解析行内时间戳 "HH:MM:SS [kind] content" → 当日时间戳
	recency := recencyWeight(line, now)
	return kwScore*0.6 + recency*0.4
}

// recencyWeight 基于行内 HH:MM:SS + 当时"今天日期"算出的 time.Time，
// 距 now 越近得分越高。30 天线性衰减到 0。
func recencyWeight(line string, now time.Time) float64 {
	ts := parseLineTime(line, now)
	if ts.IsZero() {
		return 0
	}
	days := now.Sub(ts).Hours() / 24
	if days < 0 {
		days = 0
	}
	const horizon = 30.0
	w := 1 - days/horizon
	if w < 0 {
		w = 0
	}
	return w
}

// parseLineTime 从 "HH:MM:SS [kind] content" 提取时间，结合 now 的日期。
func parseLineTime(line string, now time.Time) time.Time {
	// 跳过日期行 "# 2026-06-04"
	if strings.HasPrefix(line, "# ") {
		return time.Time{}
	}
	// 找第一个空格
	sp := strings.IndexByte(line, ' ')
	if sp < 6 {
		return time.Time{}
	}
	candidate := line[:sp]
	t, err := time.Parse("15:04:05", candidate)
	if err != nil {
		return time.Time{}
	}
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, now.Location())
}
