// Package main 是 fake-device：模拟 ESP32 设备跑一轮 turn。
//
// 流程：
//  1. POST /provision?device_id=…&code=… 拿 token
//  2. 打开 ws://host/ws
//  3. 发 hello（带 device_id + token）
//  4. 等 hello.ok
//  5. 发一段"伪 PCM"音频（实际是固定字节）
//  6. 发 end
//  7. 打印收到的所有消息（stt / text / audio / done / error）
//
// 用法：
//
//	./bin/fake-device -device-id tinypal-01 -code ca40-d744 -text "你好"
//
// Phase 7：先把"hello → end → done"链路跑通；不真传音频字节。
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/ws"
)

func main() {
	var (
		host     = flag.String("host", "http://localhost:5678", "server base URL")
		wsHost   = flag.String("ws", "ws://localhost:5678/ws", "websocket URL")
		deviceID = flag.String("device-id", "tinypal-01", "device id")
		code     = flag.String("code", "", "pairing code (empty = skip provision)")
		token    = flag.String("token", "", "pre-existing long-lived token (if you already provisioned)")
		proto    = flag.Int("proto", 1, "protocol version")
		verbose  = flag.Bool("v", true, "verbose logging")
	)
	flag.Parse()

	logging := newLogger(*verbose)
	logging("fake-device starting; set TB_FAKE_ASR_TEXT on the server to control what the mock ASR returns")

	// 1) provision（如果需要）
	tok := *token
	if tok == "" && *code != "" {
		tok = provision(*host, *deviceID, *code, logging)
		if tok == "" {
			os.Exit(1)
		}
		logging("got token, length=%d", len(tok))
	}

	// 2) WebSocket 连接
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, *wsHost, nil)
	if err != nil {
		logging("dial: %v", err)
		os.Exit(1)
	}
	defer conn.Close(websocket.StatusNormalClosure, "bye")

	// 3) hello
	hello := ws.ClientMsg{
		Type:     ws.MsgHello,
		DeviceID: *deviceID,
		Token:    tok,
		Proto:    *proto,
	}
	if err := wsjson.Write(ctx, conn, hello); err != nil {
		logging("write hello: %v", err)
		os.Exit(1)
	}
	logging("sent hello (device_id=%s, proto=%d)", *deviceID, *proto)

	// 4) 收 hello.ok
	var helloOK ws.ServerMsg
	if err := wsjson.Read(ctx, conn, &helloOK); err != nil {
		logging("read hello.ok: %v", err)
		os.Exit(1)
	}
	if helloOK.Type != ws.MsgHelloOK {
		logging("expected hello.ok, got %s err=%s", helloOK.Type, helloOK.Err)
		os.Exit(1)
	}
	logging("got hello.ok (sample_rate=%d)", helloOK.SampleRate)

	// 5) 发一段"假 PCM"（320ms 静音 = 16000 * 0.32 * 2 = 10240 字节）
	pcm := make([]byte, 10240)
	if err := sendAudio(ctx, conn, pcm); err != nil {
		logging("send audio: %v", err)
		os.Exit(1)
	}
	logging("sent 320ms audio + end")

	// 6) 收消息直到 done
	for {
		var m ws.ServerMsg
		if err := wsjson.Read(ctx, conn, &m); err != nil {
			if err == io.EOF {
				logging("connection closed")
				return
			}
			logging("read: %v", err)
			os.Exit(1)
		}
		switch m.Type {
		case ws.MsgSTT:
			logging("STT: %s", m.Text)
		case ws.MsgText:
			logging("TEXT: %s", m.Text)
		case ws.MsgAudioDn:
			logging("AUDIO chunk: %d base64 bytes (seq=%d)", len(m.Data), m.Seq)
		case ws.MsgTool:
			logging("TOOL: %s args=%s", m.Tool, m.Args)
		case ws.MsgDone:
			logging("DONE — turn complete")
			return
		case ws.MsgError:
			logging("ERROR: %s", m.Err)
		default:
			logging("UNKNOWN: %+v", m)
		}
	}
}

func sendAudio(ctx context.Context, conn *websocket.Conn, pcm []byte) error {
	// 分 5 块发，每块 2048 字节
	chunk := 2048
	for i := 0; i < len(pcm); i += chunk {
		end := i + chunk
		if end > len(pcm) {
			end = len(pcm)
		}
		msg := ws.ClientMsg{
			Type: ws.MsgAudio,
			Data: base64.StdEncoding.EncodeToString(pcm[i:end]),
			Seq:  uint32(i / chunk),
		}
		if err := wsjson.Write(ctx, conn, msg); err != nil {
			return err
		}
	}
	return wsjson.Write(ctx, conn, ws.ClientMsg{Type: ws.MsgEnd})
}

func provision(baseURL, deviceID, code string, log func(string, ...any)) string {
	u := fmt.Sprintf("%s/provision?device_id=%s&code=%s",
		baseURL, url.QueryEscape(deviceID), url.QueryEscape(code))
	resp, err := http.Post(u, "application/json", nil)
	if err != nil {
		log("provision post: %v", err)
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		log("provision status %d: %s", resp.StatusCode, string(body))
		return ""
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		log("provision decode: %v", err)
		return ""
	}
	return out.Token
}

func newLogger(verbose bool) func(string, ...any) {
	if !verbose {
		return func(string, ...any) {}
	}
	return func(format string, args ...any) {
		slog.Info(strings.TrimRight(fmt.Sprintf(format, args...), "\n"))
	}
}
