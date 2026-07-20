package integration

import "os"

// readFileOS 是 os.ReadFile 的薄包装（便于必要时 stub）。
func readFileOS(p string) ([]byte, error) { return os.ReadFile(p) }
