// asr-test 直连阿里云 ASR，用于本地调试鉴权与识别。
//
// 用法：
//
//	go run ./cmd/asr-test -file testdata/nls-sample-16k.pcm
//	TB_LOG_LEVEL=debug go run ./cmd/asr-test -file ./my-audio.pcm
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/aliyunauth"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/asr"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/config"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/logging"
)

func main() {
	file := flag.String("file", "testdata/nls-sample-16k.pcm", "PCM 16kHz/16bit/mono 文件路径")
	flag.Parse()

	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	logging.Setup(cfg.Logging.Level, cfg.Logging.Format)

	if cfg.Aliyun.Key == "" || cfg.Aliyun.Secret == "" || cfg.Aliyun.AppKey == "" {
		fmt.Fprintln(os.Stderr, "缺少阿里云凭证：TB_ALIYUN_KEY / TB_ALIYUN_SECRET / TB_ALIYUN_ASR_APP_KEY")
		os.Exit(1)
	}

	pcm, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read pcm: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("loaded pcm: %d bytes from %s\n", len(pcm), *file)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tokens := aliyunauth.NewTokenManager(aliyunauth.TokenManagerConfig{
		Key:    cfg.Aliyun.Key,
		Secret: cfg.Aliyun.Secret,
		Region: cfg.Aliyun.Region,
	})
	fmt.Println("step 1: fetching CreateToken...")
	token, err := tokens.GetToken(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CreateToken failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("CreateToken ok: token_len=%d\n", len(token))

	client := asr.NewAliyun(asr.AliyunConfig{
		Key:    cfg.Aliyun.Key,
		Secret: cfg.Aliyun.Secret,
		AppKey: cfg.Aliyun.AppKey,
		Region: cfg.Aliyun.Region,
		Tokens: tokens,
	})
	fmt.Println("step 2: calling ASR...")
	res, err := client.Transcribe(ctx, bytes.NewReader(pcm))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ASR failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("ASR ok: text=%q lang=%s duration_ms=%d\n", res.Text, res.Language, res.Duration)
}
