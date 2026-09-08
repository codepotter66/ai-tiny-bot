package main

import (
	"context"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/memory"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/store"
)

// memIndex 把 SQLite memory_index 接到 MarkdownStore 的写入与召回。
type memIndex struct {
	*store.Store
}

func (m memIndex) SearchMemory(ctx context.Context, deviceID, query string, lookbackDays int) ([]memory.IndexHit, error) {
	hits, err := m.Store.MemorySearchQuery(ctx, deviceID, query, lookbackDays)
	if err != nil {
		return nil, err
	}
	out := make([]memory.IndexHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, memory.IndexHit{Date: h.Date, Line: h.Line})
	}
	return out, nil
}
