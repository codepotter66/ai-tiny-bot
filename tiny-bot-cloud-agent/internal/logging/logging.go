// Package logging 设置 log/slog，输出 JSON（生产）或 text（开发）。
//
// 用法：
//
//	import "github.com/wisdomoasis/tiny-bot-cloud-agent/internal/logging"
//
//	func main() {
//	    logging.Setup("info", "json")
//	    slog.Info("started", "addr", ":5678")
//	}
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Setup 根据 level/format 设置默认 logger。
//   - level: debug | info | warn | error
//   - format: json | text
//
// 重复调用安全：后调用覆盖前一次。
func Setup(level, format string) {
	SetupWithWriter(os.Stdout, level, format)
}

// SetupWithWriter 同 Setup，但写入指定 writer（用于测试时捕获日志）。
func SetupWithWriter(w io.Writer, level, format string) {
	lvl := parseLevel(level)
	opts := &slog.HandlerOptions{Level: lvl}

	var h slog.Handler
	switch strings.ToLower(format) {
	case "text":
		h = slog.NewTextHandler(w, opts)
	default:
		h = slog.NewJSONHandler(w, opts)
	}
	slog.SetDefault(slog.New(h))
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ctxKey 私有类型，避免与其他包冲突。
type ctxKey struct{}

// WithDevice 把"带 device_id 字段"的 logger 挂到 ctx 上。
// 下游用 FromContext 取回。
func WithDevice(ctx context.Context, deviceID string) context.Context {
	l := slog.Default().With("device_id", deviceID)
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext 取出 ctx 上的 logger；若没有则返回 slog.Default()。
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
