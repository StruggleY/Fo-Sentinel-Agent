package plan_pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// BuildPlanAgent 驱动完整的 Plan-Execute-Replan 循环，通过两个回调实时流式推送输出。
//
// ── 底层 Agent 组合结构（planexecute.New 内部实现）──────────────────────────
//
//	SequentialAgent "plan_pipeline"
//	  ├─ Planner          （顺序执行，运行一次，生成初始步骤清单）
//	  └─ LoopAgent "execute_replan"  （循环执行，最多 MaxIterations 轮）
//	       ├─ Executor     （每轮执行 Plan.FirstStep，结果写入 Session）
//	       └─ Replanner    （读取执行结果，调用 PlanTool 继续 或 RespondTool 终止）
//
// ── 回调分工 ──────────────────────────────────────────────────────────────────
//
//	onStep  ：中间步骤输出（Planner 规划、Executor 执行过程），推送到规划过程块展示
//	onFinal ：最终答案（Replanner 终止时生成的最终回复），推送到主内容气泡展示
//
// ── 实现原理：事件分类 ────────────────────────────────────────────────────────
//
//	按消息角色和内容分类处理：
//	  - Tool 消息（Worker 工具返回结果）→ onStep（中间步骤）
//	  - Assistant 纯文本（Executor 总结）→ onStep（中间步骤）
//	  - Assistant JSON（Planner/Replanner）→ 解析 Respond 字段作为 finalAnswer
//	  - finalAnswer 存在时 → onFinal；否则 fullContent 兜底 → onFinal
func BuildPlanAgent(ctx context.Context, query string, onStep func(string), onFinal func(string)) (string, error) {
	// 每次请求重新创建子 Agent：
	// ChatModelAgent（Executor）实现了 OnSetAsSubAgent，会修改内部 parentAgent 字段，
	// 导致第二次调用 planexecute.New 时报 "agent has already been set as a sub-agent" 错误。
	// 因此三个子 Agent 和 planexecute.New 均需每次请求重建（只是 HTTP 客户端初始化，无 API 调用开销）。
	planAgent, err := NewPlanner(ctx)
	if err != nil {
		return "", fmt.Errorf("init Planner: %w", err)
	}
	executeAgent, err := NewExecutor(ctx)
	if err != nil {
		return "", fmt.Errorf("init Executor: %w", err)
	}
	replanAgent, err := NewRePlanAgent(ctx)
	if err != nil {
		return "", fmt.Errorf("init Replanner: %w", err)
	}
	agent, err := planexecute.New(ctx, &planexecute.Config{
		Planner:       planAgent,
		Executor:      executeAgent,
		Replanner:     replanAgent,
		MaxIterations: 20,
	})
	if err != nil {
		return "", fmt.Errorf("create PlanExecuteAgent: %w", err)
	}

	// adk.NewRunner 将 Agent 包装为事件驱动的运行器。
	// 注意：Runner 每次新建是正确的（持有 query 查询状态），只有 Agent 本身需要缓存。
	// r.Query 启动异步执行（内部开 goroutine），返回事件迭代器 iter。
	// 每当 Agent 有输出（LLM 回复、工具结果、步骤完成等）时，iter.Next() 返回对应事件。
	r := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
	iter := r.Query(ctx, query)

	var fullContent string
	var finalAnswer string // Replanner Respond 工具的最终答案

	// 持续消费事件，直到迭代器耗尽。
	// 事件分四类：
	//   1. Planner/Replanner 的 JSON 输出（Assistant，Content 以 "{" 开头）：
	//      - Respond JSON → 解析 response 字段作为最终答案
	//      - Plan JSON    → 跳过
	//   2. Executor LLM 纯文本输出（Assistant，无 ToolCalls）→ 推送为 plan_step
	//   3. Executor Tool 消息（Worker 工具返回结果）→ 推送为 plan_step
	//   4. 其他（带 ToolCalls 的 Assistant 消息、空内容等）→ 跳过
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		g.Log().Debugf(ctx, "[PlanPipeline] 事件到达 | agentName=%s", event.AgentName)
		if event.Err != nil {
			return fullContent, fmt.Errorf("plan agent 执行错误: %w", event.Err)
		}
		if event.Output == nil {
			continue
		}
		msg, _, err := adk.GetMessage(event)
		if err != nil || msg == nil {
			continue
		}

		// Tool 消息：Worker 工具的执行结果，直接作为步骤内容推送
		if msg.Role == schema.Tool {
			content := strings.TrimSpace(msg.Content)
			if content == "" {
				continue
			}
			if onStep != nil {
				onStep(content)
			}
			fullContent += content + "\n"
			continue
		}

		// 只处理 Assistant 消息
		if msg.Role != schema.Assistant {
			continue
		}

		// 带 ToolCalls 的 Assistant 消息（LLM 决定调用工具）→ 跳过
		if len(msg.ToolCalls) != 0 {
			continue
		}

		trimmed := strings.TrimSpace(msg.Content)
		if trimmed == "" {
			continue
		}
		// JSON 输出：Planner（Plan JSON）或 Replanner（Plan/Respond JSON）
		if strings.HasPrefix(trimmed, "{") {
			// 尝试解析为 Respond 工具的输出
			var resp planexecute.Response
			if json.Unmarshal([]byte(trimmed), &resp) == nil && resp.Response != "" {
				finalAnswer = resp.Response
			}
			continue
		}
		// Executor 自然语言文本输出 → 推送为 plan_step
		if onStep != nil {
			onStep(msg.Content)
		}
		fullContent += msg.Content + "\n"
	}

	// 优先使用 Replanner Respond 解析出的最终答案
	if finalAnswer != "" {
		if onFinal != nil {
			onFinal(finalAnswer)
		}
		return fullContent + finalAnswer, nil
	}

	// 兜底：没有 Respond JSON，用 fullContent 作为最终答案
	if fullContent == "" {
		return "", fmt.Errorf("plan agent 未产生任何文本输出")
	}
	if onFinal != nil {
		onFinal(fullContent)
	}
	return fullContent, nil
}
