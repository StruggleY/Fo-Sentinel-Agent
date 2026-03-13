// Package v1 Skill API：List、Execute（SSE 流式）
package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

// SkillListReq GET /skill/v1/list 获取已启用技能列表
type SkillListReq struct {
	g.Meta `path:"/skill/v1/list" method:"get" summary:"Skill 列表"`
}

type ParamInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type SkillInfo struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Category    string      `json:"category"`
	Params      []ParamInfo `json:"params"`
}

type SkillListRes struct {
	Skills []SkillInfo `json:"skills"`
}

// SkillExecuteReq POST /skill/v1/execute 执行技能，SSE 流式返回 step/result
type SkillExecuteReq struct {
	g.Meta  `path:"/skill/v1/execute" method:"post" summary:"执行 Skill"`
	SkillID string         `json:"skill_id" v:"required"`
	Params  map[string]any `json:"params"`
}

type SkillExecuteRes struct{}
