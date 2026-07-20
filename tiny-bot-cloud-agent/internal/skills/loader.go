// Package skills 加载并执行 workspace/skills/* 下的技能。
//
// 目录结构（每个 skill 一个目录）：
//
//	workspace/skills/<name>/
//	├── SKILL.md         # YAML frontmatter + body
//	├── scripts/         # 默认 run.sh，可由 frontmatter "script" 字段覆盖
//	├── references/      # 文本资源，按需注入到工具描述
//	└── assets/          # 静态资源
//
// SKILL.md frontmatter 示例：
//
//	---
//	name: weather
//	description: 查询某城市的实时天气
//	parameters:
//	  city:
//	    type: string
//	    description: 城市名
//	    required: true
//	script: scripts/run.sh
//	---
//
//	# Weather skill
//	返回 1 行结果。
package skills

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// Skill 加载后的内存表示。
type Skill struct {
	Name        string
	Description string
	Parameters  map[string]Param
	Script      string                 // 相对 skill 目录的脚本路径，默认 scripts/run.sh
	Body        string                 // SKILL.md 正文（去掉 frontmatter 之后）
	Dir         string                 // skill 目录绝对路径
	References  []string               // references 目录下文本文件名
}

// Param 工具参数定义。
type Param struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// LoadAll 扫描 root/skills/* 并加载全部 skill。
func LoadAll(root string) ([]*Skill, error) {
	dir := filepath.Join(root, "skills")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 目录不存在不是错
		}
		return nil, fmt.Errorf("read skills dir: %w", err)
	}
	var out []*Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, err := Load(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("load skill %s: %w", e.Name(), err)
		}
		out = append(out, s)
	}
	return out, nil
}

// Load 加载单个 skill 目录。
func Load(dir string) (*Skill, error) {
	mdPath := filepath.Join(dir, "SKILL.md")
	raw, err := os.ReadFile(mdPath)
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md: %w", err)
	}
	s, err := parseSkillMD(raw)
	if err != nil {
		return nil, err
	}
	s.Dir = dir

	// 默认脚本
	if s.Script == "" {
		s.Script = "scripts/run.sh"
	}
	// 校验脚本存在
	absScript := filepath.Join(dir, s.Script)
	if _, err := os.Stat(absScript); err != nil {
		return nil, fmt.Errorf("skill %q script not found: %s", s.Name, absScript)
	}
	// 收集 references
	refDir := filepath.Join(dir, "references")
	if entries, err := os.ReadDir(refDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				s.References = append(s.References, e.Name())
			}
		}
	}
	return s, nil
}

// parseSkillMD 解析 frontmatter + body。
func parseSkillMD(raw []byte) (*Skill, error) {
	// 必须以 --- 开头
	if !bytes.HasPrefix(raw, []byte("---")) {
		return nil, fmt.Errorf("SKILL.md must start with ---")
	}
	rest := raw[3:]
	// 找第二个 ---
	end := bytes.Index(rest, []byte("\n---"))
	if end < 0 {
		return nil, fmt.Errorf("SKILL.md missing closing ---")
	}
	fm := rest[:end]
	body := rest[end+4:]
	// 去掉 body 开头的换行
	body = bytes.TrimLeft(body, "\n")

	// frontmatter 解析
	var fmData struct {
		Name        string            `yaml:"name"`
		Description string            `yaml:"description"`
		Parameters  map[string]Param  `yaml:"parameters"`
		Script      string            `yaml:"script"`
	}
	if err := yaml.Unmarshal(fm, &fmData); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	if fmData.Name == "" {
		return nil, fmt.Errorf("SKILL.md name required")
	}
	if fmData.Description == "" {
		return nil, fmt.Errorf("SKILL.md description required")
	}
	return &Skill{
		Name:        fmData.Name,
		Description: fmData.Description,
		Parameters:  fmData.Parameters,
		Script:      fmData.Script,
		Body:        string(body),
	}, nil
}

// ScriptPath 返回 skill 脚本的绝对路径。
func (s *Skill) ScriptPath() string {
	return filepath.Join(s.Dir, s.Script)
}

// Summary 返回给 LLM 的简介（描述 + 正文前 200 字）。
func (s *Skill) Summary() string {
	b := strings.TrimSpace(s.Body)
	if len([]rune(b)) > 200 {
		b = string([]rune(b)[:200]) + "..."
	}
	if b == "" {
		return s.Description
	}
	return s.Description + "\n" + b
}
