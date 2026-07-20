package agent

import (
	"testing"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/config"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/skills"
)

func testAgentCfg(maxChars int) config.AgentConfig {
	if maxChars <= 0 {
		maxChars = 600
	}
	return config.AgentConfig{
		MaxResponseChars:       maxChars,
		HistoryTurns:           20,
		MemoryRecallK:          5,
		MemoryLookbackDays:     7,
		FactsMax:               80,
		DailyTokenCapPerDevice: 200_000,
	}
}

func mustSkillsReloader(t *testing.T, workspaceRoot string) *skills.Reloader {
	t.Helper()
	sk, err := skills.NewReloader(workspaceRoot)
	if err != nil {
		t.Fatalf("skills: %v", err)
	}
	return sk
}
