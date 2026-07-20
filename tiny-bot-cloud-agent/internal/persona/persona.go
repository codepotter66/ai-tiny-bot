// Package persona 加载并缓存 agent 的人设 / 身份 / 行为 / 用户画像 Markdown。
//
// 文件结构（位于 workspace_root 下）：
//   - SOUL.md       人设（声音、语气、性格）
//   - IDENTITY.md   身份（名字、能力、边界）
//   - AGENT.md      行为规则（怎么说话、什么时候停）
//   - USER.md       默认用户画像
//
// 启动时一次加载进内存；Reload() 用于 SIGHUP 热重载。
package persona

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Persona 加载后的内存表示。
type Persona struct {
	Soul     string
	Identity string
	Agent    string
	User     string

	// sourceDir 便于 Reload 与调试日志。
	sourceDir string

	// loadedAt 用于观察是否被热重载。
	loadedAt time.Time
}

// Load 从 dir 加载 4 个 Markdown 文件。
// 文件缺失会返回 error（v1 要求 4 个文件都存在，行为可控）。
func Load(dir string) (*Persona, error) {
	files := map[string]*string{
		"SOUL.md":     nil,
		"IDENTITY.md": nil,
		"AGENT.md":    nil,
		"USER.md":     nil,
	}
	ptrs := map[string]**string{
		"SOUL.md":     nil,
		"IDENTITY.md": nil,
		"AGENT.md":    nil,
		"USER.md":     nil,
	}
	_ = files
	_ = ptrs

	p := &Persona{sourceDir: dir}
	var errs []string
	for name, target := range map[string]*string{
		"SOUL.md":     &p.Soul,
		"IDENTITY.md": &p.Identity,
		"AGENT.md":    &p.Agent,
		"USER.md":     &p.User,
	} {
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				errs = append(errs, fmt.Sprintf("missing %s", name))
				continue
			}
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		*target = strings.TrimSpace(string(raw))
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("persona load failed in %s: %s", dir, strings.Join(errs, ", "))
	}
	p.loadedAt = time.Now()
	return p, nil
}

// SystemPrompt 把 4 个文件拼成 system prompt 字符串。
// 顺序：SOUL → IDENTITY → AGENT → USER
// 每个段用 \n\n---\n\n 分隔。
func (p *Persona) SystemPrompt() string {
	var b strings.Builder
	for i, s := range []string{p.Soul, p.Identity, p.Agent, p.User} {
		if i > 0 {
			b.WriteString("\n\n---\n\n")
		}
		b.WriteString(s)
	}
	return b.String()
}

// SourceDir 返回工作区根目录。
func (p *Persona) SourceDir() string { return p.sourceDir }

// LoadedAt 返回加载时间。
func (p *Persona) LoadedAt() time.Time { return p.loadedAt }

// Reloader 支持 SIGHUP 热重载。
type Reloader struct {
	dir string
	mu  sync.RWMutex
	p   *Persona
}

// NewReloader 构造一个 Reloader 并加载初始 persona。
func NewReloader(dir string) (*Reloader, error) {
	p, err := Load(dir)
	if err != nil {
		return nil, err
	}
	return &Reloader{dir: dir, p: p}, nil
}

// Get 返回当前 persona 的快照。
func (r *Reloader) Get() *Persona {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.p
}

// Reload 重新读盘。失败时保留旧 persona（保证服务不挂）。
func (r *Reloader) Reload() error {
	p, err := Load(r.dir)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.p = p
	r.mu.Unlock()
	return nil
}
