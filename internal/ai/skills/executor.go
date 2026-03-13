// Package skills 技能系统：可插拔的 AI 技能注册与执行框架
package skills

import (
	"Fo-Sentinel-Agent/internal/ai/models"
	toolsevent "Fo-Sentinel-Agent/internal/ai/tools/event"
	toolsobserve "Fo-Sentinel-Agent/internal/ai/tools/observe"
	toolsreport "Fo-Sentinel-Agent/internal/ai/tools/report"
	toolssystem "Fo-Sentinel-Agent/internal/ai/tools/system"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// 全局工具注册表：所有可用工具的单例实例
// 工具是无状态的，可以安全地在多个执行器之间共享
var globalToolMap = map[string]tool.BaseTool{
	"query_internal_docs":    toolssystem.NewQueryInternalDocsTool(),
	"get_current_time":       toolssystem.NewGetCurrentTimeTool(),
	"query_database":         toolssystem.NewQueryDatabaseTool(),
	"query_events":           toolsevent.NewQueryEventsTool(),
	"query_subscriptions":    toolsevent.NewQuerySubscriptionsTool(),
	"search_similar_events":  toolsevent.NewSearchSimilarEventsTool(),
	"query_reports":          toolsreport.NewQueryReportsTool(),
	"query_report_templates": toolsreport.NewQueryReportTemplatesTool(),
	"create_report":          toolsreport.NewCreateReportTool(),
	"query_metrics_alerts":   toolsobserve.NewQueryMetricsAlertsTool(),
}

// Executor 技能执行器：通过 ReAct Agent 调用工具完成推理
type Executor struct {
	skill  *Skill         // 技能定义
	params map[string]any // 用户参数
}

// NewExecutor 创建技能执行器
func NewExecutor(skillID string, params map[string]any) (*Executor, error) {
	skill := Get(skillID)
	if skill == nil {
		return nil, fmt.Errorf("skills not found: %s", skillID)
	}
	return &Executor{skill: skill, params: params}, nil
}

// Execute 执行技能：参数替换 → 工具注入 → ReAct 推理 → 流式输出
func (e *Executor) Execute(ctx context.Context, callback func(ExecuteResult)) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	callback(ExecuteResult{Type: "step", Content: "正在准备执行 " + e.skill.Name + "..."})

	// 【构建提示词】替换占位符
	prompt := e.buildPrompt()
	// 【选取工具】注入技能所需工具
	selectedTools := e.selectTools()

	// 【初始化模型】DeepSeek V3
	dsModel, err := newModel(ctx)
	if err != nil {
		return err
	}

	// 【配置 Agent】最大 10 步推理
	config := &react.AgentConfig{
		MaxStep:          10,
		ToolCallingModel: dsModel,
	}
	config.ToolsConfig.Tools = selectedTools

	agent, err := react.NewAgent(ctx, config)
	if err != nil {
		return err
	}

	callback(ExecuteResult{Type: "step", Content: "正在分析..."})

	// 【执行推理】ReAct 循环
	g.Log().Infof(ctx, "[Skill] 开始 ReAct 推理 | skills=%s | prompt_len=%d", e.skill.ID, len(prompt))
	result, err := agent.Generate(ctx, []*schema.Message{schema.UserMessage(prompt)})
	if err != nil {
		g.Log().Errorf(ctx, "[Skill] ReAct 推理失败 | skills=%s | error=%v", e.skill.ID, err)
		return err
	}
	g.Log().Infof(ctx, "[Skill] ReAct 推理完成 | skills=%s | result_len=%d", e.skill.ID, len(result.Content))

	callback(ExecuteResult{Type: "result", Content: result.Content})
	return nil
}

// buildPrompt 替换 Prompt 中的 {name} 占位符
// 示例：Prompt="分析 {query}"，params={"query":"CVE-2024-1234"} → "分析 CVE-2024-1234"
func (e *Executor) buildPrompt() string {
	prompt := e.skill.Prompt // 获取模板
	for k, v := range e.params {
		// 遍历参数，将 {key} 替换为实际值
		// strings.ReplaceAll 替换所有匹配项
		// fmt.Sprintf("%v") 支持任意类型转字符串
		prompt = strings.ReplaceAll(prompt, "{"+k+"}", fmt.Sprintf("%v", v))
	}
	return prompt
}

// selectTools 按 Skill.Tools 从全局工具表选取子集
func (e *Executor) selectTools() []tool.BaseTool {
	var selected []tool.BaseTool
	for _, name := range e.skill.Tools {
		if t, ok := globalToolMap[name]; ok {
			selected = append(selected, t)
		}
	}
	return selected
}

// newModel 创建 DeepSeek V3 模型实例
func newModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	return models.OpenAIForDeepSeekV3Quick(ctx)
}
