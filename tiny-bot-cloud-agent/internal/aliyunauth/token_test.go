package aliyunauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTokenQuery(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	q := CreateTokenQuery("cn-shanghai", "my-ak", now, "nonce-123")
	assert.Equal(t, "my-ak", q["AccessKeyId"])
	assert.Equal(t, "CreateToken", q["Action"])
	assert.Equal(t, "JSON", q["Format"])
	assert.Equal(t, "cn-shanghai", q["RegionId"])
	assert.Equal(t, "HMAC-SHA1", q["SignatureMethod"])
	assert.Equal(t, "nonce-123", q["SignatureNonce"])
	assert.Equal(t, "1.0", q["SignatureVersion"])
	assert.Equal(t, "2024-01-01T00:00:00Z", q["Timestamp"])
	assert.Equal(t, createTokenVersion, q["Version"])
}

func TestTokenManager_GetToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/", r.URL.Path)
		assert.NotEmpty(t, r.URL.Query().Get("Signature"))
		assert.Equal(t, "CreateToken", r.URL.Query().Get("Action"))
		assert.Equal(t, "test-ak", r.URL.Query().Get("AccessKeyId"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Token":{"Id":"tok-abc","ExpireTime":4102444800}}`))
	}))
	defer srv.Close()

	m := NewTokenManager(TokenManagerConfig{
		Key:      "test-ak",
		Secret:   "test-sk",
		Region:   "cn-shanghai",
		Endpoint: srv.URL,
		HTTP:     srv.Client(),
	})

	tok, err := m.GetToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "tok-abc", tok)

	// 缓存复用：不应再次请求
	tok2, err := m.GetToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "tok-abc", tok2)
}

func TestTokenManager_GetToken_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"Code":"InvalidAccessKeyId.NotFound","Message":"bad key"}`))
	}))
	defer srv.Close()

	m := NewTokenManager(TokenManagerConfig{
		Key:      "bad-ak",
		Secret:   "bad-sk",
		Endpoint: srv.URL,
		HTTP:     srv.Client(),
	})
	_, err := m.GetToken(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestTokenManager_GetToken_EmptyToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Token":{"Id":"","ExpireTime":0},"ErrMsg":"fail"}`))
	}))
	defer srv.Close()

	m := NewTokenManager(TokenManagerConfig{
		Key:      "ak",
		Secret:   "sk",
		Endpoint: srv.URL,
		HTTP:     srv.Client(),
	})
	_, err := m.GetToken(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty token")
}

func TestMetaEndpoint(t *testing.T) {
	assert.Equal(t, "https://nls-meta.cn-shanghai.aliyuncs.com", MetaEndpoint("cn-shanghai"))
}

func TestSign_CreateTokenPOP(t *testing.T) {
	now := time.Date(2019, 4, 18, 8, 32, 31, 0, time.UTC)
	params := CreateTokenQuery("cn-shanghai", "my_access_key_id", now, "b924c8c3-6d03-4c5d-ad36-d984d3116788")
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	u, err := Sign("http://nls-meta.cn-shanghai.aliyuncs.com", "/", "GET", q, "my_access_key_id", "my_access_key_secret")
	require.NoError(t, err)
	assert.Contains(t, u, "Action=CreateToken")
	assert.Contains(t, u, "Signature=")
	parsed, err := url.Parse(u)
	require.NoError(t, err)
	assert.Equal(t, "CreateToken", parsed.Query().Get("Action"))
	assert.NotEmpty(t, parsed.Query().Get("Signature"))
	assert.True(t, strings.HasPrefix(parsed.Host, "nls-meta.cn-shanghai.aliyuncs.com") || parsed.Host == "")
}
