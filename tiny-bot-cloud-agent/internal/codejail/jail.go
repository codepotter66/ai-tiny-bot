// Package codejail 在 workspace/scratch/<device_id>/ 下受限写/跑 Python。
//
// 约束：仅 *.py 文件名（无路径分隔符）、python3 -I、显式 Env（不继承 TB_*）、超时杀掉。
package codejail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// DefaultTimeout 单次运行超时。
	DefaultTimeout = 15 * time.Second
	// MaxFileBytes 单文件写入上限。
	MaxFileBytes = 32 * 1024
	// MaxOutputBytes stdout/stderr 截断上限。
	MaxOutputBytes = 8 * 1024
	// DefaultPython 解释器名。
	DefaultPython = "python3"
)

// ErrBadFilename 文件名非法。
var ErrBadFilename = errors.New("codejail: invalid filename")

// ErrTooLarge 内容超限。
var ErrTooLarge = errors.New("codejail: file too large")

// ErrPythonMissing python3 不可用。
var ErrPythonMissing = errors.New("codejail: python3 not found")

// Jail 设备级 scratch 沙箱。
type Jail struct {
	WorkspaceRoot string
	PythonBin     string
	Timeout       time.Duration
	MaxFileBytes  int
}

// RunResult 执行结果。
type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
}

func (j *Jail) pythonBin() string {
	if j != nil && strings.TrimSpace(j.PythonBin) != "" {
		return j.PythonBin
	}
	return DefaultPython
}

func (j *Jail) timeout() time.Duration {
	if j != nil && j.Timeout > 0 {
		return j.Timeout
	}
	return DefaultTimeout
}

func (j *Jail) maxFile() int {
	if j != nil && j.MaxFileBytes > 0 {
		return j.MaxFileBytes
	}
	return MaxFileBytes
}

func (j *Jail) scratchDir(deviceID string) (string, error) {
	if j == nil || strings.TrimSpace(j.WorkspaceRoot) == "" {
		return "", fmt.Errorf("codejail: workspace root required")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" || strings.Contains(deviceID, "..") || strings.ContainsAny(deviceID, `/\`) {
		return "", fmt.Errorf("codejail: invalid device_id")
	}
	return filepath.Join(j.WorkspaceRoot, "scratch", deviceID), nil
}

// ValidateFilename 只允许简单 *.py 文件名。
func ValidateFilename(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: empty", ErrBadFilename)
	}
	if name != filepath.Base(name) {
		return fmt.Errorf("%w: path components not allowed", ErrBadFilename)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("%w: .. not allowed", ErrBadFilename)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%w: separators not allowed", ErrBadFilename)
	}
	if !strings.HasSuffix(strings.ToLower(name), ".py") {
		return fmt.Errorf("%w: must end with .py", ErrBadFilename)
	}
	if utf8.RuneCountInString(name) > 64 {
		return fmt.Errorf("%w: too long", ErrBadFilename)
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return fmt.Errorf("%w: invalid character", ErrBadFilename)
	}
	return nil
}

// ResolvePath 返回 scratch 内合法绝对路径。
func (j *Jail) ResolvePath(deviceID, filename string) (string, error) {
	if err := ValidateFilename(filename); err != nil {
		return "", err
	}
	dir, err := j.scratchDir(deviceID)
	if err != nil {
		return "", err
	}
	full := filepath.Join(dir, filepath.Base(filename))
	// 二次确认仍在 scratch 下
	rel, err := filepath.Rel(dir, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("%w: escapes scratch", ErrBadFilename)
	}
	return full, nil
}

// Write 写入设备 scratch 下的 .py 文件。
func (j *Jail) Write(deviceID, filename, content string) (string, error) {
	path, err := j.ResolvePath(deviceID, filename)
	if err != nil {
		return "", err
	}
	if len(content) > j.maxFile() {
		return "", fmt.Errorf("%w: max %d bytes", ErrTooLarge, j.maxFile())
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", fmt.Errorf("codejail: mkdir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		return "", fmt.Errorf("codejail: write: %w", err)
	}
	return filepath.Base(path), nil
}

// Run 在 scratch 目录执行已写入的 .py。
func (j *Jail) Run(ctx context.Context, deviceID, filename string) (*RunResult, error) {
	path, err := j.ResolvePath(deviceID, filename)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("codejail: file not found: %s", filepath.Base(path))
		}
		return nil, fmt.Errorf("codejail: stat: %w", err)
	}
	dir := filepath.Dir(path)
	bin := j.pythonBin()
	if _, err := exec.LookPath(bin); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrPythonMissing, bin)
	}

	timeout := j.timeout()
	rctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(rctx, bin, "-I", filepath.Base(path))
	cmd.Dir = dir
	cmd.Env = []string{
		"PATH=/usr/bin:/bin",
		"LANG=C.UTF-8",
		"HOME=" + dir,
		"PYTHONDONTWRITEBYTECODE=1",
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	res := &RunResult{
		Stdout: truncate(stdout.String(), MaxOutputBytes),
		Stderr: truncate(stderr.String(), MaxOutputBytes),
	}
	if rctx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
		res.ExitCode = -1
		return res, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	}
	if err != nil {
		return res, fmt.Errorf("codejail: run: %w", err)
	}
	res.ExitCode = 0
	return res, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
