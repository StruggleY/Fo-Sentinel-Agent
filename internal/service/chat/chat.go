// chat.go 对话核心业务逻辑：标准意图路由 + 深度思考模式（Plan Agent）。
package chatsvc

import (
	"context"

	"Fo-Sentinel-Agent/internal/ai/agent/plan_pipeline"
	"Fo-Sentinel-Agent/internal/ai/cache"
	"Fo-Sentinel-Agent/internal/ai/intent"
	"Fo-Sentinel-Agent/internal/ai/intent/core"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// ExecuteIntent 标准意图路由。
// Router 只在 chat / event / report / risk / solve 5 类意图中识别。
func ExecuteIntent(ctx context.Context, sessionId, query string, onOutput func(intentType, chunk string)) error {
	g.Log().Infof(ctx, "[Intent] 收到请求 | session=%s | query=%q", sessionId, query)
	ig := intent.NewIntent(ctx, sessionId)
	_, err := ig.Execute(query, func(intentType intent.IntentType, chunk string) {
		onOutput(string(intentType), chunk)
	})
	if err != nil {
		g.Log().Errorf(ctx, "[Intent] 执行失败 | session=%s | err=%v", sessionId, err)
	}
	return err
}

// ExecuteDeepThink 深度思考模式对话。
// 直接调用 Plan Agent（Supervisor-Worker 架构），跳过意图识别路由层，无 LLM 路由开销。
//
// sessionId 注入机制：
//
//	context.WithValue(ctx, SessionIdCtxKey{}, sessionId) 将 sessionId 嵌入 context，
//	随后整个调用树（BuildPlanAgent → adk.Runner → Executor → Worker 工具 Invoke）共享同一个 ctx。
//	Worker 工具（agent_worker.go）在 Invoke 时通过 ctx.Value(SessionIdCtxKey{}) 取出 sessionId，
//	调用 cache.GetSessionMemory(sessionId) 读取进程内会话历史（零 Redis 开销），
//	将最近 3 条历史拼成上下文前缀注入给专业 Agent，使 Worker 感知多轮对话上下文。
//	这是 Go 惯用的跨层透明传递方式，中间层（BuildPlanAgent、adk.Runner）无需感知。
func ExecuteDeepThink(ctx context.Context, sessionId, query string, onOutput func(intentType, chunk string)) error {
	g.Log().Infof(ctx, "[Intent] 深度思考请求 | session=%s | query=%q", sessionId, query)

	// 注入 sessionId：Worker 工具通过 ctx.Value(SessionIdCtxKey{}) 取出，读取会话历史后注入专业 Agent。
	// 传递路径：此处注入 → BuildPlanAgent → adk.Runner.Query → Executor → Worker.Invoke → buildWorkerContext
	ctx = context.WithValue(ctx, plan_pipeline.SessionIdCtxKey{}, sessionId)

	// 加载并初始化会话记忆（与标准路由 NewIntent 保持一致）
	mem := cache.GetSessionMemory(sessionId)
	recent, summary, err := cache.LoadSession(ctx, sessionId)
	if err == nil && (len(recent) > 0 || summary != "") {
		mem.SetState(recent, summary)
	} else if err != nil {
		g.Log().Warningf(ctx, "[Intent] 深度思考加载会话失败，使用进程内存 | session=%s | err=%v", sessionId, err)
	}
	mem.SetMessages(schema.UserMessage(query))

	// 阶段一：预思考（流式推送 think 事件，错误不中断主流程）
	onOutput(string(core.IntentStatus), "深度思考中...")
	thinkErr := plan_pipeline.StreamThinkChunks(ctx, query, func(chunk string) {
		onOutput(string(core.IntentPlanStep), plan_pipeline.MarshalThinkChunk(chunk))
	})
	if thinkErr != nil {
		g.Log().Warningf(ctx, "[Intent] 预思考阶段失败，继续执行 Plan Agent | session=%s | err=%v", sessionId, thinkErr)
	}

	// 阶段二：Plan Agent（任务规划 + 执行）
	onOutput(string(core.IntentStatus), "[Plan Agent 规划执行...]")
	content, execErr := plan_pipeline.BuildPlanAgent(ctx, query,
		func(chunk string) { onOutput(string(core.IntentPlanStep), chunk) }, // 中间步骤 → 规划过程块
		func(chunk string) { onOutput(string(core.IntentPlan), chunk) },     // 最终答案 → 正式内容
	)
	if execErr != nil {
		g.Log().Errorf(ctx, "[Intent] 深度思考执行失败 | session=%s | err=%v", sessionId, execErr)
		return execErr
	}

	// 将助手回复写入会话记忆并异步持久化到 Redis
	if content != "" {
		mem.SetMessages(schema.AssistantMessage(content, nil))
		go func() {
			bgCtx := context.Background()
			if persistErr := cache.SaveSessionWithRetry(bgCtx, sessionId,
				mem.GetRecentMessages(), mem.GetLongTermSummary()); persistErr != nil {
				g.Log().Errorf(bgCtx, "[Intent] 深度思考保存会话失败（已重试） | session=%s | err=%v", sessionId, persistErr)
			}
		}()
	}

	return nil
}
