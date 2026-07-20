package store

import "os"

// osMkdirAll 是 os.MkdirAll 的薄包装（便于单测 stub）。
func osMkdirAll(dir string) error { return os.MkdirAll(dir, 0o755) }
