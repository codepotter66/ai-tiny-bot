// Package aliyunauth 提供阿里云 OpenAPI v3 HMAC-SHA1 签名 + NLS Token 管理。
//
// 复用于：
//   - internal/asr/aliyun.go (Paraformer 一句话识别)
//   - internal/tts/aliyun.go (CosyVoice 短文本合成，待迁移 Token 鉴权)
//
// NLS REST 鉴权流程：
//  1. GET nls-meta.{region}.aliyuncs.com CreateToken（AK/SK POP 签名）
//  2. ASR/TTS 请求 Header: X-NLS-Token: <Token.Id>
//
// v3 签名（Sign / canonicalQuery / urlEncode）见 sign.go。
package aliyunauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// Sign 把 query 参数加上 v3 签名 Signature 并返回完整 URL。
//
//   base   — 不含 query 的 endpoint，如 "https://nls-gateway-cn-shanghai.aliyuncs.com"
//   path   — URL path，如 "/stream/v1/asr"
//   method — "GET" 或 "POST"
//   query  — 待签名参数（会被排序、URL 编码、加 Signature）
//   key    — AccessKeyId
//   secret — AccessKeySecret
//
// 返回：完整 URL（含 ?Signature=...）。
func Sign(base, path, method string, query url.Values, key, secret string) (string, error) {
	if key == "" || secret == "" {
		return "", fmt.Errorf("aliyunauth: key/secret required")
	}
	if query == nil {
		query = url.Values{}
	}

	// 1) CanonicalizedQuery：按键名排序，URL 编码 key=value 用 & 连接
	canonical, err := canonicalQuery(query)
	if err != nil {
		return "", fmt.Errorf("aliyunauth: canonicalize: %w", err)
	}

	// 2) StringToSign = METHOD + "&" + urlencoded("/") + "&" + urlencoded(canonical)
	//    注意 "/" 是 path 的 urlencoded；此函数专用于 root path（阿里云 NLS gateway 的 asr/tts 都在根）
	stringToSign := method + "&" + urlEncode("/") + "&" + urlEncode(canonical)

	// 3) Signature = base64(HMAC-SHA1(key + "&", stringToSign))
	mac := hmac.New(sha1.New, []byte(secret+"&"))
	mac.Write([]byte(stringToSign))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// 4) 拼回 URL（深拷贝 query 避免污染入参）
	final := url.Values{}
	for k, vs := range query {
		for _, v := range vs {
			final.Add(k, v)
		}
	}
	final.Set("Signature", sig)
	u := base + path
	encoded := final.Encode()
	if encoded != "" {
		u += "?" + encoded
	}
	return u, nil
}

// RandomToken 生成 32 字节随机字符串（用于一次性 nonce，目前未在 v1.1 使用，
// 但保留给未来加 sts 临时凭证用）。
func RandomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// canonicalQuery 把 query 按 key 排序、URL 编码后用 & 连接。
// 返回值不含前导 "?"。
func canonicalQuery(q url.Values) (string, error) {
	if len(q) == 0 {
		return "", nil
	}
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		// 阿里云 v3 要求：每个 key=value 单独编码后用 = 连接，整体再被 urlEncode 一次
		// 但 url.Values.Encode 已经做了编码，所以这里直接拼接
		b.WriteString(urlEncode(k))
		b.WriteByte('=')
		b.WriteString(urlEncode(q.Get(k)))
	}
	return b.String(), nil
}

// urlEncode 按阿里云 v3 签名规范做 URL 编码：
//   - unreserved 字符（字母数字 + - _ . ~）不编码
//   - 空格编码为 %20（不是 +，跟 Go url.QueryEscape 默认不同）
//   - 其他字符（= & 等）正常 percent-encode
//
// v1.1.18 修复：之前用 url.PathEscape 是错的——Aliyun 规范用的是 QueryEscape 语义
// （= 编码为 %3D、& 编码为 %26），PathEscape 不编码 = 和 & 导致 query value 含 = & 时签名错。
func urlEncode(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}
