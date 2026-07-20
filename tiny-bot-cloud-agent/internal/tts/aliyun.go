package tts

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/aliyunauth"
)

// AliyunConfig 是 aliyun CosyVoice 短文本语音合成所需的运行时参数。
//
// 凭证获取：aliyun.com → 智能语音交互 → 创建项目 → 拿 AccessKey + 项目 AppKey。
// CosyVoice 音色列表：https://help.aliyun.com/zh/isi/getting-started/voice-list
type AliyunConfig struct {
	Key    string // TB_ALIYUN_KEY   (AccessKeyId)
	Secret string // TB_ALIYUN_SECRET (AccessKeySecret)
	AppKey string // TB_ALIYUN_ASR_APP_KEY (TTS 共用 AppKey)
	Region string // TB_ALIYUN_REGION，默认 cn-shanghai
	Host   string // 可选：覆盖 Region 默认 host（测试时用）
	Voice  string // TB_ALIYUN_TTS_VOICE，默认 zhitian_emo
	Format string // TB_ALIYUN_TTS_FORMAT，默认 pcm
	SampleRate string // TB_ALIYUN_TTS_SAMPLE_RATE，默认 16000
	HTTP   *http.Client  // 可选：测试时注入
	Timeout time.Duration // 单次 HTTP 超时，默认 8s
}

// AliyunTTS 是 tts.Synthesizer 的真实实现。
// CosyVoice REST：POST 文本 + 音色 → 响应 body 是 PCM/WAV 流（不是 JSON）。
type AliyunTTS struct {
	cfg AliyunConfig
}

// NewAliyun 构造 provider。Region / Voice / Format / SampleRate / Timeout 缺失时填默认值。
func NewAliyun(cfg AliyunConfig) *AliyunTTS {
	if cfg.Region == "" && cfg.Host == "" {
		cfg.Region = "cn-shanghai"
	}
	if cfg.Voice == "" {
		cfg.Voice = "zhitian_emo"
	}
	if cfg.Format == "" {
		cfg.Format = "pcm"
	}
	if cfg.SampleRate == "" {
		cfg.SampleRate = "16000"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 8 * time.Second
	}
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: cfg.Timeout}
	}
	return &AliyunTTS{cfg: cfg}
}

func (a *AliyunTTS) Name() string { return "aliyun" }

func (a *AliyunTTS) Synthesize(ctx context.Context, text string, _ *SynthOptions) (PCMStream, error) {
	if a.cfg.Key == "" || a.cfg.Secret == "" || a.cfg.AppKey == "" {
		return nil, fmt.Errorf("%w: aliyun tts requires KEY/SECRET/APPKEY", ErrNotImplemented)
	}
	text = strings.TrimSpace(text)
	text = CleanText(text)
	if text == "" {
		// 空文本：返回 ~300ms 静音（PCM 16k16mono = 32000 字节）
		return io.NopCloser(strings.NewReader(string(make([]byte, 32_000)))), nil
	}

	// 1) 拼 query（v3 签名）
	q := url.Values{
		"appkey":      {a.cfg.AppKey},
		"text":        {text},
		"voice":       {a.cfg.Voice},
		"format":      {a.cfg.Format},
		"sample_rate": {a.cfg.SampleRate},
		"speech_rate": {"0"}, // 默认语速
		"pitch_rate":  {"0"}, // 默认音调
		"volume":      {"50"},
	}
	endpoint := a.host()
	fullURL, err := aliyunauth.Sign(endpoint, "/stream/v1/tts", http.MethodPost, q, a.cfg.Key, a.cfg.Secret)
	if err != nil {
		return nil, fmt.Errorf("tts aliyun: sign: %w", err)
	}

	// 2) POST（body = 空，参数全在 query）
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.cfg.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts aliyun: http: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("tts aliyun: http status=%d body=%s",
			resp.StatusCode, truncateBody(body, 200))
	}
	// 3) resp.Body 直接是 PCM 流，返回给 agent 用 32 KiB chunk Read
	return resp.Body, nil
}

func (a *AliyunTTS) host() string {
	if a.cfg.Host != "" {
		if strings.HasPrefix(a.cfg.Host, "http://") || strings.HasPrefix(a.cfg.Host, "https://") {
			return a.cfg.Host
		}
		return "https://" + a.cfg.Host
	}
	return "https://" + fmt.Sprintf("nls-gateway-%s.aliyuncs.com", a.cfg.Region)
}

func truncateBody(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
