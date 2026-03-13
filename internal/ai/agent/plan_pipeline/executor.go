package plan_pipeline

import (
	"Fo-Sentinel-Agent/internal/ai/models"
	toolsobserve "Fo-Sentinel-Agent/internal/ai/tools/observe"
	toolssystem "Fo-Sentinel-Agent/internal/ai/tools/system"
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/gogf/gf/v2/frame/g"
)

// NewExecutor 构建执行器，每次调用只执行 Plan 中的当前第一个步骤，执行完将结果交给 Replanner 评估。
// 使用快速响应模型：执行阶段以工具调用为主，速度优先。
// MaxIterations 设为不限，确保单步骤内工具调用可充分执行；整体步骤上限由外层 MaxIterations:20 控制。
//
// 底层实现原理（planexecute.NewExecutor）：
//  1. 每次调用时从 Session 读取三项共享状态：
//     UserInput       - 原始任务描述（key="UserInput"）
//     Plan            - 当前完整步骤列表，取 FirstStep() 作为本次执行目标（key="Plan"）
//     ExecutedSteps   - 已完成步骤及结果列表，注入 Prompt 供 LLM 了解上下文（key="ExecutedSteps"）
//  2. 将以上内容渲染为 ExecutorPrompt，以 ReAct 循环（ChatModelAgent）驱动工具调用完成该步骤。
//  3. 执行结果存入 Session（key="ExecutedStep"），供 Replanner 在下一轮读取做重规划决策。
func NewExecutor(ctx context.Context) (adk.Agent, error) {
	// MCP 服务不可用时（未配置或连接失败）优雅跳过，不影响其他工具的注册。
	var toolList []tool.BaseTool
	if mcpTools, err := toolsobserve.GetLogMcpTool(ctx); err != nil {
		g.Log().Warningf(ctx, "[PlanPipeline] MCP 日志工具加载失败，跳过: %v", err)
	} else {
		toolList = append(toolList, mcpTools...)
	}
	toolList = append(toolList, toolsobserve.NewQueryMetricsAlertsTool()) // Prometheus 活跃告警
	toolList = append(toolList, toolssystem.NewQueryInternalDocsTool())   // 内部文档检索
	toolList = append(toolList, toolssystem.NewGetCurrentTimeTool())      // 实时时间戳

	execModel, err := models.OpenAIForDeepSeekV3Quick(ctx)
	if err != nil {
		return nil, err
	}
	return planexecute.NewExecutor(ctx, &planexecute.ExecutorConfig{
		Model: execModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: toolList,
			},
		},
		MaxIterations: 999999,
	})
}
