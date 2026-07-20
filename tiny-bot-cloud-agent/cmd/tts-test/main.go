// tts-test 直连 MiniMax TTS，用于本地调试语音合成效果。
//
// 用法：
//
//	go run ./cmd/tts-test
//	go run ./cmd/tts-test -text "小主人在吗？想聊什么都可以告诉我呀"
//	go run ./cmd/tts-test -file /tmp/clean_reply.txt -out testdata/tts-out.pcm
//	go run ./cmd/tts-test -llm -prompt "你好"
//	go run ./cmd/tts-test -model speech-2.8-hd -voice female-shaonv
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/audio"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/config"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/logging"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/tts"
)

const defaultText = "小主人在吗？想聊什么都可以告诉我呀"

func main() {
	textFlag := flag.String("text", "", "要合成的文本")
	fileFlag := flag.String("file", "", "从文件读取文本")
	outFlag := flag.String("out", "testdata/tts-out.pcm", "输出 PCM 路径")
	writeWAV := flag.Bool("wav", true, "同时写出可播放的 WAV（默认与 PCM 同名 .wav）")
	wavFlag := flag.String("wav-out", "", "WAV 输出路径（默认由 -out 改扩展名）")
	useLLM := flag.Bool("llm", false, "先调 LLM（经 ThinkingFilter）再合成")
	prompt := flag.String("prompt", "你好", "与 -llm 联用时发给模型的用户输入")
	modelFlag := flag.String("model", "", "覆盖 TB_MINIMAX_TTS_MODEL")
	voiceFlag := flag.String("voice", "", "覆盖 TB_MINIMAX_TTS_VOICE")
	flag.Parse()

	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	if *modelFlag != "" {
		cfg.MiniMax.Model = *modelFlag
	}
	if *voiceFlag != "" {
		cfg.MiniMax.Voice = *voiceFlag
	}
	logging.Setup(cfg.Logging.Level, cfg.Logging.Format)
	fmt.Printf("tts: model=%s voice=%s sample_rate=%d\n",
		cfg.MiniMax.Model, cfg.MiniMax.Voice, cfg.MiniMax.SampleRate)

	text, err := resolveText(*useLLM, *textFlag, *fileFlag, *prompt, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	fmt.Printf("text=%q chars=%d\n", text, len([]rune(text)))

	synth, err := buildTTS(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tts: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	start := time.Now()
	stream, err := synth.Synthesize(ctx, text, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "synthesize: %v\n", err)
		os.Exit(1)
	}
	defer stream.Close()

	if err := os.MkdirAll(filepath.Dir(*outFlag), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	out, err := os.Create(*outFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create out: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	n, err := io.Copy(out, stream)
	if err != nil {
		fmt.Fprintf(os.Stderr, "write pcm: %v\n", err)
		os.Exit(1)
	}

	sampleRate := cfg.MiniMax.SampleRate
	if sampleRate == 0 {
		sampleRate = 16000
	}

	var wavPath string
	if *writeWAV {
		wavPath = *wavFlag
		if wavPath == "" {
			wavPath = strings.TrimSuffix(*outFlag, filepath.Ext(*outFlag)) + ".wav"
		}
		if err := writeWAVFile(wavPath, *outFlag, sampleRate); err != nil {
			fmt.Fprintf(os.Stderr, "write wav: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("TTS ok: bytes=%d elapsed=%s out=%s\n", n, time.Since(start).Round(time.Millisecond), *outFlag)
	if wavPath != "" {
		fmt.Printf("WAV ok: out=%s\n", wavPath)
		fmt.Println("试听（推荐）: afplay " + wavPath)
	}
	fmt.Printf("raw PCM: ffplay -nodisp -autoexit -f s16le -ar %d %s\n", sampleRate, *outFlag)
	fmt.Println("说明: 裸 PCM 无法用系统播放器直接双击打开；请用上面的 WAV 或 ffplay 命令。")
}

func resolveText(useLLM bool, text, file, prompt string, cfg *config.Config) (string, error) {
	if useLLM {
		return llmVisibleText(prompt, cfg)
	}
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read file: %w", err)
		}
		return string(b), nil
	}
	if text != "" {
		return text, nil
	}
	return defaultText, nil
}

func llmVisibleText(prompt string, cfg *config.Config) (string, error) {
	if cfg.OpenAICompat.APIKey == "" || cfg.OpenAICompat.BaseURL == "" || cfg.OpenAICompat.Model == "" {
		return "", fmt.Errorf("缺少 LLM 配置：TB_OPENAI_COMPAT_API_KEY / BASE_URL / MODEL")
	}
	client := llm.NewOpenAICompat(llm.OpenAICompatConfig{
		BaseURL: cfg.OpenAICompat.BaseURL,
		APIKey:  cfg.OpenAICompat.APIKey,
		Model:   cfg.OpenAICompat.Model,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	filter := llm.NewStreamThinkingFilter()
	var visible string
	fmt.Printf("llm: model=%s prompt=%q\n", cfg.OpenAICompat.Model, prompt)
	err := client.Chat(ctx, []llm.Message{
		{Role: llm.RoleUser, Content: prompt},
	}, nil, func(tok string, end bool) error {
		visible += filter.Feed(tok)
		if end {
			visible += filter.Flush()
		}
		return nil
	}, nil)
	if err != nil {
		return "", fmt.Errorf("llm: %w", err)
	}
	if visible == "" {
		return "", fmt.Errorf("llm 返回空可见文本")
	}
	fmt.Printf("llm visible: %q\n", visible)
	return visible, nil
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

func writeWAVFile(wavPath, pcmPath string, sampleRate int) error {
	pcm, err := os.ReadFile(pcmPath)
	if err != nil {
		return fmt.Errorf("read pcm: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(wavPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(wavPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := audio.WriteWAV(f, pcm, sampleRate, 1, 16); err != nil {
		return err
	}
	return nil
}
