// Package agent 实现 turn 循环：ASR → 拼消息 → LLM 流式 → 句子聚合 → TTS。
package agent

import (
	"sync"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"
)

// History 每设备的对话历史（ring buffer，按 turn 数量裁剪）。
type History struct {
	mu    sync.Mutex
	turns int
	items []llm.Message
}

// NewHistory 构造一个最多保留 maxTurns 个 turn 的 history。
// 1 turn = 1 user + 1 assistant。
func NewHistory(maxTurns int) *History {
	if maxTurns <= 0 {
		maxTurns = 20
	}
	return &History{turns: maxTurns}
}

// Append 加一条消息。
func (h *History) Append(m llm.Message) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.items = append(h.items, m)
	h.trim()
}

// Snapshot 返回当前消息快照（不持有锁）。
func (h *History) Snapshot() []llm.Message {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]llm.Message, len(h.items))
	copy(out, h.items)
	return out
}

// Reset 清空。
func (h *History) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.items = h.items[:0]
}

// Load 用持久化消息覆盖内存历史（仅 user/assistant）。
func (h *History) Load(msgs []llm.Message) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.items = make([]llm.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == llm.RoleUser || m.Role == llm.RoleAssistant {
			h.items = append(h.items, m)
		}
	}
	h.trim()
}

// trim 保留最近 maxTurns*2 条。
func (h *History) trim() {
	max := h.turns * 2
	if max <= 0 {
		return
	}
	if len(h.items) > max {
		h.items = h.items[len(h.items)-max:]
	}
}

// HistoryRegistry per-device History 容器。
type HistoryRegistry struct {
	mu       sync.Mutex
	maxTurns int
	byDevice map[string]*History
}

// NewHistoryRegistry 构造 registry。
func NewHistoryRegistry(maxTurns int) *HistoryRegistry {
	return &HistoryRegistry{maxTurns: maxTurns, byDevice: map[string]*History{}}
}

// Get 取 device 的 history，没有就建。
func (r *HistoryRegistry) Get(deviceID string) *History {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.byDevice[deviceID]; ok {
		return h
	}
	h := NewHistory(r.maxTurns)
	r.byDevice[deviceID] = h
	return h
}
