package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// Result skill 执行结果。
type Result struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Duration int64  `json:"duration_ms"`
}

// DefaultTimeout 单次 skill 执行超时。
const DefaultTimeout = 5 * time.Second

// MaxStdoutBytes stdout 截断上限。
const MaxStdoutBytes = 8 * 1024

// Run 在 skill 目录里跑脚本，argsJSON 写到 stdin。
// stdout/stderr 截断到 MaxStdoutBytes。
func Run(ctx context.Context, s *Skill, argsJSON string) (*Result, error) {
	timeout := DefaultTimeout
	if dl, ok := ctx.Deadline(); ok {
		// 优先用 ctx 自己的 deadline，但留出 100ms 给收尾
		if rem := time.Until(dl) - 100*time.Millisecond; rem < timeout {
			timeout = rem
		}
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, s.ScriptPath())
	cmd.Dir = s.Dir

	var stdin bytes.Buffer
	stdin.WriteString(argsJSON)
	cmd.Stdin = &stdin

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	dur := time.Since(start).Milliseconds()

	res := &Result{
		Stdout:   truncate(stdout.String(), MaxStdoutBytes),
		Stderr:   truncate(stderr.String(), MaxStdoutBytes),
		Duration: dur,
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		res.ExitCode = exitErr.ExitCode()
	} else if err == nil {
		res.ExitCode = 0
	} else {
		return res, fmt.Errorf("run skill %s: %w", s.Name, err)
	}
	return res, nil
}

// truncate 简单按字节截断。
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// EncodeArgs 把任意 args map 序列化为 JSON 字符串喂给 stdin。
func EncodeArgs(args map[string]interface{}) (string, error) {
	b, err := json.Marshal(args)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
