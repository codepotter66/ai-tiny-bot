package skills

import "github.com/wisdomoasis/tiny-bot-cloud-agent/internal/llm"

// ToToolDef 把 Skill 转换成 llm.ToolDef。
//
// 生成的 parameters schema 形如：
//
//	{
//	  "type": "object",
//	  "properties": {
//	    "city": {"type": "string", "description": "城市名"}
//	  },
//	  "required": ["city"]
//	}
func (s *Skill) ToToolDef() llm.ToolDef {
	props := map[string]interface{}{}
	var required []string
	for name, p := range s.Parameters {
		prop := map[string]interface{}{
			"type":        coalesce(p.Type, "string"),
			"description": p.Description,
		}
		props[name] = prop
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
		Name:        s.Name,
		Description: s.Summary(),
		Parameters:  schema,
	}
}

func coalesce(a, b string) string {
	if a == "" {
		return b
	}
	return a
}
