// Package solve_pipeline 单事件解决方案生成 Agent。
// 区别于 event_analysis_pipeline（批量综合报告），本 Agent 专注于单条安全事件的
// 应急处置方案生成：相似历史事件检索 → 内部知识库查询 → 结构化三段式输出。
package solve_pipeline

import (
	"context"
	"sync"

	"Fo-Sentinel-Agent/internal/ai/agent/base"
	toolsevent "Fo-Sentinel-Agent/internal/ai/tools/event"
	toolssystem "Fo-Sentinel-Agent/internal/ai/tools/system"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// UserMessage 复用 base.UserMessage（type alias）。
type UserMessage = base.UserMessage

var (
	solveRunner  compose.Runnable[*base.UserMessage, *schema.Message] // 解决方案 Agent 单例
	solveOnce    sync.Once                                            // 确保单例只初始化一次
	solveInitErr error                                                // 初始化错误
)

// GetSolveAgent 返回解决方案生成 Agent 单例（懒初始化，线程安全）。
// 单例复用同一 Runnable，避免每次请求重复编译 DAG。
func GetSolveAgent(ctx context.Context) (compose.Runnable[*base.UserMessage, *schema.Message], error) {
	solveOnce.Do(func() {
		solveRunner, solveInitErr = buildSolveAgent(context.Background())
	})
	return solveRunner, solveInitErr
}

// buildSolveAgent 组装解决方案生成 Agent。
//
// DAG 拓扑（由 base.BuildReactAgentGraph 统一实现）：
//
//	START → [InputToRag, InputToChat] → MilvusRetriever → Template → ReactAgent → END
//
// 模型：DeepSeek V3 Think（深度推理，单事件方案需要充分推导攻击路径和修复措施）
// 工具集（最小化，专注于单事件）：
//   - search_similar_events：检索相似历史事件和处置记录，提供参考依据
//   - query_internal_docs：查询内部安全知识库，获取漏洞类型相关规范
//
// 刻意不包含 query_events（批量查询）和 get_current_time，
// 防止 Agent 偏离聚焦分析路径，降低 Token 消耗。
func buildSolveAgent(ctx context.Context) (compose.Runnable[*base.UserMessage, *schema.Message], error) {
	m, err := newSolveModel(ctx)
	if err != nil {
		return nil, err
	}
	return base.BuildReactAgentGraph(ctx, base.BuildConfig{
		GraphName:    "SolveAgent",
		SystemPrompt: solveSystemPrompt,
		MaxStep:      10,
		Model:        m,
		Tools: []tool.BaseTool{
			toolsevent.NewSearchSimilarEventsTool(), // 检索相似历史事件，提供处置参考
			toolssystem.NewQueryInternalDocsTool(),  // 内部安全知识库，获取行业规范和最佳实践
		},
	})
}
