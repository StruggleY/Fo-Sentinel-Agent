// Package skills 技能系统：可插拔的 AI 技能注册与执行框架
// SKILL.md 格式：YAML frontmatter（元数据）+ Markdown body（指令）
package skills

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillMetadata SKILL.md 的 YAML frontmatter 元数据
type SkillMetadata struct {
	Name         string        `yaml:"name"`
	Description  string        `yaml:"description"`
	Version      string        `yaml:"version,omitempty"`
	Author       string        `yaml:"author,omitempty"`
	Category     string        `yaml:"category,omitempty"`
	Tags         []string      `yaml:"tags,omitempty"`
	AllowedTools []string      `yaml:"allowed-tools,omitempty"`
	Params       []ParamSchema `yaml:"params,omitempty"`
}

// ParamSchema 参数定义
type ParamSchema struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

// ParseSkillMD 解析完整的 SKILL.md 文件
// 解析流程：文件分离 → YAML 解析 → 字段验证 → 提取指令
func ParseSkillMD(content []byte) (*Skill, error) {
	// 【文件分离】按 "---" 分隔符切分为 3 部分
	parts := strings.SplitN(string(content), "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid SKILL.md format: missing frontmatter")
	}

	// 【YAML 解析】解析 frontmatter 为结构体
	var meta SkillMetadata
	if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// 【字段验证】检查必需字段
	if meta.Name == "" {
		return nil, fmt.Errorf("missing required field: name")
	}
	if meta.Description == "" {
		return nil, fmt.Errorf("missing required field: description")
	}

	// 【提取指令】Markdown body 作为 Prompt 模板
	instructions := strings.TrimSpace(parts[2])

	// 转换参数
	params := make([]SkillParam, len(meta.Params))
	for i, p := range meta.Params {
		params[i] = SkillParam{
			Name:        p.Name,
			Type:        p.Type,
			Description: p.Description,
			Required:    p.Required,
		}
	}

	return &Skill{
		ID:          meta.Name,
		Name:        meta.Name,
		Description: meta.Description,
		Category:    meta.Category,
		Tools:       meta.AllowedTools,
		Prompt:      instructions,
		Params:      params,
		Enabled:     true,
	}, nil
}
