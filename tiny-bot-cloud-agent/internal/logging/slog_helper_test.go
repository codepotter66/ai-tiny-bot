package logging

import "log/slog"

// slogDefault 辅助函数，单测里拿当前默认 logger。
func slogDefault() *slog.Logger { return slog.Default() }
