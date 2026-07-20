package skills

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"
)

// BuiltinHandler 内置工具执行器（无脚本）。
type BuiltinHandler func(ctx context.Context, args map[string]interface{}) (string, error)

// Builtin 内存中的内置 skill。
type Builtin struct {
	Name        string
	Description string
	Parameters  map[string]Param
	Handler     BuiltinHandler
}

// ToToolDef 转成 LLM tool 描述。
func (b *Builtin) ToToolDef() llm.ToolDef {
	props := map[string]interface{}{}
	var required []string
	for name, p := range b.Parameters {
		props[name] = map[string]interface{}{
			"type":        coalesce(p.Type, "string"),
			"description": p.Description,
		}
		if p.Required {
			required = append(required, name)
		}
	}
	schema := map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return llm.ToolDef{
		Name:        b.Name,
		Description: b.Description,
		Parameters:  schema,
	}
}

// RunBuiltin 执行内置工具。
func RunBuiltin(ctx context.Context, b *Builtin, argsJSON string) (string, error) {
	if b == nil || b.Handler == nil {
		name := ""
		if b != nil {
			name = b.Name
		}
		return "", fmt.Errorf("builtin %q not runnable", name)
	}
	args := map[string]interface{}{}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("parse args: %w", err)
		}
	}
	return b.Handler(ctx, args)
}
