package asr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/aliyunauth"
	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/lang"
)

// AliyunConfig 是 aliyun Paraformer 一句话识别所需的运行时参数。
//
// 凭证获取：aliyun.com → 智能语音交互 → 创建项目 → 拿 AccessKey + 项目 AppKey。
// 鉴权：AK/SK 调 CreateToken → ASR 请求 Header X-NLS-Token。
type AliyunConfig struct {
	Key     string // TB_ALIYUN_KEY   (AccessKeyId)
	Secret  string // TB_ALIYUN_SECRET (AccessKeySecret)
	AppKey  string // TB_ALIYUN_ASR_APP_KEY (项目 AppKey)
	Region  string // TB_ALIYUN_REGION，默认 cn-shanghai
	Host    string // 可选：覆盖 ASR host（测试用）
	HTTP    *http.Client  // 可选：测试时注入
	Timeout time.Duration // 单次 HTTP 超时，默认 10s
	Tokens  *aliyunauth.TokenManager // 可选：测试时注入
}

// Aliyun 是 asr.Transcriber 的真实实现。
// 一句话识别 REST：POST 整段 PCM → JSON 响应包含文本。
type Aliyun struct {
	cfg    AliyunConfig
	tokens *aliyunauth.TokenManager
}

// NewAliyun 构造 provider。Region / Timeout 缺失时填默认值。
func NewAliyun(cfg AliyunConfig) *Aliyun {
	if cfg.Region == "" && cfg.Host == "" {
		cfg.Region = "cn-shanghai"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.HTTP == nil {
		cfg.HTTP = &http.Client{Timeout: cfg.Timeout}
	}
	tokens := cfg.Tokens
	if tokens == nil {
		tokens = aliyunauth.NewTokenManager(aliyunauth.TokenManagerConfig{
			Key:    cfg.Key,
			Secret: cfg.Secret,
			Region: cfg.Region,
			HTTP:   cfg.HTTP,
		})
	}
	return &Aliyun{cfg: cfg, tokens: tokens}
}

// host 解析：
//   - cfg.Host 带 scheme (http://x 或 https://x) → 原样用（测试用 http）
//   - cfg.Host 只 host:port → 自动加 https://（生产场景）
//   - 否则从 Region 拼 nls-gateway-{region}.aliyuncs.com（默认 https）
func (a *Aliyun) host() string {
	if a.cfg.Host != "" {
		if strings.HasPrefix(a.cfg.Host, "http://") || strings.HasPrefix(a.cfg.Host, "https://") {
			return a.cfg.Host
		}
		return "https://" + a.cfg.Host
	}
	return "https://" + fmt.Sprintf("nls-gateway-%s.aliyuncs.com", a.cfg.Region)
}

func (a *Aliyun) Name() string { return "aliyun" }

func (a *Aliyun) Transcribe(ctx context.Context, pcm PCMStream) (*Result, error) {
	if a.cfg.Key == "" || a.cfg.Secret == "" || a.cfg.AppKey == "" {
		return nil, fmt.Errorf("%w: aliyun requires KEY/SECRET/APPKEY (set TB_ALIYUN_KEY/TB_ALIYUN_SECRET/TB_ALIYUN_ASR_APP_KEY)", ErrNotImplemented)
	}

	data, err := io.ReadAll(pcm)
	if err != nil {
		return nil, fmt.Errorf("asr aliyun: read pcm: %w", err)
	}
	if cerr := ctx.Err(); cerr != nil {
		return nil, cerr
	}
	if len(data) == 0 {
		return &Result{Text: "", Language: "zh"}, nil
	}

	token, err := a.tokens.GetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("asr aliyun: token: %w", err)
	}

	q := url.Values{
		"appkey":      {a.cfg.AppKey},
		"format":      {"pcm"},
		"sample_rate": {"16000"},
	}
	fullURL := a.host() + "/stream/v1/asr?" + q.Encode()
	slog.Debug("aliyun asr request",
		"url", fullURL,
		"appkey_len", len(a.cfg.AppKey),
		"body_bytes", len(data),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-NLS-Token", token)

	resp, err := a.cfg.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("asr aliyun: http: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("asr aliyun: status=%d body=%s",
			resp.StatusCode, truncate(body, 300))
	}

	var ar struct {
		Header struct {
			Status  int    `json:"status"`
			Message string `json:"message"`
		} `json:"header"`
		Body struct {
			Result string `json:"result"`
		} `json:"body"`
		Status  int    `json:"status"`
		Message string `json:"message"`
		Result  string `json:"result"`
	}
	if err := json.Unmarshal(body, &ar); err != nil {
		return nil, fmt.Errorf("asr aliyun: decode: %w body=%s", err, truncate(body, 300))
	}
	status := ar.Header.Status
	msg := ar.Header.Message
	text := ar.Body.Result
	if status == 0 {
		status = ar.Status
		msg = ar.Message
		text = ar.Result
	}
	if status != 20000000 {
		return nil, fmt.Errorf("asr aliyun: api status=%d msg=%s body=%s",
			status, msg, truncate(body, 300))
	}
	return &Result{
		Text:     text,
		Duration: len(data) * 1000 / (16000 * 2),
		Language: lang.Detect(text).String(),
	}, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
