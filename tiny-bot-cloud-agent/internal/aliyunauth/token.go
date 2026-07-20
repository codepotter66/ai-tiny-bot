// Package aliyunauth 提供阿里云 OpenAPI v3 HMAC-SHA1 签名 + NLS Token 管理。
//
// NLS REST 鉴权流程：
//  1. GET nls-meta.{region}.aliyuncs.com CreateToken（AK/SK POP 签名）
//  2. ASR/TTS 请求 Header: X-NLS-Token: <Token.Id>
//
// v3 签名（Sign / canonicalQuery / urlEncode）见 sign.go。
package aliyunauth

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const createTokenVersion = "2019-02-28"

// Token 缓存项。
type Token struct {
	Value     string
	ExpiresAt time.Time
}

// IsValid 报告 token 还有效。
func (t *Token) IsValid() bool {
	return t != nil && time.Until(t.ExpiresAt) > 5*time.Minute
}

// TokenManagerConfig 构造 TokenManager 的参数。
type TokenManagerConfig struct {
	Key      string // AccessKeyId
	Secret   string // AccessKeySecret
	Region   string // 默认 cn-shanghai
	Endpoint string // 可选：测试时覆盖 meta endpoint（含 scheme）
	HTTP     *http.Client
}

// TokenManager NLS Token 缓存（in-memory + 锁）。
type TokenManager struct {
	key      string
	secret   string
	region   string
	endpoint string
	client   *http.Client

	mu     sync.Mutex
	cached *Token
}

// NewTokenManager 创建 token 管理器。
func NewTokenManager(cfg TokenManagerConfig) *TokenManager {
	region := cfg.Region
	if region == "" {
		region = "cn-shanghai"
	}
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = MetaEndpoint(region)
	}
	client := cfg.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &TokenManager{
		key:      cfg.Key,
		secret:   cfg.Secret,
		region:   region,
		endpoint: endpoint,
		client:   client,
	}
}

// MetaEndpoint 返回 nls-meta POP 服务地址。
func MetaEndpoint(region string) string {
	if region == "" {
		region = "cn-shanghai"
	}
	return "https://nls-meta." + region + ".aliyuncs.com"
}

// CreateTokenQuery 构造 CreateToken POP 请求参数（不含 Signature）。
func CreateTokenQuery(region, accessKeyID string, now time.Time, nonce string) map[string]string {
	if region == "" {
		region = "cn-shanghai"
	}
	return map[string]string{
		"AccessKeyId":      accessKeyID,
		"Action":           "CreateToken",
		"Format":           "JSON",
		"RegionId":         region,
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   nonce,
		"SignatureVersion": "1.0",
		"Timestamp":        now.UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          createTokenVersion,
	}
}

// GetToken 返回有效 token，必要时刷新。
func (m *TokenManager) GetToken(ctx context.Context) (string, error) {
	m.mu.Lock()
	if m.cached != nil && m.cached.IsValid() {
		tok := m.cached.Value
		m.mu.Unlock()
		return tok, nil
	}
	m.mu.Unlock()

	tok, err := m.fetch(ctx)
	if err != nil {
		m.mu.Lock()
		m.cached = nil
		m.mu.Unlock()
		return "", err
	}
	m.mu.Lock()
	m.cached = tok
	m.mu.Unlock()
	return tok.Value, nil
}

func (m *TokenManager) fetch(ctx context.Context) (*Token, error) {
	if m.key == "" || m.secret == "" {
		return nil, fmt.Errorf("aliyunauth: key/secret required")
	}

	nonce, err := signatureNonce()
	if err != nil {
		return nil, fmt.Errorf("aliyunauth: nonce: %w", err)
	}
	params := CreateTokenQuery(m.region, m.key, time.Now(), nonce)
	query := url.Values{}
	for k, v := range params {
		query.Set(k, v)
	}

	fullURL, err := Sign(m.endpoint, "/", http.MethodGet, query, m.key, m.secret)
	if err != nil {
		return nil, fmt.Errorf("aliyunauth: sign: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("aliyunauth: new req: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("aliyunauth: http: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("aliyunauth: status=%d body=%s",
			resp.StatusCode, truncateTokenBody(body, 300))
	}

	var result struct {
		Token struct {
			ID         string `json:"Id"`
			ExpireTime int64  `json:"ExpireTime"`
		} `json:"Token"`
		ErrMsg string `json:"ErrMsg"`
		Code   string `json:"Code"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("aliyunauth: decode: %w body=%s", err, truncateTokenBody(body, 300))
	}
	if result.Token.ID == "" {
		msg := result.ErrMsg
		if msg == "" {
			msg = truncateTokenBody(body, 300)
		}
		return nil, fmt.Errorf("aliyunauth: empty token (code=%s msg=%s)", result.Code, msg)
	}
	return &Token{
		Value:     result.Token.ID,
		ExpiresAt: time.Unix(result.Token.ExpireTime, 0),
	}, nil
}

func signatureNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func truncateTokenBody(b []byte, n int) string {
	if len(b) <= n {
		return strings.TrimSpace(string(b))
	}
	return strings.TrimSpace(string(b[:n])) + "..."
}
