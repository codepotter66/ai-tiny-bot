// llm-test 直连 MiniMax LLM，用于本地调试模型名与 API Key。
//
// 用法：
//
//	go run ./cmd/llm-test
//	go run ./cmd/llm-test -prompt "你好，请用一句话介绍你自己"
//	go run ./cmd/llm-test -raw   # 同时打印原始流（含 thinking）
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/config"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/logging"
)

func main() {
	prompt := flag.String("prompt", "你好，请用一句话介绍你自己", "用户输入")
	showRaw := flag.Bool("raw", false, "同时打印未过滤的原始流")
	flag.Parse()

	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	logging.Setup(cfg.Logging.Level, cfg.Logging.Format)

	if cfg.OpenAICompat.APIKey == "" || cfg.OpenAICompat.BaseURL == "" || cfg.OpenAICompat.Model == "" {
		fmt.Fprintln(os.Stderr, "缺少 LLM 配置：TB_OPENAI_COMPAT_API_KEY / BASE_URL / MODEL")
		os.Exit(1)
	}

	fmt.Printf("model=%s base=%s\n", cfg.OpenAICompat.Model, cfg.OpenAICompat.BaseURL)

	client := llm.NewOpenAICompat(llm.OpenAICompatConfig{
		BaseURL: cfg.OpenAICompat.BaseURL,
		APIKey:  cfg.OpenAICompat.APIKey,
		Model:   cfg.OpenAICompat.Model,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	filter := llm.NewStreamThinkingFilter()
	var raw, visible string
	fmt.Println("streaming (visible)...")
	err = client.Chat(ctx, []llm.Message{
		{Role: llm.RoleUser, Content: *prompt},
	}, nil, func(tok string, endTurn bool) error {
		raw += tok
		if *showRaw {
			fmt.Fprintf(os.Stderr, "[raw] %s", tok)
		}
		if chunk := filter.Feed(tok); chunk != "" {
			visible += chunk
			fmt.Print(chunk)
		}
		if endTurn {
			if tail := filter.Flush(); tail != "" {
				visible += tail
				fmt.Print(tail)
			}
			fmt.Println()
			if *showRaw {
				fmt.Fprintf(os.Stderr, "\n[raw total] chars=%d\n", len(raw))
			}
		}
		return nil
	}, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nLLM failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nLLM ok: visible_chars=%d raw_chars=%d\n", len(visible), len(raw))
}
