package asr

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/aliyunauth"
)

func TestAliyun_Name(t *testing.T) {
	a := NewAliyun(AliyunConfig{Key: "ak", Secret: "sk", AppKey: "app"})
	assert.Equal(t, "aliyun", a.Name())
}

func TestAliyun_MissingCreds(t *testing.T) {
	cases := []AliyunConfig{
		{Secret: "sk", AppKey: "app"},
		{Key: "ak", AppKey: "app"},
		{Key: "ak", Secret: "sk"},
	}
	for _, c := range cases {
		a := NewAliyun(c)
		_, err := a.Transcribe(context.Background(), strings.NewReader("pcm"))
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrNotImplemented)
	}
}

func TestAliyun_HTTPTest(t *testing.T) {
	metaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Token":{"Id":"test-nls-token","ExpireTime":4102444800}}`))
	}))
	defer metaSrv.Close()

	asrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/octet-stream", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-nls-token", r.Header.Get("X-NLS-Token"))
		assert.Equal(t, "/stream/v1/asr", r.URL.Path)
		assert.Equal(t, "pcm", r.URL.Query().Get("format"))
		assert.Equal(t, "16000", r.URL.Query().Get("sample_rate"))
		assert.Equal(t, "test-app-key", r.URL.Query().Get("appkey"))
		assert.Empty(t, r.URL.Query().Get("Signature"))
		assert.Empty(t, r.URL.Query().Get("model"))
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, "fake pcm bytes", string(body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":20000000,"message":"SUCCESS","result":"今天天气怎么样"}`))
	}))
	defer asrSrv.Close()

	tokens := aliyunauth.NewTokenManager(aliyunauth.TokenManagerConfig{
		Key:      "testak",
		Secret:   "testsk",
		Endpoint: metaSrv.URL,
		HTTP:     metaSrv.Client(),
	})

	a := NewAliyun(AliyunConfig{
		Key:    "testak",
		Secret: "testsk",
		AppKey: "test-app-key",
		Host:   asrSrv.URL,
		HTTP:   asrSrv.Client(),
		Tokens: tokens,
	})
	res, err := a.Transcribe(context.Background(), strings.NewReader("fake pcm bytes"))
	require.NoError(t, err)
	assert.Equal(t, "今天天气怎么样", res.Text)
	assert.Equal(t, "zh", res.Language)
}

func TestAliyun_HostDefaultsToHTTPS(t *testing.T) {
	a := NewAliyun(AliyunConfig{
		Key: "k", Secret: "s", AppKey: "a",
		Region: "cn-shanghai",
	})
	assert.Equal(t, "https://nls-gateway-cn-shanghai.aliyuncs.com", a.host())
}

func TestAliyun_HostPreservesScheme(t *testing.T) {
	a := NewAliyun(AliyunConfig{
		Key: "k", Secret: "s", AppKey: "a",
		Host: "http://test:8080",
	})
	assert.Equal(t, "http://test:8080", a.host())
}

func TestAliyun_HostAddsHTTPSIfNoScheme(t *testing.T) {
	a := NewAliyun(AliyunConfig{
		Key: "k", Secret: "s", AppKey: "a",
		Host: "test:8080",
	})
	assert.Equal(t, "https://test:8080", a.host())
}

func TestAliyun_CtxCancel(t *testing.T) {
	metaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Token":{"Id":"tok","ExpireTime":4102444800}}`))
	}))
	defer metaSrv.Close()

	asrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{"status":20000000,"result":"x"}`))
	}))
	defer asrSrv.Close()

	tokens := aliyunauth.NewTokenManager(aliyunauth.TokenManagerConfig{
		Key: "ak", Secret: "sk", Endpoint: metaSrv.URL, HTTP: metaSrv.Client(),
	})
	a := NewAliyun(AliyunConfig{
		Key: "ak", Secret: "sk", AppKey: "app",
		Host: asrSrv.URL, HTTP: asrSrv.Client(), Tokens: tokens,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := a.Transcribe(ctx, strings.NewReader("pcm"))
	assert.Error(t, err)
}

func TestAliyun_EmptyPCM(t *testing.T) {
	metaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("empty pcm should not fetch token")
	}))
	defer metaSrv.Close()

	a := NewAliyun(AliyunConfig{
		Key: "ak", Secret: "sk", AppKey: "app",
		Tokens: aliyunauth.NewTokenManager(aliyunauth.TokenManagerConfig{
			Key: "ak", Secret: "sk", Endpoint: metaSrv.URL, HTTP: metaSrv.Client(),
		}),
	})
	res, err := a.Transcribe(context.Background(), strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, "", res.Text)
}

func TestAliyun_APIErr(t *testing.T) {
	metaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Token":{"Id":"tok","ExpireTime":4102444800}}`))
	}))
	defer metaSrv.Close()

	asrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":40000001,"message":"audio too long"}`))
	}))
	defer asrSrv.Close()

	tokens := aliyunauth.NewTokenManager(aliyunauth.TokenManagerConfig{
		Key: "ak", Secret: "sk", Endpoint: metaSrv.URL, HTTP: metaSrv.Client(),
	})
	a := NewAliyun(AliyunConfig{
		Key: "ak", Secret: "sk", AppKey: "app",
		Host: asrSrv.URL, HTTP: asrSrv.Client(), Tokens: tokens,
	})
	_, err := a.Transcribe(context.Background(), strings.NewReader("pcm"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "audio too long")
}

func TestAliyun_HTTPStatusNot2xx(t *testing.T) {
	metaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Token":{"Id":"tok","ExpireTime":4102444800}}`))
	}))
	defer metaSrv.Close()

	asrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`internal error`))
	}))
	defer asrSrv.Close()

	tokens := aliyunauth.NewTokenManager(aliyunauth.TokenManagerConfig{
		Key: "ak", Secret: "sk", Endpoint: metaSrv.URL, HTTP: metaSrv.Client(),
	})
	a := NewAliyun(AliyunConfig{
		Key: "ak", Secret: "sk", AppKey: "app",
		Host: asrSrv.URL, HTTP: asrSrv.Client(), Tokens: tokens,
	})
	_, err := a.Transcribe(context.Background(), strings.NewReader("pcm"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestAliyun_ParseError(t *testing.T) {
	metaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Token":{"Id":"tok","ExpireTime":4102444800}}`))
	}))
	defer metaSrv.Close()

	asrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer asrSrv.Close()

	tokens := aliyunauth.NewTokenManager(aliyunauth.TokenManagerConfig{
		Key: "ak", Secret: "sk", Endpoint: metaSrv.URL, HTTP: metaSrv.Client(),
	})
	a := NewAliyun(AliyunConfig{
		Key: "ak", Secret: "sk", AppKey: "app",
		Host: asrSrv.URL, HTTP: asrSrv.Client(), Tokens: tokens,
	})
	_, err := a.Transcribe(context.Background(), strings.NewReader("pcm"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

func TestAliyun_TokenError(t *testing.T) {
	metaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"Code":"InvalidAccessKeyId.NotFound"}`))
	}))
	defer metaSrv.Close()

	tokens := aliyunauth.NewTokenManager(aliyunauth.TokenManagerConfig{
		Key: "ak", Secret: "sk", Endpoint: metaSrv.URL, HTTP: metaSrv.Client(),
	})
	a := NewAliyun(AliyunConfig{
		Key: "ak", Secret: "sk", AppKey: "app",
		Tokens: tokens,
	})
	_, err := a.Transcribe(context.Background(), strings.NewReader("pcm"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token")
}
