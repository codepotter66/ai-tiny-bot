package skills

import (
	"context"
	"fmt"
)

// Execute 按名执行 skill：内置优先，否则跑脚本。
func (r *Registry) Execute(ctx context.Context, name, argsJSON string) (string, error) {
	if b, ok := r.GetBuiltin(name); ok {
		out, err := RunBuiltin(ctx, b, argsJSON)
		if err != nil {
			return "", err
		}
		return out, nil
	}
	s, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("unknown skill %q", name)
	}
	res, err := Run(ctx, s, argsJSON)
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		msg := res.Stderr
		if msg == "" {
			msg = fmt.Sprintf("exit code %d", res.ExitCode)
		}
		return "", fmt.Errorf("skill %s failed: %s", name, msg)
	}
	return res.Stdout, nil
}
