// Package skills 提供技能面板 HTTP 控制器。
// 技能通过 skills/ 目录下的 SKILL.md 文件声明式定义，启动时加载到全局注册表。
// 包含两个端点：
//   - List：返回所有已注册技能的元数据（名称、描述、分类、参数列表）
//   - Execute：按 skill_id 执行技能，基于 ReAct Agent 推理并 SSE 流式输出结果
package skills

import (
	"context"

	v1 "Fo-Sentinel-Agent/api/skill/v1"
	"Fo-Sentinel-Agent/internal/ai/skills"
	"Fo-Sentinel-Agent/utility/sse"

	"github.com/gogf/gf/v2/frame/g"
)

type ControllerV1 struct{}

func NewV1() *ControllerV1 {
	return &ControllerV1{}
}

// List 返回已注册的技能列表，供前端 Skills 面板展示。
// 技能元数据包含：ID、名称、描述、分类、参数定义（名称/类型/是否必填）。
// 数据来自启动时扫描 skills/ 目录加载的全局注册表，无数据库查询。
func (c *ControllerV1) List(ctx context.Context, req *v1.SkillListReq) (*v1.SkillListRes, error) {
	list := skills.List()
	res := &v1.SkillListRes{Skills: make([]v1.SkillInfo, 0, len(list))}
	for _, s := range list {
		// 将技能定义转换为 API 响应 DTO，展开嵌套的参数列表
		info := v1.SkillInfo{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			Category:    s.Category,
			Params:      make([]v1.ParamInfo, 0, len(s.Params)),
		}
		// 逐个转换参数定义（前端据此渲染表单）
		for _, p := range s.Params {
			info.Params = append(info.Params, v1.ParamInfo{
				Name:        p.Name,
				Type:        p.Type,
				Description: p.Description,
				Required:    p.Required,
			})
		}
		res.Skills = append(res.Skills, info)
	}
	return res, nil
}

// Execute 执行指定技能，SSE 流式推送执行过程和结果，结束时发送 [DONE]。
// 执行流程：匹配技能 → 替换参数占位符 → 选取工具子集 → ReAct Agent 推理 → 流式输出。
// 创建执行器失败（如 skill_id 不存在）时立即发送 error 并关闭连接，不阻塞。
func (c *ControllerV1) Execute(ctx context.Context, req *v1.SkillExecuteReq) (*v1.SkillExecuteRes, error) {
	client := sse.NewClient(g.RequestFromCtx(ctx))

	g.Log().Infof(ctx, "[Skill] 开始执行 skill_id=%s", req.SkillID)

	// 创建执行器：根据 skill_id 从注册表查找技能定义，绑定请求参数替换占位符
	executor, err := skills.NewExecutor(req.SkillID, req.Params)
	if err != nil {
		g.Log().Errorf(ctx, "[Skill] 创建执行器失败: %v", err)
		client.Send("error", err.Error())
		client.Done()
		return nil, nil
	}

	// 执行技能：ReAct Agent 推理过程中通过回调函数逐步推送结果
	// result.Type 可为 "step"（推理步骤）或 "result"（最终结论）
	if err = executor.Execute(ctx, func(result skills.ExecuteResult) {
		g.Log().Debugf(ctx, "[Skill] 推送 type=%s len=%d", result.Type, len(result.Content))
		client.Send(result.Type, result.Content)
	}); err != nil {
		g.Log().Errorf(ctx, "[Skill] 执行失败: %v", err)
		client.Send("error", err.Error())
	}

	g.Log().Infof(ctx, "[Skill] 执行完成 skill_id=%s", req.SkillID)
	// 发送结束标志，通知前端关闭 SSE 连接
	client.Done()
	return nil, nil
}
