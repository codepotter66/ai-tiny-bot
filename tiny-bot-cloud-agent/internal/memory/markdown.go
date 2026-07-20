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
	mu            sync.Mutex // 串行化写同一个文件
}

// MemoryIndexer 可选的记忆索引后端（如 SQLite memory_index）。
type MemoryIndexer interface {
	IndexMemoryLine(ctx context.Context, deviceID, date string, lineNo int, tsMs int64, content string) error
}

// NewMarkdownStore 构造一个 MarkdownStore。
func NewMarkdownStore(workspaceRoot string) *MarkdownStore {
	return &MarkdownStore{workspaceRoot: workspaceRoot}
}

// SetIndexer 注入可选索引器（Record 时同步写入）。
func (s *MarkdownStore) SetIndexer(idx MemoryIndexer) {
	s.indexer = idx
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
// 实现：扫描 device 目录下最近 lookbackDays 的 .md 文件，每行打 keyword × 0.6 + recency × 0.4。
func (s *MarkdownStore) Recall(ctx context.Context, deviceID, query string, lookbackDays, k int) ([]Snippet, error) {
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
		// 简单日期校验
		if _, err := time.Parse("2006-01-02", date); err != nil {
			continue
		}
		// lookback 过滤
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

	// 排序 + 取 top-k
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
	return scored, nil
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
