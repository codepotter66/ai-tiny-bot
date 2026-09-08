package memory

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MarkdownStore Store 接口的 Markdown 文件实现。
type MarkdownStore struct {
	workspaceRoot string
	indexer       MemoryIndexer
	searcher      MemorySearcher
	mu            sync.Mutex // 串行化写同一个文件
}

// MemoryIndexer 可选的记忆索引后端（如 SQLite memory_index）。
type MemoryIndexer interface {
	IndexMemoryLine(ctx context.Context, deviceID, date string, lineNo int, tsMs int64, content string) error
}

// IndexHit 索引召回的一行位置（1-based 行号）。
type IndexHit struct {
	Date string
	Line int
}

// MemorySearcher 用索引缩小 Recall 扫描范围。
type MemorySearcher interface {
	SearchMemory(ctx context.Context, deviceID, query string, lookbackDays int) ([]IndexHit, error)
}

// NewMarkdownStore 构造一个 MarkdownStore。
func NewMarkdownStore(workspaceRoot string) *MarkdownStore {
	return &MarkdownStore{workspaceRoot: workspaceRoot}
}

// SetIndexer 注入可选索引器（Record 时同步写入）。
func (s *MarkdownStore) SetIndexer(idx MemoryIndexer) {
	s.indexer = idx
	if sr, ok := idx.(MemorySearcher); ok {
		s.searcher = sr
	} else {
		s.searcher = nil
	}
}

// Record 追加一行到 workspace/memory/<device>/YYYY-MM-DD.md
// 文件格式：
//   - 第一行：`# YYYY-MM-DD`
//   - 之后每行：`HH:MM:SS [kind] content`
func (s *MarkdownStore) Record(ctx context.Context, deviceID, kind, content string, ts time.Time) error {
	if deviceID == "" {
		return fmt.Errorf("memory: deviceID empty")
	}
	if content == "" {
		return nil
	}
	if ts.IsZero() {
		ts = time.Now()
	}
	date := ts.Format("2006-01-02")
	tsStr := ts.Format("15:04:05")

	dir := deviceDir(s.workspaceRoot, deviceID)
	path := filepath.Join(dir, date+".md")

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("memory mkdir: %w", err)
	}
	exists := fileExists(path)
	lineNo := 0
	if exists {
		lineNo, _ = countLines(path)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("memory open: %w", err)
	}
	if !exists {
		if _, err := fmt.Fprintf(f, "# %s\n", date); err != nil {
			_ = f.Close()
			return err
		}
		lineNo = 1
	}
	if _, err := fmt.Fprintf(f, "%s [%s] %s\n", tsStr, kind, strings.TrimSpace(content)); err != nil {
		_ = f.Close()
		return err
	}
	_ = f.Close()
	lineNo++
	if s.indexer != nil {
		line := fmt.Sprintf("%s [%s] %s", tsStr, kind, strings.TrimSpace(content))
		_ = s.indexer.IndexMemoryLine(ctx, deviceID, date, lineNo, ts.UnixMilli(), line)
	}
	return nil
}

// Recall 召回 top-k 片段。
// 有 SearchMemory 且命中非空时只打分索引行；否则扫描最近 lookbackDays 的 .md。
func (s *MarkdownStore) Recall(ctx context.Context, deviceID, query string, lookbackDays, k int) ([]Snippet, error) {
	if s.searcher != nil {
		hits, err := s.searcher.SearchMemory(ctx, deviceID, query, lookbackDays)
		if err == nil && len(hits) > 0 {
			return s.recallFromHits(deviceID, query, hits, k)
		}
	}
	return s.recallScanFiles(deviceID, query, lookbackDays, k)
}

func (s *MarkdownStore) recallFromHits(deviceID, query string, hits []IndexHit, k int) ([]Snippet, error) {
	queryKW := tokenize(query)
	now := time.Now()
	dir := deviceDir(s.workspaceRoot, deviceID)
	var scored []Snippet
	seen := map[string]struct{}{}
	for _, h := range hits {
		key := h.Date + ":" + fmt.Sprintf("%d", h.Line)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		line, ok := readFileLine(filepath.Join(dir, h.Date+".md"), h.Line)
		if !ok {
			continue
		}
		score := scoreLine(queryKW, line, now)
		if score <= 0 {
			continue
		}
		d, err := time.Parse("2006-01-02", h.Date)
		ts := int64(0)
		if err == nil {
			ts = d.UnixMilli()
		}
		scored = append(scored, Snippet{
			Date:    h.Date,
			Line:    h.Line,
			TSMs:    ts,
			Content: line,
			Score:   score,
		})
	}
	return topKSnippets(scored, k), nil
}

func (s *MarkdownStore) recallScanFiles(deviceID, query string, lookbackDays, k int) ([]Snippet, error) {
	dir := deviceDir(s.workspaceRoot, deviceID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("memory readdir: %w", err)
	}

	queryKW := tokenize(query)
	now := time.Now()
	var scored []Snippet

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		date := strings.TrimSuffix(e.Name(), ".md")
		if _, err := time.Parse("2006-01-02", date); err != nil {
			continue
		}
		d, _ := time.Parse("2006-01-02", date)
		if int(now.Sub(d).Hours()/24) > lookbackDays {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scan := bufio.NewScanner(f)
		scan.Buffer(make([]byte, 64*1024), 1024*1024)
		lineNo := 0
		for scan.Scan() {
			lineNo++
			line := scan.Text()
			if lineNo == 1 || strings.TrimSpace(line) == "" {
				continue
			}
			score := scoreLine(queryKW, line, now)
			if score <= 0 {
				continue
			}
			scored = append(scored, Snippet{
				Date:    date,
				Line:    lineNo,
				TSMs:    d.UnixMilli(),
				Content: line,
				Score:   score,
			})
		}
		f.Close()
	}
	return topKSnippets(scored, k), nil
}

func topKSnippets(scored []Snippet, k int) []Snippet {
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].Score > scored[i].Score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}
	if k > 0 && len(scored) > k {
		scored = scored[:k]
	}
	return scored
}

func readFileLine(path string, lineNo int) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	n := 0
	for scan.Scan() {
		n++
		if n == lineNo {
			return scan.Text(), true
		}
	}
	return "", false
}

// TodayPath 返回 device 今天的文件路径（不保证存在）。
func (s *MarkdownStore) TodayPath(deviceID string) string {
	return filepath.Join(deviceDir(s.workspaceRoot, deviceID), TodayKey()+".md")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n := 0
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		n++
	}
	return n, scan.Err()
}
