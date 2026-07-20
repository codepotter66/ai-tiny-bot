// Package ws 定义 WebSocket 双向消息协议。
//
// 消息一律是 JSON 文本帧。type 字段决定含义。
package ws

// 消息类型常量
const (
	// 客户端 → 服务端
	MsgHello     = "hello"     // 客户端首次连接，发 device_id + token
	MsgAudio     = "audio"     // 上行音频 chunk（base64 PCM 16k16 mono）
	MsgEnd       = "end"       // 一句话结束，触发 ASR
	MsgPing      = "ping"      // 客户端 keepalive
	MsgInterrupt = "interrupt" // 打断当前播放

	// 服务端 → 客户端
	MsgHelloOK = "hello.ok" // 鉴权通过，返回 sample_rate 等
	MsgSTT     = "stt"      // ASR 结果（text 字段是识别文本）
	MsgText    = "text"     // LLM 中间文本片段（debug 模式）
	MsgAudioDn = "audio"    // 下行 TTS chunk（base64 PCM）
	MsgTool    = "tool"     // 工具调用（tool + args 字段）
	MsgDone    = "done"     // turn 结束
	MsgError   = "error"    // 错误（error 字段是 code:message）
	MsgPong    = "pong"     // keepalive
)

// ClientMsg 客户端发来的消息。
type ClientMsg struct {
	Type     string `json:"type"`
	DeviceID string `json:"device_id,omitempty"`
	Token    string `json:"token,omitempty"`
	Proto    int    `json:"proto,omitempty"`
	Seq      uint32 `json:"seq,omitempty"`
	Data     string `json:"data,omitempty"`    // base64 PCM
	TSMs     int64  `json:"ts_ms,omitempty"`   // 客户端时间戳
}

// ServerMsg 服务端返回的消息。
type ServerMsg struct {
	Type       string `json:"type"`
	Seq        uint32 `json:"seq,omitempty"`
	Data       string `json:"data,omitempty"`   // base64 PCM（16k16 mono）
	Text       string `json:"text,omitempty"`   // STT / text 类型
	Lang       string `json:"lang,omitempty"`   // stt 类型：检测到的语种 zh/yue/en
	Tool       string `json:"tool,omitempty"`   // tool 类型：skill name
	Args       string `json:"args,omitempty"`   // tool 类型：JSON args
	Err        string `json:"error,omitempty"`  // error 类型
	SampleRate int    `json:"sample_rate,omitempty"` // hello.ok
	Proto      int    `json:"proto,omitempty"`
}

// Error 错误码常量（写到 ServerMsg.Err 前缀）。
const (
	ErrAuth     = "AUTH_FAIL"
	ErrProto    = "BAD_PROTO"
	ErrRate     = "RATE_LIMIT"
	ErrSTT      = "STT_FAIL"
	ErrLLM      = "LLM_FAIL"
	ErrTTS      = "TTS_FAIL"
	ErrInternal = "INTERNAL"
	ErrTaken    = "TAKEN_OVER"
)
