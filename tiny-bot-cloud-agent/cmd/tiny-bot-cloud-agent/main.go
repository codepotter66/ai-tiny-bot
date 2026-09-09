// Package main 是 tiny-bot-cloud-agent 主进程入口。
//
//
// 启动顺序：
//  1. 加载 config
//  2. 初始化 logging
//  3. 打开 SQLite
//  4. 加载 persona / memory / skills
//  5. 构造 ASR / LLM / TTS（按 providers 选择 mock 或真实实现）
//  6. 启动 HTTP 服务（含 /ws）
//  7. 监听 SIGHUP → 热重载
//  8. 监听 SIGINT/SIGTERM → 优雅退出
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/agent"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/asr"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/auth"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/config"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/logging"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/memory"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/persona"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/server"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/skills"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/store"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/tts"
)

func main() {
	flag.Parse()

	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(2)
	}
	logging.Setup(cfg.Logging.Level, cfg.Logging.Format)

	slog.Info("tiny-bot-cloud-agent starting",
		"version", "0.1.0",
		"config_source", ".env + env",
		"addr", cfg.Server.Addr,
		"db_path", cfg.Storage.DBPath,
		"workspace_root", cfg.Storage.WorkspaceRoot,
		"providers", map[string]string{
			"asr": cfg.Providers.ASR,
			"llm": cfg.Providers.LLM,
			"tts": cfg.Providers.TTS,
		},
	)

	ctx := context.Background()

	// 1) Store
	st, err := store.Open(ctx, cfg.Storage.DBPath)
	if err != nil {
		slog.Error("open store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	if cfg.Server.DemoEnabled {
		if err := st.EnsureDemoDevice(ctx); err != nil {
			slog.Warn("ensure demo device", "err", err)
		} else {
			slog.Info("demo device ready",
				"device_id", store.DemoDeviceID, "user_id", store.DemoUserID)
		}
	}

	// 2) Auth
	authSvc := auth.New(st)

	// 3) Persona
	ps, err := persona.NewReloader(cfg.Storage.WorkspaceRoot)
	if err != nil {
		slog.Error("load persona", "err", err)
		os.Exit(1)
	}

	// 4) Memory
	mem := memory.NewMarkdownStore(cfg.Storage.WorkspaceRoot)
	mem.SetIndexer(memIndex{st})

	// 5) Skills
	sk, err := skills.NewReloader(cfg.Storage.WorkspaceRoot)
	if err != nil {
		slog.Error("load skills", "err", err)
		os.Exit(1)
	}
	slog.Info("skills loaded", "names", sk.Get().Names())

	// 6) History
	hist := agent.NewHistoryRegistry(cfg.Agent.HistoryTurns)

	// 7) ASR / LLM / TTS（按 cfg.Providers 选 mock 或真实现）
	asrImpl, err := buildASR(cfg)
	if err != nil {
		slog.Error("build asr", "provider", cfg.Providers.ASR, "err", err)
		os.Exit(1)
	}
	llmImpl, err := buildLLM(cfg)
	if err != nil {
		slog.Error("build llm", "provider", cfg.Providers.LLM, "err", err)
		os.Exit(1)
	}
	ttsImpl, err := buildTTS(cfg)
	if err != nil {
		slog.Error("build tts", "provider", cfg.Providers.TTS, "err", err)
		os.Exit(1)
	}
	slog.Info("providers ready",
		"asr", asrImpl.Name(), "llm", llmImpl.Name(), "tts", ttsImpl.Name())

	// 8) Server
	srv := server.New(server.Deps{
		Cfg:     cfg,
		Store:   st,
		Auth:    authSvc,
		Persona: ps,
		Memory:  mem,
		Skills:  sk,
		History: hist,
		ASR:     asrImpl,
		LLM:     llmImpl,
		TTS:     ttsImpl,
	})

	// 9) SIGHUP 热重载（persona / skills）
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	go func() {
		for range sighup {
			if err := ps.Reload(); err != nil {
				slog.Warn("persona reload failed", "err", err)
			} else {
				slog.Info("persona reloaded")
			}
			if err := sk.Reload(); err != nil {
				slog.Warn("skills reload failed", "err", err)
			} else {
				slog.Info("skills reloaded", "names", sk.Get().Names())
			}
		}
	}()

	// 10) 启动 + 优雅退出
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("signal received, shutting down", "sig", sig)
	case err := <-errCh:
		if err != nil {
			slog.Error("server exited", "err", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	slog.Info("bye")
}

// ============================================================
// Provider 构造（按 cfg.Providers 选 mock 或真实现）
// ============================================================

func buildASR(cfg *config.Config) (asr.Transcriber, error) {
	switch cfg.Providers.ASR {
	case "mock", "":
		return asr.NewMock(""), nil
	case "aliyun":
		return asr.NewAliyun(asr.AliyunConfig{
			Key:     cfg.Aliyun.Key,
			Secret:  cfg.Aliyun.Secret,
			AppKey:  cfg.Aliyun.AppKey,
			Region:  cfg.Aliyun.Region,
		}), nil
	default:
		return nil, fmt.Errorf("unknown asr provider %q", cfg.Providers.ASR)
	}
}

func buildLLM(cfg *config.Config) (llm.Chat, error) {
	switch cfg.Providers.LLM {
	case "mock", "":
		return llm.NewMock(), nil
	case "openai_compat":
		return llm.NewOpenAICompat(llm.OpenAICompatConfig{
			BaseURL:   cfg.OpenAICompat.BaseURL,
			APIKey:    cfg.OpenAICompat.APIKey,
			Model:     cfg.OpenAICompat.Model,
			MaxTokens: cfg.OpenAICompat.MaxTokens,
		}), nil
	default:
		return nil, fmt.Errorf("unknown llm provider %q", cfg.Providers.LLM)
	}
}

func buildTTS(cfg *config.Config) (tts.Synthesizer, error) {
	switch cfg.Providers.TTS {
	case "mock", "":
		return tts.NewMock(), nil
	case "aliyun":
		appKey := cfg.Aliyun.AppKey
		if cfg.Aliyun.TTSAppKey != "" {
			appKey = cfg.Aliyun.TTSAppKey
		}
		return tts.NewAliyun(tts.AliyunConfig{
			Key:    cfg.Aliyun.Key,
			Secret: cfg.Aliyun.Secret,
			AppKey: appKey,
			Region: cfg.Aliyun.Region,
		}), nil
	case "minimax":
		return tts.NewMiniMax(tts.MiniMaxConfig{
			APIKey:     cfg.MiniMax.APIKey,
			Model:      cfg.MiniMax.Model,
			Voice:      cfg.MiniMax.Voice,
			Format:     cfg.MiniMax.Format,
			SampleRate: cfg.MiniMax.SampleRate,
			WSURL:      cfg.MiniMax.WSURL,
			Timeout:    cfg.MiniMax.Timeout,
		}), nil
	default:
		return nil, fmt.Errorf("unknown tts provider %q", cfg.Providers.TTS)
	}
}
