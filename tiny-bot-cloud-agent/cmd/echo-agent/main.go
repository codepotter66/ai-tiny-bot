// Package main 是 echo-agent：把一段文本送进完整 STT→LLM→TTS pipeline 并打印结果。
//
// 用法：
//
//	./bin/echo-agent -text "你好，小陪"        # 用 mock 跑一遍
//	./bin/echo-agent -text "你好" -print-prompt # 顺便打印组合后的 system prompt
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/audio"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/asr"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/persona"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/tts"
)

func main() {
	var (
		text        = flag.String("text", "今天天气怎么样", "input text to feed ASR (mock will echo it)")
		printPrompt = flag.Bool("print-prompt", false, "print composed system prompt and exit")
	)
	flag.Parse()

	p, err := persona.Load("./workspace")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load persona: %v\n", err)
		os.Exit(1)
	}

	if *printPrompt {
		sys := llm.BuildSystemPrompt(&llm.PersonaInputs{
			Soul:     p.Soul,
			Identity: p.Identity,
			Agent:    p.Agent,
			User:     p.User,
		})
		fmt.Println(sys)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 1) ASR（mock）：把 -text 灌到一个 PCM 流里"录"过去
	asrImpl := asr.NewMock(*text)
	pcmIn := bytes.NewReader([]byte("fake-pcm")) // mock 不真正解析字节
	st, err := asrImpl.Transcribe(ctx, pcmIn)
	if err != nil {
		slog.Error("asr", "err", err)
		os.Exit(1)
	}
	slog.Info("asr done", "text", st.Text, "lang", st.Language)

	// 2) 拼消息
	llmImpl := llm.NewMock()
	sys := llm.BuildSystemPrompt(&llm.PersonaInputs{
		Soul:     p.Soul,
		Identity: p.Identity,
		Agent:    p.Agent,
		User:     p.User,
	})
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: sys},
		{Role: llm.RoleUser, Content: st.Text},
	}

	// 3) LLM 流式输出 → 句子聚合（单 worker 串行 TTS，避免 chunk 交错）
	ttsImpl := tts.NewMock()
	var fullText strings.Builder
	ttsQueue := make(chan string, 8)
	var ttsWg sync.WaitGroup
	ttsWg.Add(1)
	go func() {
		defer ttsWg.Done()
		for text := range ttsQueue {
			stream, err := ttsImpl.Synthesize(ctx, text, nil)
			if err != nil {
				slog.Error("tts", "err", err)
				continue
			}
			n, _ := io.Copy(io.Discard, stream)
			stream.Close()
			slog.Info("tts chunk", "text", text, "pcm_bytes", n)
		}
	}()
	agg := audio.NewAggregator(80, 4, func(sent string) {
		fullText.WriteString(sent)
		fullText.WriteString("\n")
		slog.Info("sentence", "text", sent)
		ttsQueue <- sent
	})

	if err := llmImpl.Chat(ctx, msgs, nil, func(tok string, end bool) error {
		if end {
			agg.Flush()
			return nil
		}
		agg.Push(tok)
		return nil
	}, nil); err != nil {
		slog.Error("llm", "err", err)
		os.Exit(1)
	}

	close(ttsQueue)
	ttsWg.Wait()
	slog.Info("done", "full_text", fullText.String())
}
