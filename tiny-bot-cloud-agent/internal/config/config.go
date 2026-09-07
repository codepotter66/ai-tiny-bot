// Package config 加载 .env + 进程 env vars。
//
// 优先级（高 → 低）：
//  1. 进程 env vars（CI / systemd EnvironmentFile 走这条）
//  2. ./.env（docker-compose env_file 走这条；Go 启动时也会自加载）
//  3. 代码内 defaults（兜底）
//
// 设计原则：
//   - 单一配置源：.env 一份文件包含全部配置
//   - 密钥一律走 .env（.env 已在 .gitignore）
//   - 校验严格：必填字段缺失、未知 provider、非法端口等都会让进程退出码非 0
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/lang"
)

// Config 总体配置。
type Config struct {
	Server       ServerConfig
	Storage      StorageConfig
	Providers    ProvidersConfig
	OpenAICompat OpenAICompatConfig
	Aliyun       AliyunConfig
	MiniMax      MiniMaxConfig
	Agent        AgentConfig
	Logging      LoggingConfig
}

type ServerConfig struct {
	Addr                 string
	WSPath               string
	PProf                bool
	Healthz              bool
	MaxFrameBytes        int64
	PingInterval         time.Duration
	IdleTimeout          time.Duration
	DemoEnabled          bool   // TB_DEMO_ENABLED，默认 true
	DemoDir              string // TB_DEMO_DIR，空则自动探测 ./demo 或 /opt/tiny-bot/demo
	WorkspaceEditorToken string // TB_WORKSPACE_EDITOR_TOKEN，空则禁用 /api/workspace/*
}

type StorageConfig struct {
	DBPath        string 
	WorkspaceRoot string 
}

type ProvidersConfig struct {
	ASR string // mock | aliyun
	LLM string // mock | openai_compat
	TTS string // mock | aliyun | minimax
}

type OpenAICompatConfig struct {
	BaseURL   string
	APIKey    string
	Model     string
	MaxTokens int
}

// AliyunConfig 阿里云智能语音交互凭证。
// 凭证获取：aliyun.com → 智能语音交互 → 创建项目 → 拿 AccessKey + 项目 AppKey。
type AliyunConfig struct {
	Key       string // TB_ALIYUN_KEY   (AccessKeyId)
	Secret    string // TB_ALIYUN_SECRET (AccessKeySecret)
	AppKey    string // TB_ALIYUN_ASR_APP_KEY
	TTSAppKey string // TB_ALIYUN_TTS_APP_KEY（可选，aliyun TTS 专用）
	Region    string // TB_ALIYUN_REGION，默认 cn-shanghai
}

// VoiceProfile 是某语种对应的 MiniMax TTS 音色与 language_boost。
type VoiceProfile struct {
	VoiceID       string // MiniMax voice_id
	LanguageBoost string // Chinese | Chinese,Yue | English | auto
}

// MiniMaxConfig MiniMax 流式 TTS 配置（Bearer Token 鉴权）。
// 通常用同一个 LLM 的 API key；音色和模型按需切换。
type MiniMaxConfig struct {
	APIKey     string        // TB_MINIMAX_API_KEY（与 TB_OPENAI_COMPAT_API_KEY 共用）
	Model      string        // TB_MINIMAX_TTS_MODEL，默认 speech-2.8-turbo
	Voice      string        // TB_MINIMAX_TTS_VOICE，默认 male-qn-qingse（普通话 fallback）
	Format     string        // TB_MINIMAX_TTS_FORMAT，默认 pcm
	SampleRate int           // TB_MINIMAX_TTS_SAMPLE_RATE，默认 16000
	WSURL      string        // 可选：测试时覆盖
	Timeout    time.Duration // 单次连接超时，默认 15s
	VoiceZh    VoiceProfile  // 普通话音色
	VoiceYue   VoiceProfile  // 粤语音色
	VoiceEn    VoiceProfile  // 英语音色
}

type AgentConfig struct {
	MaxResponseChars       int
	HistoryTurns           int
	MemoryRecallK          int
	MemoryLookbackDays     int
	FactsMax               int
	DailyTokenCapPerDevice int
	TurnTimeout            time.Duration // 单轮 turn 超时，默认 120s
	MaxToolRounds          int           // tool 循环上限，默认 8
	CodeRunTimeout         time.Duration // code.run 超时，默认 15s
	CodePythonBin          string        // python 解释器，默认 python3
}

type LoggingConfig struct {
	Level  string 
	Format string 
}

// Load 加载配置。参数已废弃（保留仅为兼容旧 main.go 的 `-config` flag）。
// 真实路径：defaults → .env（cwd）→ 进程 env → Validate。
func Load(_ string) (*Config, error) {
	// 1) 先加载 .env
	loadDotEnv()

	// 2) defaults + 进程 env
	cfg := defaults()
	overlayEnv(cfg)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Addr:          ":5678",
			WSPath:        "/ws",
			PProf:         false,
			Healthz:       true,
			MaxFrameBytes: 1 << 20,
			PingInterval:  20 * time.Second,
			IdleTimeout:   60 * time.Second,
			DemoEnabled:   true,
		},
		Storage: StorageConfig{
			DBPath:        "./data/tiny-bot.db",
			WorkspaceRoot: "./workspace",
		},
		Providers: ProvidersConfig{
			ASR: "mock",
			LLM: "mock",
			TTS: "mock",
		},
		OpenAICompat: OpenAICompatConfig{
			MaxTokens: 512,
		},
		Aliyun: AliyunConfig{
			Region: "cn-shanghai",
		},
		MiniMax: MiniMaxConfig{
			Model:      "speech-2.8-turbo",
			Voice:      "male-qn-qingse",
			Format:     "pcm",
			SampleRate: 16000,
			Timeout:    15 * time.Second,
			VoiceZh: VoiceProfile{
				VoiceID:       "Chinese (Mandarin)_Lyrical_Voice",
				LanguageBoost: "Chinese",
			},
			VoiceYue: VoiceProfile{
				VoiceID:       "Cantonese_GentleLady",
				LanguageBoost: "Chinese,Yue",
			},
			VoiceEn: VoiceProfile{
				VoiceID:       "English_Graceful_Lady",
				LanguageBoost: "English",
			},
		},
		Agent: AgentConfig{
			MaxResponseChars:       600,
			HistoryTurns:           20,
			MemoryRecallK:          5,
			MemoryLookbackDays:     7,
			FactsMax:               80,
			DailyTokenCapPerDevice: 200_000,
			TurnTimeout:            120 * time.Second,
			MaxToolRounds:          8,
			CodeRunTimeout:         15 * time.Second,
			CodePythonBin:          "python3",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

// overlayEnv 用 TB_* 环境变量覆盖默认值。
// .env 已被 loadDotEnv 注入到进程 env，所以这里读到的就是 .env + shell env 的并集。
func overlayEnv(c *Config) {
	if v := os.Getenv("TB_AGENT_ADDR"); v != "" {
		c.Server.Addr = v
	}
	if v := os.Getenv("TB_WS_PATH"); v != "" {
		c.Server.WSPath = v
	}
	if v := os.Getenv("TB_PPROF_ENABLED"); v != "" {
		c.Server.PProf = parseBool(v)
	}
	if v := os.Getenv("TB_HEALTHZ_ENABLED"); v != "" {
		c.Server.Healthz = parseBool(v)
	}
	if v := os.Getenv("TB_MAX_FRAME_BYTES"); v != "" {
		if n, err := parseInt64(v); err == nil {
			c.Server.MaxFrameBytes = n
		}
	}
	if v := os.Getenv("TB_WS_PING_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Server.PingInterval = d
		}
	}
	if v := os.Getenv("TB_WS_IDLE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Server.IdleTimeout = d
		}
	}
	if v := os.Getenv("TB_DEMO_ENABLED"); v != "" {
		c.Server.DemoEnabled = parseBool(v)
	}
	if v := os.Getenv("TB_DEMO_DIR"); v != "" {
		c.Server.DemoDir = v
	}
	if v := os.Getenv("TB_WORKSPACE_EDITOR_TOKEN"); v != "" {
		c.Server.WorkspaceEditorToken = v
	}

	if v := os.Getenv("TB_DB_PATH"); v != "" {
		c.Storage.DBPath = v
	}
	if v := os.Getenv("TB_WORKSPACE_ROOT"); v != "" {
		c.Storage.WorkspaceRoot = v
	}

	if v := os.Getenv("TB_PROVIDER_ASR"); v != "" {
		c.Providers.ASR = v
	}
	if v := os.Getenv("TB_PROVIDER_LLM"); v != "" {
		c.Providers.LLM = v
	}
	if v := os.Getenv("TB_PROVIDER_TTS"); v != "" {
		c.Providers.TTS = v
	}

	if v := os.Getenv("TB_OPENAI_COMPAT_BASE_URL"); v != "" {
		c.OpenAICompat.BaseURL = v
	}
	if v := os.Getenv("TB_OPENAI_COMPAT_API_KEY"); v != "" {
		c.OpenAICompat.APIKey = v
	}
	if v := os.Getenv("TB_OPENAI_COMPAT_MODEL"); v != "" {
		c.OpenAICompat.Model = v
	}
	if v := os.Getenv("TB_OPENAI_COMPAT_MAX_TOKENS"); v != "" {
		if n, err := parseInt(v); err == nil && n > 0 {
			c.OpenAICompat.MaxTokens = n
		}
	}

	// Aliyun（ASR + TTS 共用凭证）
	if v := os.Getenv("TB_ALIYUN_KEY"); v != "" {
		c.Aliyun.Key = v
	}
	if v := os.Getenv("TB_ALIYUN_SECRET"); v != "" {
		c.Aliyun.Secret = v
	}
	if v := os.Getenv("TB_ALIYUN_ASR_APP_KEY"); v != "" {
		c.Aliyun.AppKey = v
	}
	if v := os.Getenv("TB_ALIYUN_TTS_APP_KEY"); v != "" {
		c.Aliyun.TTSAppKey = v
	}
	if v := os.Getenv("TB_ALIYUN_REGION"); v != "" {
		c.Aliyun.Region = v
	}

	// MiniMax TTS（Bearer Token 鉴权）
	if v := os.Getenv("TB_MINIMAX_API_KEY"); v != "" {
		c.MiniMax.APIKey = v
	}
	// v1.1.1 便捷：LLM 配了 KEY 但 TTS 没配，自动填同一个
	if c.MiniMax.APIKey == "" {
		if v := os.Getenv("TB_OPENAI_COMPAT_API_KEY"); v != "" {
			c.MiniMax.APIKey = v
		}
	}
	if v := os.Getenv("TB_MINIMAX_TTS_MODEL"); v != "" {
		c.MiniMax.Model = v
	}
	if v := os.Getenv("TB_MINIMAX_TTS_VOICE"); v != "" {
		c.MiniMax.Voice = v
		if c.MiniMax.VoiceZh.VoiceID == "" || c.MiniMax.VoiceZh.VoiceID == "Chinese (Mandarin)_Lyrical_Voice" {
			c.MiniMax.VoiceZh.VoiceID = v
		}
	}
	if v := os.Getenv("TB_MINIMAX_TTS_VOICE_ZH"); v != "" {
		c.MiniMax.VoiceZh.VoiceID = v
	}
	if v := os.Getenv("TB_MINIMAX_TTS_LANG_BOOST_ZH"); v != "" {
		c.MiniMax.VoiceZh.LanguageBoost = v
	}
	if v := os.Getenv("TB_MINIMAX_TTS_VOICE_YUE"); v != "" {
		c.MiniMax.VoiceYue.VoiceID = v
	}
	if v := os.Getenv("TB_MINIMAX_TTS_LANG_BOOST_YUE"); v != "" {
		c.MiniMax.VoiceYue.LanguageBoost = v
	}
	if v := os.Getenv("TB_MINIMAX_TTS_VOICE_EN"); v != "" {
		c.MiniMax.VoiceEn.VoiceID = v
	}
	if v := os.Getenv("TB_MINIMAX_TTS_LANG_BOOST_EN"); v != "" {
		c.MiniMax.VoiceEn.LanguageBoost = v
	}
	if v := os.Getenv("TB_MINIMAX_TTS_FORMAT"); v != "" {
		c.MiniMax.Format = v
	}
	if v := os.Getenv("TB_MINIMAX_TTS_SAMPLE_RATE"); v != "" {
		if n, err := parseInt(v); err == nil {
			c.MiniMax.SampleRate = n
		}
	}
	if v := os.Getenv("TB_MINIMAX_TTS_WS_URL"); v != "" {
		c.MiniMax.WSURL = v
	}

	if v := os.Getenv("TB_AGENT_MAX_RESPONSE_CHARS"); v != "" {
		if n, err := parseInt(v); err == nil {
			c.Agent.MaxResponseChars = n
		}
	}
	if v := os.Getenv("TB_AGENT_HISTORY_TURNS"); v != "" {
		if n, err := parseInt(v); err == nil {
			c.Agent.HistoryTurns = n
		}
	}
	if v := os.Getenv("TB_AGENT_MEMORY_RECALL_K"); v != "" {
		if n, err := parseInt(v); err == nil {
			c.Agent.MemoryRecallK = n
		}
	}
	if v := os.Getenv("TB_AGENT_MEMORY_LOOKBACK_DAYS"); v != "" {
		if n, err := parseInt(v); err == nil {
			c.Agent.MemoryLookbackDays = n
		}
	}
	if v := os.Getenv("TB_AGENT_FACTS_MAX"); v != "" {
		if n, err := parseInt(v); err == nil {
			c.Agent.FactsMax = n
		}
	}
	if v := os.Getenv("TB_AGENT_DAILY_TOKEN_CAP"); v != "" {
		if n, err := parseInt(v); err == nil {
			c.Agent.DailyTokenCapPerDevice = n
		}
	}
	if v := os.Getenv("TB_AGENT_TURN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Agent.TurnTimeout = d
		}
	}
	if v := os.Getenv("TB_AGENT_MAX_TOOL_ROUNDS"); v != "" {
		if n, err := parseInt(v); err == nil {
			c.Agent.MaxToolRounds = n
		}
	}
	if v := os.Getenv("TB_CODE_RUN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			c.Agent.CodeRunTimeout = d
		}
	}
	if v := os.Getenv("TB_CODE_PYTHON_BIN"); v != "" {
		c.Agent.CodePythonBin = v
	}

	if v := os.Getenv("TB_LOG_LEVEL"); v != "" {
		c.Logging.Level = v
	}
	if v := os.Getenv("TB_LOG_FORMAT"); v != "" {
		c.Logging.Format = v
	}
}

func parseBool(s string) bool {
	switch strings.ToLower(s) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func parseInt64(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// Validate 校验配置合法性。错误立即让进程退出。
func (c *Config) Validate() error {
	var errs []string

	if !strings.HasPrefix(c.Server.Addr, ":") && !strings.Contains(c.Server.Addr, ":") {
		errs = append(errs, "server.addr must include port, e.g. :5678 or 0.0.0.0:5678")
	}
	if c.Server.MaxFrameBytes <= 0 {
		errs = append(errs, "server.max_frame_bytes must be > 0")
	}
	if c.Server.PingInterval <= 0 {
		errs = append(errs, "server.ping_interval must be > 0")
	}
	if c.Server.IdleTimeout <= 0 {
		errs = append(errs, "server.idle_timeout must be > 0")
	}

	if c.Storage.DBPath == "" {
		errs = append(errs, "storage.db_path required")
	}
	if c.Storage.WorkspaceRoot == "" {
		errs = append(errs, "storage.workspace_root required")
	}

	for slot, name := range map[string]string{
		"asr": c.Providers.ASR,
		"llm": c.Providers.LLM,
		"tts": c.Providers.TTS,
	} {
		if name == "" {
			errs = append(errs, fmt.Sprintf("providers.%s empty", slot))
		}
	}
	if c.Providers.LLM != "mock" && c.Providers.LLM != "openai_compat" {
		errs = append(errs, fmt.Sprintf("providers.llm=%q not supported (use mock or openai_compat)", c.Providers.LLM))
	}
	if c.Providers.ASR != "mock" && c.Providers.ASR != "aliyun" {
		errs = append(errs, fmt.Sprintf("providers.asr=%q not supported (use mock or aliyun)", c.Providers.ASR))
	}
	if c.Providers.TTS != "mock" && c.Providers.TTS != "aliyun" && c.Providers.TTS != "minimax" {
		errs = append(errs, fmt.Sprintf("providers.tts=%q not supported in v1.1 (use mock, aliyun, or minimax)", c.Providers.TTS))
	}
	if c.Providers.TTS == "minimax" && c.MiniMax.APIKey == "" {
		errs = append(errs, "minimax.api_key required when providers.tts=minimax (set TB_MINIMAX_API_KEY or TB_OPENAI_COMPAT_API_KEY)")
	}
	if c.Providers.LLM == "openai_compat" {
		if c.OpenAICompat.BaseURL == "" {
			errs = append(errs, "openai_compat.base_url required when providers.llm=openai_compat (set TB_OPENAI_COMPAT_BASE_URL)")
		}
		if c.OpenAICompat.APIKey == "" {
			errs = append(errs, "openai_compat.api_key required (set TB_OPENAI_COMPAT_API_KEY)")
		}
		if c.OpenAICompat.Model == "" {
			errs = append(errs, "openai_compat.model required when providers.llm=openai_compat (set TB_OPENAI_COMPAT_MODEL)")
		}
	}
	// Aliyun 凭证必填检查（ASR 或 TTS 任一用 aliyun 就要凭证完整）
	if c.Providers.ASR == "aliyun" || c.Providers.TTS == "aliyun" {
		if c.Aliyun.Key == "" {
			errs = append(errs, "aliyun.key required when providers.asr/tts=aliyun (set TB_ALIYUN_KEY)")
		}
		if c.Aliyun.Secret == "" {
			errs = append(errs, "aliyun.secret required (set TB_ALIYUN_SECRET)")
		}
		if c.Aliyun.AppKey == "" {
			errs = append(errs, "aliyun.app_key required (set TB_ALIYUN_ASR_APP_KEY)")
		}
	}

	switch c.Logging.Level {
	case "debug", "info", "warn", "error":
	case "":
		errs = append(errs, "logging.level required (debug|info|warn|error)")
	default:
		errs = append(errs, fmt.Sprintf("logging.level=%q invalid", c.Logging.Level))
	}
	if c.Logging.Format != "json" && c.Logging.Format != "text" {
		errs = append(errs, fmt.Sprintf("logging.format=%q invalid (json|text)", c.Logging.Format))
	}

	if c.Agent.MaxResponseChars <= 0 {
		errs = append(errs, "agent.max_response_chars must be > 0")
	}
	if c.Agent.HistoryTurns <= 0 {
		errs = append(errs, "agent.history_turns must be > 0")
	}
	if c.Agent.MemoryRecallK <= 0 {
		errs = append(errs, "agent.memory_recall_k must be > 0")
	}
	if c.Agent.MemoryLookbackDays <= 0 {
		errs = append(errs, "agent.memory_lookback_days must be > 0")
	}
	if c.Agent.FactsMax <= 0 {
		errs = append(errs, "agent.facts_max must be > 0")
	}
	if c.Agent.TurnTimeout <= 0 {
		errs = append(errs, "agent.turn_timeout must be > 0")
	}
	if c.Agent.MaxToolRounds <= 0 {
		errs = append(errs, "agent.max_tool_rounds must be > 0")
	}
	if c.Agent.CodeRunTimeout <= 0 {
		errs = append(errs, "agent.code_run_timeout must be > 0")
	}
	if strings.TrimSpace(c.Agent.CodePythonBin) == "" {
		errs = append(errs, "agent.code_python_bin required")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config invalid:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// SafeView 返回不包含任何 secret 的配置视图（给 demo / debug API 用）。
// 字段对应 .env 里的同名变量；secret（API key、SK、AppKey、Token 等）一律不返回。
func (c *Config) SafeView() map[string]any {
	return map[string]any{
		"server": map[string]any{
			"addr":                       c.Server.Addr,
			"ws_path":                    c.Server.WSPath,
			"max_frame_bytes":            c.Server.MaxFrameBytes,
			"ping_interval":              c.Server.PingInterval.String(),
			"idle_timeout":               c.Server.IdleTimeout.String(),
			"demo_enabled":               c.Server.DemoEnabled,
			"demo_dir":                   c.Server.ResolveDemoDir(),
			"workspace_editor_enabled":   c.Server.WorkspaceEditorToken != "",
		},
		"storage": map[string]any{
			"db_path":        c.Storage.DBPath,
			"workspace_root": c.Storage.WorkspaceRoot,
		},
		"providers": map[string]any{
			"asr": c.Providers.ASR,
			"llm": c.Providers.LLM,
			"tts": c.Providers.TTS,
		},
		"openai_compat": map[string]any{
			"base_url":    c.OpenAICompat.BaseURL,
			"model":       c.OpenAICompat.Model,
			"max_tokens":  c.OpenAICompat.MaxTokens,
			// APIKey 故意省略
		},
		"aliyun": map[string]any{
			"region":   c.Aliyun.Region,
			"has_key":     c.Aliyun.Key != "",
			"has_secret":  c.Aliyun.Secret != "",
			"has_appkey":  c.Aliyun.AppKey != "",
			// Key/Secret/AppKey 故意省略
		},
		"minimax": map[string]any{
			"model":       c.MiniMax.Model,
			"voice":       c.MiniMax.Voice,
			"format":      c.MiniMax.Format,
			"sample_rate": c.MiniMax.SampleRate,
			"has_api_key": c.MiniMax.APIKey != "",
			"voices": map[string]any{
				"zh": map[string]any{
					"voice_id":       c.MiniMax.VoiceProfileFor(lang.Zh).VoiceID,
					"language_boost": c.MiniMax.VoiceProfileFor(lang.Zh).LanguageBoost,
				},
				"yue": map[string]any{
					"voice_id":       c.MiniMax.VoiceProfileFor(lang.Yue).VoiceID,
					"language_boost": c.MiniMax.VoiceProfileFor(lang.Yue).LanguageBoost,
				},
				"en": map[string]any{
					"voice_id":       c.MiniMax.VoiceProfileFor(lang.En).VoiceID,
					"language_boost": c.MiniMax.VoiceProfileFor(lang.En).LanguageBoost,
				},
			},
			// APIKey 故意省略
		},
		"agent": map[string]any{
			"max_response_chars":   c.Agent.MaxResponseChars,
			"history_turns":        c.Agent.HistoryTurns,
			"memory_recall_k":      c.Agent.MemoryRecallK,
			"memory_lookback_days": c.Agent.MemoryLookbackDays,
			"facts_max":            c.Agent.FactsMax,
			"daily_token_cap":      c.Agent.DailyTokenCapPerDevice,
			"turn_timeout":         c.Agent.TurnTimeout.String(),
			"max_tool_rounds":      c.Agent.MaxToolRounds,
			"code_run_timeout":     c.Agent.CodeRunTimeout.String(),
			"code_python_bin":      c.Agent.CodePythonBin,
		},
		"logging": map[string]any{
			"level":  c.Logging.Level,
			"format": c.Logging.Format,
		},
	}
}

// VoiceProfileFor 返回语种对应的音色配置；VoiceID 为空时 fallback 到全局 Voice。
func (m MiniMaxConfig) VoiceProfileFor(code lang.Code) VoiceProfile {
	var p VoiceProfile
	switch code {
	case lang.Yue:
		p = m.VoiceYue
	case lang.En:
		p = m.VoiceEn
	default:
		p = m.VoiceZh
	}
	if p.VoiceID == "" {
		p.VoiceID = m.Voice
	}
	return p
}

// ResolveDemoDir 返回可用的 demo 静态目录；不存在时返回空字符串。
func (s ServerConfig) ResolveDemoDir() string {
	if !s.DemoEnabled {
		return ""
	}
	candidates := []string{}
	if s.DemoDir != "" {
		candidates = append(candidates, s.DemoDir)
	}
	candidates = append(candidates, "./demo", "/opt/tiny-bot/demo")
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	return ""
}

// Port 从 ":5678" / "0.0.0.0:5678" 提取端口号。
func (s ServerConfig) Port() int {
	idx := strings.LastIndex(s.Addr, ":")
	if idx < 0 {
		return 0
	}
	var p int
	_, _ = fmt.Sscanf(s.Addr[idx+1:], "%d", &p)
	return p
}

// loadDotEnv 从 cwd 加载 .env；不存在不报错。
// 行为：KEY=value 一行；# 开头是注释；空行跳过；引号自动去掉；
//       已存在的 env 变量不会被覆盖（避免被 shell 注入的同名变量劫持）。
func loadDotEnv() {
	paths := []string{".env", "./.env"}
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			eq := strings.IndexByte(line, '=')
			if eq <= 0 {
				continue
			}
			k := strings.TrimSpace(line[:eq])
			v := strings.TrimSpace(line[eq+1:])
			// 去掉可选的引号
			if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
				v = v[1 : len(v)-1]
			} else if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
				v = v[1 : len(v)-1]
			}
			// 已有则不覆盖
			if _, exists := os.LookupEnv(k); !exists {
				_ = os.Setenv(k, v)
			}
		}
		// 只读第一个存在的 .env
		return
	}
	// 静默忽略 .env 不存在的情况
	_ = errors.New("")
}
