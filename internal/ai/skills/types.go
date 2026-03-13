// Package skills 技能系统：可插拔的 AI 技能注册与执行框架
package skills

// Skill 技能定义：元信息 + 执行配置
type Skill struct {
	ID          string       `json:"id"`          // 技能唯一标识
	Name        string       `json:"name"`        // 显示名称
	Description string       `json:"description"` // 功能描述
	Category    string       `json:"category"`    // 分类标签
	Enabled     bool         `json:"enabled"`     // 是否启用
	Tools       []string     `json:"tools"`       // 允许的工具列表
	Prompt      string       `json:"-"`           // 提示词模板，支持 {name} 占位符
	Params      []SkillParam `json:"params"`      // 参数定义
}

// SkillParam 参数定义
type SkillParam struct {
	Name        string `json:"name"`        // 参数名
	Type        string `json:"type"`        // 类型：string/int/bool
	Description string `json:"description"` // 描述
	Required    bool   `json:"required"`    // 是否必填
}

// ExecuteRequest 执行请求
type ExecuteRequest struct {
	SkillID string         `json:"skill_id"` // 技能 ID
	Params  map[string]any `json:"params"`   // 参数 map
}

// ExecuteResult 执行结果：通过 SSE 流式推送
type ExecuteResult struct {
	Type    string `json:"type"`    // step=进度，result=最终结果
	Content string `json:"content"` // 内容
}
