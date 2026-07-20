package llm

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMock_StreamsEcho(t *testing.T) {
	m := &Mock{Prefix: "A:", Suffix: ".", PerTokenDelay: 0}
	var got strings.Builder
	end := false
	err := m.Chat(context.Background(), []Message{
		{Role: RoleUser, Content: "hi"},
	}, nil, func(tok string, e bool) error {
		if e {
			end = true
			return nil
		}
		got.WriteString(tok)
		return nil
	}, nil)
	require.NoError(t, err)
	assert.True(t, end)
	assert.Equal(t, "A:hi.", got.String())
}

func TestMock_CtxCancel(t *testing.T) {
	m := &Mock{PerTokenDelay: 100 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := m.Chat(ctx, []Message{{Role: RoleUser, Content: "x"}}, nil, func(string, bool) error { return nil }, nil)
	assert.Error(t, err)
}

func TestBuildSystemPrompt_Order(t *testing.T) {
	sys := BuildSystemPrompt(&PersonaInputs{
		Soul:     "S",
		Identity: "I",
		Agent:    "A",
		User:     "U",
		Facts: []Fact{
			{Date: "2026-07-16", Content: "F1"},
		},
		MemorySnippets: []Snippet{
			{Date: "2026-06-04", Content: "M1", Score: 0.9},
		},
		Tools: []ToolDef{{Name: "t1", Description: "d1"}},
	})
	// 顺序：S < I < A < U < Facts < M < tools
	assert.True(t, strings.Index(sys, "S") < strings.Index(sys, "I"))
	assert.True(t, strings.Index(sys, "I") < strings.Index(sys, "A"))
	assert.True(t, strings.Index(sys, "A") < strings.Index(sys, "U"))
	assert.True(t, strings.Index(sys, "U") < strings.Index(sys, "F1"))
	assert.True(t, strings.Index(sys, "F1") < strings.Index(sys, "M1"))
	assert.True(t, strings.Index(sys, "M1") < strings.Index(sys, "t1"))
	assert.Contains(t, sys, "## 已知事实")
}

func TestBuildSystemPrompt_NoMemoryNoTools(t *testing.T) {
	sys := BuildSystemPrompt(&PersonaInputs{
		Soul:     "S",
		Identity: "I",
		Agent:    "A",
		User:     "U",
	})
	assert.NotContains(t, sys, "已知事实")
	assert.NotContains(t, sys, "历史记忆片段")
	assert.NotContains(t, sys, "可用技能")
}
