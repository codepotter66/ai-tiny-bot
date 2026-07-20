package ws

import (
	"fmt"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

// Upgrade 把 HTTP 请求升级为 WebSocket，返回 *websocket.Conn。
//
// 限制：
//   - 最大消息 1 MiB（音频 chunk 上限）
//   - coder/websocket 用 ctx 控制超时；ping 由库自动处理
func Upgrade(w http.ResponseWriter, r *http.Request, _ time.Duration) (*websocket.Conn, error) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// 不限制 origin（设备可能是局域网直连）
		InsecureSkipVerify: true,
	})
	if err != nil {
		return nil, fmt.Errorf("ws accept: %w", err)
	}
	conn.SetReadLimit(1 << 20) // 1 MiB
	return conn, nil
}
