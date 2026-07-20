// list-voices 调用 MiniMax /v1/get_voice，列出当前账号可用的音色 ID。
//
// 用法：
//
//	go run ./cmd/list-voices
//	go run ./cmd/list-voices -type system
//	go run ./cmd/list-voices -q 管家
//	go run ./cmd/list-voices -q Reliable -type system
//	go run ./cmd/list-voices -json
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/config"
)

const defaultBaseURL = "https://api.minimaxi.com"

type getVoiceReq struct {
	VoiceType string `json:"voice_type"`
}

type baseResp struct {
	StatusCode int64  `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

type systemVoice struct {
	VoiceID     string   `json:"voice_id"`
	VoiceName   string   `json:"voice_name"`
	Description []string `json:"description"`
	CreatedTime string   `json:"created_time"`
}

type customVoice struct {
	VoiceID     string   `json:"voice_id"`
	Description []string `json:"description"`
	CreatedTime string   `json:"created_time"`
}

type getVoiceResp struct {
	SystemVoice     []systemVoice `json:"system_voice"`
	VoiceCloning    []customVoice `json:"voice_cloning"`
	VoiceGeneration []customVoice `json:"voice_generation"`
	BaseResp        baseResp      `json:"base_resp"`
}

func main() {
	typeFlag := flag.String("type", "all", "音色类型: system | voice_cloning | voice_generation | all")
	query := flag.String("q", "", "按 voice_id / 名称 / 描述子串过滤（不区分大小写）")
	baseURL := flag.String("base-url", "", "API 根地址（默认 https://api.minimaxi.com）")
	dumpJSON := flag.Bool("json", false, "输出原始 JSON")
	flag.Parse()

	switch *typeFlag {
	case "system", "voice_cloning", "voice_generation", "all":
	default:
		fmt.Fprintf(os.Stderr, "无效 -type %q（需 system|voice_cloning|voice_generation|all）\n", *typeFlag)
		os.Exit(2)
	}

	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	apiKey := cfg.MiniMax.APIKey
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "缺少 API Key：请设置 TB_MINIMAX_API_KEY 或 TB_OPENAI_COMPAT_API_KEY")
		os.Exit(1)
	}

	root := strings.TrimRight(*baseURL, "/")
	if root == "" {
		root = defaultBaseURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, raw, err := fetchVoices(ctx, root, apiKey, *typeFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "get_voice: %v\n", err)
		os.Exit(1)
	}
	if resp.BaseResp.StatusCode != 0 {
		fmt.Fprintf(os.Stderr, "API error: code=%d msg=%s\n", resp.BaseResp.StatusCode, resp.BaseResp.StatusMsg)
		os.Exit(1)
	}

	if *dumpJSON {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, raw, "", "  "); err != nil {
			fmt.Println(string(raw))
		} else {
			fmt.Println(pretty.String())
		}
		return
	}

	q := strings.ToLower(strings.TrimSpace(*query))
	current := map[string]string{
		cfg.MiniMax.Voice:            "TB_MINIMAX_TTS_VOICE",
		cfg.MiniMax.VoiceZh.VoiceID:  "TB_MINIMAX_TTS_VOICE_ZH",
		cfg.MiniMax.VoiceYue.VoiceID: "TB_MINIMAX_TTS_VOICE_YUE",
		cfg.MiniMax.VoiceEn.VoiceID:  "TB_MINIMAX_TTS_VOICE_EN",
	}

	fmt.Printf("type=%s base=%s\n", *typeFlag, root)
	nSys := printSystem(resp.SystemVoice, q, current)
	nClone := printCustom("voice_cloning", resp.VoiceCloning, q, current)
	nGen := printCustom("voice_generation", resp.VoiceGeneration, q, current)
	fmt.Printf("\n总计: system=%d cloning=%d generation=%d", nSys, nClone, nGen)
	if q != "" {
		fmt.Printf("（已按 %q 过滤）", *query)
	}
	fmt.Println()
	fmt.Println("试听可用: go run ./cmd/tts-test -voice '<voice_id>' -text '你好，我是艾希'")
}

func fetchVoices(ctx context.Context, baseURL, apiKey, voiceType string) (*getVoiceResp, []byte, error) {
	body, err := json.Marshal(getVoiceReq{VoiceType: voiceType})
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/get_voice", bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer httpResp.Body.Close()
	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, nil, err
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, raw, fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, truncate(string(raw), 400))
	}
	var parsed getVoiceResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, raw, fmt.Errorf("decode: %w; body=%s", err, truncate(string(raw), 400))
	}
	return &parsed, raw, nil
}

func printSystem(list []systemVoice, q string, current map[string]string) int {
	fmt.Printf("\n## system (%d)\n", len(list))
	n := 0
	for _, v := range list {
		desc := strings.Join(v.Description, "；")
		if !matchFilter(q, v.VoiceID, v.VoiceName, desc) {
			continue
		}
		n++
		mark := currentMark(v.VoiceID, current)
		fmt.Printf("  %-48s  %s%s\n", v.VoiceID, v.VoiceName, mark)
		if desc != "" {
			fmt.Printf("      %s\n", desc)
		}
	}
	if n == 0 {
		fmt.Println("  （无匹配）")
	}
	return n
}

func printCustom(title string, list []customVoice, q string, current map[string]string) int {
	fmt.Printf("\n## %s (%d)\n", title, len(list))
	n := 0
	for _, v := range list {
		desc := strings.Join(v.Description, "；")
		if !matchFilter(q, v.VoiceID, desc) {
			continue
		}
		n++
		mark := currentMark(v.VoiceID, current)
		created := v.CreatedTime
		if created == "" {
			created = "-"
		}
		fmt.Printf("  %-48s  created=%s%s\n", v.VoiceID, created, mark)
		if desc != "" {
			fmt.Printf("      %s\n", desc)
		}
	}
	if n == 0 {
		fmt.Println("  （无匹配）")
	}
	return n
}

func matchFilter(q string, fields ...string) bool {
	if q == "" {
		return true
	}
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), q) {
			return true
		}
	}
	return false
}

func currentMark(voiceID string, current map[string]string) string {
	if voiceID == "" {
		return ""
	}
	if env, ok := current[voiceID]; ok && env != "" {
		return "  ← 当前 " + env
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
