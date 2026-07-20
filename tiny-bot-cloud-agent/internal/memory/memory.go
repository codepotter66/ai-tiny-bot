// Package memory 提供 agent 的长期记忆能力。
//
// 设计：
//   - 情节日志：workspace/memory/<device_id>/YYYY-MM-DD.md（按天分文件）
//   - 事实巩固：workspace/memory/<device_id>/facts.md（不受 lookback 限制）
//   - 情节召回：关键词 + 时间衰减打分（不引入向量库）
//   - Recall / ListFacts 返回片段，由调用方拼到 system prompt
package memory

import (
	"context"
	"fmt"
	"time"
)

// Snippet 召回的一条情节记忆片段。
type Snippet struct {
	Date    string // YYYY-MM-DD
	Line    int    // 1-based
	TSMs    int64
	Content string
	Score   float64
}

// Fact 一条巩固事实（跨 lookback 始终可用）。
type Fact struct {
	Date    string // YYYY-MM-DD
	Content string
}

// Store 记忆读写接口。
type Store interface {
	// Record 把一段对话写入今日情节日志。
	// ts 可以为零（= 当前时间）；kind 区分 user / assistant / system。
	Record(ctx context.Context, deviceID, kind, content string, ts time.Time) error

	// Recall 召回与 query 最相关的 Top-K 情节片段。
	// lookbackDays：只考虑最近 N 天；k：返回数量上限。
	Recall(ctx context.Context, deviceID, query string, lookbackDays, k int) ([]Snippet, error)

	// SaveFact 把重要信息写入事实层（facts.md）；max 为保留条数上限（<=0 用默认）。
	SaveFact(ctx context.Context, deviceID, content string, ts time.Time, max int) error

	// ListFacts 读取事实层（最多 max 条，旧的在前；<=0 用默认上限）。
	ListFacts(ctx context.Context, deviceID string, max int) ([]Fact, error)

	// TodayPath 返回今日情节日志路径（用于"今天说了什么"等场景）。
	TodayPath(deviceID string) string
}

// TodayKey 返回 YYYY-MM-DD（设备本地时区，UTC+8 默认）。
func TodayKey() string {
	return time.Now().Format("2006-01-02")
}

// formatTS 毫秒时间戳 → 简短字符串，便于写入记忆文件。
func formatTS(ts int64) string {
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}
	return time.UnixMilli(ts).Format("15:04:05")
}

// deviceDir 返回 workspace/memory/<device_id>，不存在会建。
func deviceDir(workspaceRoot, deviceID string) string {
	return fmt.Sprintf("%s/memory/%s", workspaceRoot, deviceID)
}
