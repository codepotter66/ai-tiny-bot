package skills

import (
	"fmt"
	"sync"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"
)

// Registry 维护 name → Skill 的查找表，并可被 LLM 当作 tools 暴露。
type Registry struct {
	mu       sync.RWMutex
	byName   map[string]*Skill
	builtins map[string]*Builtin
}

// NewRegistry 构造一个空的 registry。
func NewRegistry() *Registry {
	return &Registry{byName: map[string]*Skill{}, builtins: map[string]*Builtin{}}
}

// Add 注册一个 skill。重名时返回错误。
func (r *Registry) Add(s *Skill) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byName[s.Name]; ok {
		return fmt.Errorf("skill %q already registered", s.Name)
	}
	r.byName[s.Name] = s
	return nil
}

// Get 按名查找。
func (r *Registry) Get(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.byName[name]
	return s, ok
}

// Names 返回全部 skill 名。
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.byName))
	for n := range r.byName {
		out = append(out, n)
	}
	return out
}

// AddBuiltin 注册内置工具（可覆盖同名脚本 skill）。
func (r *Registry) AddBuiltin(b *Builtin) error {
	if b == nil || b.Name == "" {
		return fmt.Errorf("builtin name required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.builtins[b.Name] = b
	return nil
}

// GetBuiltin 按名查找内置工具。
func (r *Registry) GetBuiltin(name string) (*Builtin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.builtins[name]
	return b, ok
}

// ToolDefs 返回所有 skill 的 LLM tool 描述。
func (r *Registry) ToolDefs() []llm.ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]llm.ToolDef, 0, len(r.byName)+len(r.builtins))
	seen := map[string]struct{}{}
	for _, b := range r.builtins {
		out = append(out, b.ToToolDef())
		seen[b.Name] = struct{}{}
	}
	for _, s := range r.byName {
		if _, ok := seen[s.Name]; ok {
			continue
		}
		out = append(out, s.ToToolDef())
	}
	return out
}

// LoadFromDir 一次性从 workspace 根目录加载所有 skill 并注册。
func LoadFromDir(workspaceRoot string) (*Registry, error) {
	all, err := LoadAll(workspaceRoot)
	if err != nil {
		return nil, err
	}
	r := NewRegistry()
	for _, s := range all {
		if err := r.Add(s); err != nil {
			return nil, err
		}
	}
	return r, nil
}
