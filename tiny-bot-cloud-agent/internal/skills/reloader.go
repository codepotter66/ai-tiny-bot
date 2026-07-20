package skills

import (
	"fmt"
	"sync"
)

// Reloader 支持 SIGHUP 热重载 skills registry。
type Reloader struct {
	dir      string
	mu       sync.RWMutex
	registry *Registry
}

// NewReloader 从 workspace 加载初始 registry。
func NewReloader(workspaceRoot string) (*Reloader, error) {
	reg, err := LoadFromDir(workspaceRoot)
	if err != nil {
		return nil, err
	}
	return &Reloader{dir: workspaceRoot, registry: reg}, nil
}

// Get 返回当前 registry 快照指针（调用方只读使用）。
func (r *Reloader) Get() *Registry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.registry
}

// Reload 重新扫描 workspace/skills。失败时保留旧 registry。
func (r *Reloader) Reload() error {
	reg, err := LoadFromDir(r.dir)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.registry = reg
	r.mu.Unlock()
	return nil
}

// RegisterBuiltins 把内置工具注册到当前 registry（每次 reload 后需重新调用）。
func (r *Reloader) RegisterBuiltins(builtins ...*Builtin) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range builtins {
		if err := r.registry.AddBuiltin(b); err != nil {
			return fmt.Errorf("register builtin %q: %w", b.Name, err)
		}
	}
	return nil
}
