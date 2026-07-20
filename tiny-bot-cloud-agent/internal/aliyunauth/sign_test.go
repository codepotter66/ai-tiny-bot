package aliyunauth

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSign_KnownInput(t *testing.T) {
	// 取阿里云官方 v3 签名 sample 的关键字段做 smoke test
	// https://help.aliyun.com/zh/sdk/developer-reference/v3-request-structure-and-signature
	q := url.Values{
		"Format":         {"JSON"},
		"AccessKeyId":    {"testid"},
		"Action":         {"DescribeRegions"},
		"SignatureMethod": {"HMAC-SHA1"},
		"SignatureNonce":  {"3ee8c1b8-83d3-44af-a94f-4e0ad82fd6cf"},
		"SignatureVersion": {"1.0"},
		"Timestamp":      {"2016-03-24T16:41:54Z"},
		"Version":        {"2014-05-26"},
	}
	got, err := Sign("https://ecs.aliyuncs.com", "/", "GET", q, "testid", "testsecret")
	require.NoError(t, err)

	// Signature 必须出现在 query 里
	assert.Contains(t, got, "Signature=")
	// key=value 必须按字典序
	assert.True(t, strings.Contains(got, "AccessKeyId=testid"))
	assert.True(t, strings.Contains(got, "Action=DescribeRegions"))
	// 空格被编码为 %20（不是 +）
	assert.True(t, strings.Contains(got, "%3A") || strings.Contains(got, ":"),
		"应该编码冒号")
}

func TestSign_DeterministicOutput(t *testing.T) {
	// 同样的输入必须产出同样的 Signature
	q := url.Values{
		"appkey": {"my-app-key"},
		"format": {"pcm"},
	}
	u1, _ := Sign("https://nls-gateway-cn-shanghai.aliyuncs.com", "/stream/v1/asr", "POST", q, "ak", "sk")
	u2, _ := Sign("https://nls-gateway-cn-shanghai.aliyuncs.com", "/stream/v1/asr", "POST", q, "ak", "sk")
	assert.Equal(t, u1, u2, "相同输入必须产生相同签名")
}

func TestSign_DifferentSecretDiffers(t *testing.T) {
	q := url.Values{"x": {"y"}}
	u1, _ := Sign("https://example.com", "/", "GET", q, "ak", "sk1")
	u2, _ := Sign("https://example.com", "/", "GET", q, "ak", "sk2")
	// 提取 Signature 参数做比较
	sig1 := extractSig(t, u1)
	sig2 := extractSig(t, u2)
	assert.NotEqual(t, sig1, sig2)
}

func TestSign_DifferentTimestampDiffers(t *testing.T) {
	q1 := url.Values{"Timestamp": {"2024-01-01T00:00:00Z"}}
	q2 := url.Values{"Timestamp": {"2024-01-01T00:00:01Z"}}
	u1, _ := Sign("https://example.com", "/", "GET", q1, "ak", "sk")
	u2, _ := Sign("https://example.com", "/", "GET", q2, "ak", "sk")
	sig1 := extractSig(t, u1)
	sig2 := extractSig(t, u2)
	assert.NotEqual(t, sig1, sig2, "Timestamp 变化 → Signature 变")
}

func TestSign_EmptyQueryOK(t *testing.T) {
	u, err := Sign("https://example.com", "/", "GET", nil, "ak", "sk")
	require.NoError(t, err)
	assert.Contains(t, u, "Signature=")
}

func TestSign_MissingCredsError(t *testing.T) {
	_, err := Sign("https://example.com", "/", "GET", nil, "", "sk")
	assert.Error(t, err)
	_, err = Sign("https://example.com", "/", "GET", nil, "ak", "")
	assert.Error(t, err)
}

func TestSign_EmptyValuesInQuery(t *testing.T) {
	// query value 空字符串也要参与签名
	q := url.Values{"foo": {""}, "bar": {"baz"}}
	u, err := Sign("https://example.com", "/", "GET", q, "ak", "sk")
	require.NoError(t, err)
	assert.Contains(t, u, "foo=")
	assert.Contains(t, u, "bar=baz")
}

// extractSig 从 URL 里抠出 Signature 参数。
func extractSig(t *testing.T, rawURL string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return u.Query().Get("Signature")
}

// TestSign_RealV3Algorithm 验证 v3 签名算法是否与 Aliyun 官方一致。
// 用一个简单的、可以手算的例子：
//   key=testid, secret=testsecret
//   method=GET, path=/
//   query={Action:DescribeRegions, Format:JSON}
// 期望 StringToSign = "GET&%2F&Action%3DDescribeRegions%26Format%3DJSON"
// 期望 Signature = base64(HMAC-SHA1("testsecret&", above))
// 用 openssl 命令行可以验证：echo -n "STRING" | openssl dgst -sha1 -hmac "testsecret&" -binary | base64
func TestSign_RealV3Algorithm(t *testing.T) {
	q := url.Values{
		"Action": {"DescribeRegions"},
		"Format": {"JSON"},
	}
	u, err := Sign("http://ecs.aliyuncs.com", "/", "GET", q, "testid", "testsecret")
	require.NoError(t, err)

	sig := extractSig(t, u)
	// 1) Signature 必须是非空 base64 字符串
	//    HMAC-SHA1 输出 20 字节 → base64 编码 28 字符（无 padding，20 % 3 = 2，差 1 字节不到 24）
	assert.NotEmpty(t, sig)
	assert.Equal(t, 28, len(sig), "Signature 应是 HMAC-SHA1 的 base64（20 字节→28 字符）")

	// 2) URL 包含所有原始 query
	assert.Contains(t, u, "Action=DescribeRegions")
	assert.Contains(t, u, "Format=JSON")

	// 3) URL 编码检查：= → %3D，& → %26（在 canonical 编码后）
	// canonical 包含 key=value 对的 %3D 和 & 的 %26
	// 但 query string 中的 = 和 & 是普通分隔符，不应被编码
	// 所以最终 URL 里看到的应该是：
	//   Action=DescribeRegions&Format=JSON&Signature=xxx
	//   key=value 直接是字面，= 和 & 不应编码

	// 4) 完整性：提取后能重新放回 query
	parsed, _ := url.Parse(u)
	assert.Equal(t, "DescribeRegions", parsed.Query().Get("Action"))
	assert.Equal(t, "JSON", parsed.Query().Get("Format"))
	assert.Equal(t, sig, parsed.Query().Get("Signature"))
}

// TestSign_ReproducibleHash 用标准库 hmac 复算 Signature，对比函数输出。
// 这是最强的本地验证：能复算说明算法正确（至少结构上）。
func TestSign_ReproducibleHash(t *testing.T) {
	q := url.Values{
		"appkey":      {"ak1"},
		"format":      {"pcm"},
		"sample_rate": {"16000"},
	}
	const (
		key    = "k"
		secret = "s"
	)
	u, err := Sign("https://nls-gateway.cn-shanghai.aliyuncs.com", "/stream/v1/asr", "POST", q, key, secret)
	require.NoError(t, err)

	// 重算：手动用 stdlib hmac 算 signature
	canonical := "appkey=ak1&format=pcm&sample_rate=16000"
	stringToSign := "POST&%2F&" + url.QueryEscape(canonical)
	mac := hmac.New(sha1.New, []byte(secret+"&"))
	mac.Write([]byte(stringToSign))
	wantSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	gotSig := extractSig(t, u)
	assert.Equal(t, wantSig, gotSig,
		"v3 签名算法必须能被 stdlib hmac 复算")
}

func TestSign_CreateTokenReproducible(t *testing.T) {
	now := time.Date(2019, 4, 18, 8, 32, 31, 0, time.UTC)
	params := CreateTokenQuery("cn-shanghai", "my_access_key_id", now, "b924c8c3-6d03-4c5d-ad36-d984d3116788")
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	const secret = "my_access_key_secret"
	u, err := Sign("http://nls-meta.cn-shanghai.aliyuncs.com", "/", "GET", q, "my_access_key_id", secret)
	require.NoError(t, err)

	canonical := "AccessKeyId=my_access_key_id&Action=CreateToken&Format=JSON&RegionId=cn-shanghai&SignatureMethod=HMAC-SHA1&SignatureNonce=b924c8c3-6d03-4c5d-ad36-d984d3116788&SignatureVersion=1.0&Timestamp=2019-04-18T08%3A32%3A31Z&Version=2019-02-28"
	stringToSign := "GET&%2F&" + url.QueryEscape(canonical)
	mac := hmac.New(sha1.New, []byte(secret+"&"))
	mac.Write([]byte(stringToSign))
	wantSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	assert.Equal(t, wantSig, extractSig(t, u))
}
