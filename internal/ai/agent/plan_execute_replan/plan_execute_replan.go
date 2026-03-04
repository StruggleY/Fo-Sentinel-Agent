package plan_execute_replan

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino-examples/adk/common/prints"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
)

var (
	globalPlanExecAgent   adk.Agent
	globalPlanExecOnce    sync.Once
	globalPlanExecInitErr error
)

// getPlanExecuteAgent 返回全局缓存的 Plan-Execute-Replan Agent（懒初始化，线程安全）。
//
// 进程生命周期内只执行一次 Agent 组装（Planner+Executor+Replanner → planexecute.New），
// 所有并发请求复用同一个 Agent 实例（adk.NewRunner 每次新建是正确的，因为 Runner 持有查询状态）。
func getPlanExecuteAgent(ctx context.Context) (adk.Agent, error) {
	globalPlanExecOnce.Do(func() {
		initCtx := context.Background()

		// Planner：任务拆解（深度思考模型，运行一次）
		planAgent, err := NewPlanner(initCtx)
		if err != nil {
			globalPlanExecInitErr = fmt.Errorf("init Planner: %w", err)
			return
		}

		// Executor：执行当前步骤（快速响应模型，ReAct 循环工具调用）
		executeAgent, err := NewExecutor(initCtx)
		if err != nil {
			globalPlanExecInitErr = fmt.Errorf("init Executor: %w", err)
			return
		}

		// Replanner：评估执行结果并决策继续/终止（深度思考模型）
		replanAgent, err := NewRePlanAgent(initCtx)
		if err != nil {
			globalPlanExecInitErr = fmt.Errorf("init Replanner: %w", err)
			return
		}

		// planexecute.New 在底层组合为：SequentialAgent[Planner, LoopAgent[Executor, Replanner]]
		// MaxIterations:20 是 LoopAgent 的循环上限，即 Executor+Replanner 最多交替执行 20 轮
		globalPlanExecAgent, globalPlanExecInitErr = planexecute.New(initCtx, &planexecute.Config{
			Planner:       planAgent,
			Executor:      executeAgent,
			Replanner:     replanAgent,
			MaxIterations: 20,
		})
	})
	return globalPlanExecAgent, globalPlanExecInitErr
}

// BuildPlanAgent 组装 Planner / Executor / Replanner 三个角色，
// 驱动完整的 Plan-Execute-Replan 循环，阻塞直到任务完成后返回最终结果。
//
// 注意：此函数保持原有签名不变（接受 query 参数并立即运行），调用方无感知内部缓存优化。
// 内部改为复用缓存的 Agent，仅 adk.NewRunner 是每次新建（正确，因为 Runner 持有查询状态）。
//
// ── 底层 Agent 组合结构（planexecute.New 内部实现）──────────────────────────
//
//	SequentialAgent "plan_execute_replan"
//	  ├─ Planner          （顺序执行，运行一次，生成初始步骤清单）
//	  └─ LoopAgent "execute_replan"  （循环执行，最多 MaxIterations 轮）
//	       ├─ Executor     （每轮执行 Plan.FirstStep，结果写入 Session）
//	       └─ Replanner    （读取执行结果，调用 PlanTool 继续 或 RespondTool 终止）
//
// ── 三个角色的通信机制：Session KV ──────────────────────────────────────────
//
//	三个角色之间不通过函数参数传递数据，而是通过共享的 Session（ctx 内嵌的 KV store）：
//	  "UserInput"     ← Planner 写入原始 query，Executor/Replanner 读取
//	  "Plan"          ← Planner/Replanner 写入步骤列表，Executor 读取 FirstStep 执行
//	  "ExecutedStep"  ← Executor 写入本步结果，Replanner 读取后追加到 ExecutedSteps
//	  "ExecutedSteps" ← Replanner 累积维护，注入 Prompt 使 LLM 了解已完成工作
//
// ── 完整循环流程 ──────────────────────────────────────────────────────────────
//
//	① Planner   接收 query，强制调用 PlanTool 生成 {"steps":[...]}，存入 Session["Plan"]
//	② Executor  从 Session 读取 Plan.FirstStep，以 ReAct 循环调用工具完成该步，结果存入 Session["ExecutedStep"]
//	③ Replanner 读取执行结果，结合原始 query + 全部已完成步骤，强制调用以下两个工具之一：
//	              PlanTool    → 更新 Session["Plan"]（仅剩余步骤），LoopAgent 进入下一轮
//	              RespondTool → 发送 BreakLoopAction 打破 LoopAgent，流程终止
//	④ 重复 ②③ 直到 Replanner 调用 RespondTool 或达到 MaxIterations 上限
//
// ── 参数与返回 ────────────────────────────────────────────────────────────────
//
//	query   ：初始任务描述，发送给 Planner 做任务分解
//	string  ：最终回答（Replanner 调用 RespondTool 时输出的 response 字段）
//	[]string：所有步骤的中间输出列表，供前端展示执行过程或后端审计
func BuildPlanAgent(ctx context.Context, query string) (string, []string, error) {
	// 复用全局缓存的 Agent（Planner+Executor+Replanner 已组装完毕）
	agent, err := getPlanExecuteAgent(ctx)
	if err != nil {
		return "", []string{}, fmt.Errorf("get cached PlanExecuteAgent: %w", err)
	}

	// adk.NewRunner 将 Agent 包装为事件驱动的运行器。
	// 注意：Runner 每次新建是正确的（持有 query 查询状态），只有 Agent 本身需要缓存。
	// r.Query 启动异步执行（内部开 goroutine），返回事件迭代器 iter。
	// 每当 Agent 有输出（LLM 回复、工具结果、步骤完成等）时，iter.Next() 返回对应事件。
	r := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
	iter := r.Query(ctx, query)

	var lastMessage adk.Message
	var detail []string

	// 持续消费事件，直到迭代器耗尽（所有步骤执行完毕或达到 MaxIterations）。
	// event.Output != nil 表示该事件携带 LLM 文本输出（区别于 Action 事件、Error 事件等）。
	// 每次收到输出都更新 lastMessage，循环结束后 lastMessage 即为 Replanner 的最终回答。
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		fmt.Println("------------- Event -------------")
		prints.Event(event) // 将事件结构格式化打印到控制台，便于开发时追踪每步执行情况
		if event.Output != nil {
			lastMessage, _, err = adk.GetMessage(event)
			detail = append(detail, lastMessage.String())
		}
	}

	// lastMessage 为 nil 说明整个流程未产生任何有效 LLM 输出，
	// 通常是 Planner 或首步 Executor 初始化失败导致。
	if lastMessage == nil {
		return "", []string{}, fmt.Errorf("get lastMessage Error")
	}
	return lastMessage.Content, detail, nil
}
