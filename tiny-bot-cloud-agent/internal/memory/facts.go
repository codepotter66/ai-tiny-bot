package memory

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	factsFileName   = "facts.md"
	factsHeader     = "# Facts\n"
	defaultFactsMax = 80
)

func factsPath(workspaceRoot, deviceID string) string {
	return filepath.Join(deviceDir(workspaceRoot, deviceID), factsFileName)
}

func normalizeFactContent(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func resolveFactsMax(max int) int {
	if max <= 0 {
		return defaultFactsMax
	}
	return max
}

// SaveFact 追加一条事实到 facts.md；规范化后全文相同则跳过；超出 max 删最旧。
func (s *MarkdownStore) SaveFact(ctx context.Context, deviceID, content string, ts time.Time, max int) error {
	_ = ctx
	if deviceID == "" {
		return fmt.Errorf("memory: deviceID empty")
	}
	content = normalizeFactContent(content)
	if content == "" {
		return nil
	}
	if ts.IsZero() {
		ts = time.Now()
	}
	date := ts.Format("2006-01-02")
	max = resolveFactsMax(max)

	dir := deviceDir(s.workspaceRoot, deviceID)
	path := factsPath(s.workspaceRoot, deviceID)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("memory facts mkdir: %w", err)
	}

	facts, err := readFactsFile(path)
	if err != nil {
		return err
	}
	for _, f := range facts {
		if normalizeFactContent(f.Content) == content {
			return nil // 去重
		}
	}
	facts = append(facts, Fact{Date: date, Content: content})
	if len(facts) > max {
		facts = facts[len(facts)-max:]
	}
	return writeFactsFile(path, facts)
}

// ListFacts 读取事实层；最多返回 max 条（最旧在前）。
func (s *MarkdownStore) ListFacts(ctx context.Context, deviceID string, max int) ([]Fact, error) {
	_ = ctx
	if deviceID == "" {
		return nil, nil
	}
	max = resolveFactsMax(max)
	path := factsPath(s.workspaceRoot, deviceID)

	s.mu.Lock()
	defer s.mu.Unlock()

	facts, err := readFactsFile(path)
	if err != nil {
		return nil, err
	}
	if len(facts) > max {
		facts = facts[len(facts)-max:]
	}
	return facts, nil
}

func readFactsFile(path string) ([]Fact, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("memory facts open: %w", err)
	}
	defer f.Close()

	var out []Fact
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 64*1024), 1024*1024)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fact, ok := parseFactLine(line)
		if !ok {
			continue
		}
		out = append(out, fact)
	}
	if err := scan.Err(); err != nil {
		return nil, fmt.Errorf("memory facts scan: %w", err)
	}
	return out, nil
}

// parseFactLine 解析 "- [YYYY-MM-DD] content" 或宽松 "- content"。
func parseFactLine(line string) (Fact, bool) {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "- ") {
		line = strings.TrimSpace(line[2:])
	}
	if line == "" {
		return Fact{}, false
	}
	if strings.HasPrefix(line, "[") {
		end := strings.IndexByte(line, ']')
		if end > 1 {
			date := line[1:end]
			content := normalizeFactContent(line[end+1:])
			if content == "" {
				return Fact{}, false
			}
			if _, err := time.Parse("2006-01-02", date); err == nil {
				return Fact{Date: date, Content: content}, true
			}
		}
	}
	return Fact{Content: normalizeFactContent(line)}, true
}

func writeFactsFile(path string, facts []Fact) error {
	var b strings.Builder
	b.WriteString(factsHeader)
	for _, f := range facts {
		date := f.Date
		if date == "" {
			date = TodayKey()
		}
		fmt.Fprintf(&b, "- [%s] %s\n", date, f.Content)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("memory facts write: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("memory facts rename: %w", err)
	}
	return nil
}
