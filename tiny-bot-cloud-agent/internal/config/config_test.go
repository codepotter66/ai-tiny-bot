package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_DefaultsWhenNoFileNoEnv(t *testing.T) {
	cwd, _ := os.Getwd()
	tmp := t.TempDir()
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// 清掉可能影响默认值的 env
	for _, k := range []string{
		"TB_AGENT_ADDR", "TB_WS_PATH",
		"TB_PROVIDER_LLM", "TB_PROVIDER_ASR", "TB_PROVIDER_TTS",
		"TB_OPENAI_COMPAT_BASE_URL", "TB_OPENAI_COMPAT_API_KEY", "TB_OPENAI_COMPAT_MODEL",
		"TB_LOG_LEVEL", "TB_LOG_FORMAT",
		"TB_DB_PATH", "TB_WORKSPACE_ROOT",
		"TB_AGENT_MAX_RESPONSE_CHARS", "TB_AGENT_HISTORY_TURNS",
		"TB_AGENT_MEMORY_RECALL_K", "TB_AGENT_MEMORY_LOOKBACK_DAYS", "TB_AGENT_DAILY_TOKEN_CAP",
		"TB_AGENT_FACTS_MAX", "TB_AGENT_TURN_TIMEOUT", "TB_AGENT_MAX_TOOL_ROUNDS",
		"TB_CODE_RUN_TIMEOUT", "TB_CODE_PYTHON_BIN",
		"TB_PPROF_ENABLED", "TB_HEALTHZ_ENABLED",
		"TB_MAX_FRAME_BYTES", "TB_WS_PING_INTERVAL", "TB_WS_IDLE_TIMEOUT",
	} {
		_ = os.Unsetenv(k)
	}

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, ":5678", cfg.Server.Addr)
	assert.Equal(t, "mock", cfg.Providers.LLM)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, 120*time.Second, cfg.Agent.TurnTimeout)
	assert.Equal(t, 8, cfg.Agent.MaxToolRounds)
	assert.Equal(t, 15*time.Second, cfg.Agent.CodeRunTimeout)
	assert.Equal(t, "python3", cfg.Agent.CodePythonBin)
}

func TestLoad_EnvOverridesDefaults(t *testing.T) {
	cwd, _ := os.Getwd()
	tmp := t.TempDir()
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// 清掉可能影响默认值的 env
	for _, k := range []string{
		"TB_AGENT_ADDR", "TB_LOG_LEVEL", "TB_OPENAI_COMPAT_BASE_URL",
	} {
		_ = os.Unsetenv(k)
	}

	t.Setenv("TB_AGENT_ADDR", ":7777")
	t.Setenv("TB_LOG_LEVEL", "debug")
	t.Setenv("TB_OPENAI_COMPAT_BASE_URL", "https://env-host")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, ":7777", cfg.Server.Addr)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "https://env-host", cfg.OpenAICompat.BaseURL)
}

func TestLoad_DotEnvLoaded(t *testing.T) {
	cwd, _ := os.Getwd()
	tmp := t.TempDir()
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	// 清掉可能影响默认值的 env
	for _, k := range []string{
		"TB_AGENT_ADDR", "TB_LOG_LEVEL",
	} {
		_ = os.Unsetenv(k)
	}

	require.NoError(t, os.WriteFile(".env", []byte(`
TB_AGENT_ADDR=:8888
TB_LOG_LEVEL=warn
`), 0o600))

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, ":8888", cfg.Server.Addr)
	assert.Equal(t, "warn", cfg.Logging.Level)
}

func TestValidate_MissingOpenAICompatFields(t *testing.T) {
	c := defaults()
	c.Providers.LLM = "openai_compat"
	c.OpenAICompat.BaseURL = "" // 故意缺
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "openai_compat.base_url")
}

func TestValidate_InvalidLevel(t *testing.T) {
	c := defaults()
	c.Logging.Level = "verbose"
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logging.level")
}

func TestServerConfig_Port(t *testing.T) {
	assert.Equal(t, 5678, ServerConfig{Addr: ":5678"}.Port())
	assert.Equal(t, 9090, ServerConfig{Addr: "0.0.0.0:9090"}.Port())
	assert.Equal(t, 0, ServerConfig{Addr: "no-port"}.Port())
}

func TestValidate_Aliyun_MissingKey(t *testing.T) {
	c := defaults()
	c.Providers.ASR = "aliyun"
	c.Providers.TTS = "mock"
	// Key 故意缺
	c.Aliyun.Secret = "sk"
	c.Aliyun.AppKey = "ak"
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "aliyun.key")
}

func TestValidate_Aliyun_MissingAppKey(t *testing.T) {
	c := defaults()
	c.Providers.TTS = "aliyun"
	c.Providers.ASR = "mock"
	c.Aliyun.Key = "ak"
	c.Aliyun.Secret = "sk"
	// AppKey 故意缺
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "aliyun.app_key")
}

func TestValidate_Aliyun_OK(t *testing.T) {
	c := defaults()
	c.Providers.ASR = "aliyun"
	c.Providers.TTS = "aliyun"
	c.Aliyun.Key = "ak"
	c.Aliyun.Secret = "sk"
	c.Aliyun.AppKey = "app"
	err := c.Validate()
	assert.NoError(t, err)
}

func TestValidate_Aliyun_TTSOnly_OK(t *testing.T) {
	c := defaults()
	c.Providers.ASR = "mock"
	c.Providers.TTS = "aliyun"
	c.Aliyun.Key = "ak"
	c.Aliyun.Secret = "sk"
	c.Aliyun.AppKey = "app"
	err := c.Validate()
	assert.NoError(t, err, "只 TTS 用 aliyun 时也应该通过")
}

func TestValidate_UnknownProvider(t *testing.T) {
	c := defaults()
	c.Providers.ASR = "azure"
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "providers.asr")

	c2 := defaults()
	c2.Providers.TTS = "google"
	err2 := c2.Validate()
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "providers.tts")
}

func TestValidate_MiniMax_OK(t *testing.T) {
	c := defaults()
	c.Providers.TTS = "minimax"
	c.MiniMax.APIKey = "sk-cp-test"
	err := c.Validate()
	assert.NoError(t, err)
}

func TestValidate_MiniMax_MissingKey(t *testing.T) {
	c := defaults()
	c.Providers.TTS = "minimax"
	// APIKey 为空
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "minimax.api_key")
}

func TestLoad_MiniMaxEnvOverlay(t *testing.T) {
	t.Setenv("TB_MINIMAX_TTS_MODEL", "speech-2.8-hd")
	t.Setenv("TB_MINIMAX_TTS_VOICE", "female-shaonv")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "speech-2.8-hd", cfg.MiniMax.Model)
	assert.Equal(t, "female-shaonv", cfg.MiniMax.Voice)
	assert.Equal(t, 16000, cfg.MiniMax.SampleRate) // 默认
}

func TestLoad_MiniMax_FallbackFromOpenAIKey(t *testing.T) {
	// 没设 TB_MINIMAX_API_KEY，但设了 TB_OPENAI_COMPAT_API_KEY
	t.Setenv("TB_MINIMAX_API_KEY", "")
	t.Setenv("TB_OPENAI_COMPAT_API_KEY", "sk-shared-key")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "sk-shared-key", cfg.MiniMax.APIKey)
}

func TestLoad_AliyunEnvOverlay(t *testing.T) {
	t.Setenv("TB_ALIYUN_KEY", "ak-from-env")
	t.Setenv("TB_ALIYUN_SECRET", "sk-from-env")
	t.Setenv("TB_ALIYUN_ASR_APP_KEY", "app-from-env")
	t.Setenv("TB_ALIYUN_REGION", "cn-beijing")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "ak-from-env", cfg.Aliyun.Key)
	assert.Equal(t, "sk-from-env", cfg.Aliyun.Secret)
	assert.Equal(t, "app-from-env", cfg.Aliyun.AppKey)
	assert.Equal(t, "cn-beijing", cfg.Aliyun.Region)
}

func TestLoadDotEnv(t *testing.T) {
	cwd, _ := os.Getwd()
	tmp := t.TempDir()
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	envContent := `
# comment line
TB_OPENAI_COMPAT_BASE_URL=https://from-dotenv.example

KEY_WITH_QUOTES="hello world"
SINGLE_QUOTED='hi'
EMPTY=
NO_EQUALS
=NO_KEY
`
	require.NoError(t, os.WriteFile(".env", []byte(envContent), 0o600))

	// 清掉可能残留的
	for _, k := range []string{"TB_OPENAI_COMPAT_BASE_URL", "KEY_WITH_QUOTES", "SINGLE_QUOTED", "EMPTY"} {
		_ = os.Unsetenv(k)
	}

	loadDotEnv()

	assert.Equal(t, "https://from-dotenv.example", os.Getenv("TB_OPENAI_COMPAT_BASE_URL"))
	assert.Equal(t, "hello world", os.Getenv("KEY_WITH_QUOTES"))
	assert.Equal(t, "hi", os.Getenv("SINGLE_QUOTED"))
	assert.Equal(t, "", os.Getenv("EMPTY"))
}

func TestLoadDotEnv_DoesNotOverrideExistingEnv(t *testing.T) {
	cwd, _ := os.Getwd()
	tmp := t.TempDir()
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	require.NoError(t, os.WriteFile(".env", []byte("TB_OPENAI_COMPAT_BASE_URL=https://from-file\n"), 0o600))
	t.Setenv("TB_OPENAI_COMPAT_BASE_URL", "https://already-set")

	loadDotEnv()

	assert.Equal(t, "https://already-set", os.Getenv("TB_OPENAI_COMPAT_BASE_URL"),
		"existing env should NOT be overridden by .env")
}
