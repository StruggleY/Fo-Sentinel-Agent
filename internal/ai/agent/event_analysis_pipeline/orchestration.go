package event_analysis_pipeline

import (
	"context"
	"sync"

	"Fo-Sentinel-Agent/internal/ai/agent/base"
	toolsevent "Fo-Sentinel-Agent/internal/ai/tools/event"
	toolsreport "Fo-Sentinel-Agent/internal/ai/tools/report"
	toolssystem "Fo-Sentinel-Agent/internal/ai/tools/system"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

var (
	eventRunner  compose.Runnable[*base.UserMessage, *schema.Message] // 事件分析 Agent 运行器单例
	eventOnce    sync.Once                                            // 确保单例只初始化一次
	eventInitErr error                                                // 初始化过程中的错误
)

// GetEventAnalysisAgent 返回事件分析 Agent 单例（懒初始化，线程安全）。
// 使用 sync.Once 确保 DAG 只编译一次，所有并发请求复用同一个 Runnable 实例。
func GetEventAnalysisAgent(ctx context.Context) (compose.Runnable[*base.UserMessage, *schema.Message], error) {
	eventOnce.Do(func() {
		eventRunner, eventInitErr = buildEventAnalysisAgent(context.Background())
	})
	return eventRunner, eventInitErr
}

// buildEventAnalysisAgent 使用公共 DAG 构建器组装事件分析 Agent。
//
// DAG 拓扑（由 base.BuildReactAgentGraph 统一实现）：
//
//	START → [InputToRag, InputToChat] → MilvusRetriever → Template → ReactAgent → END
//
// 模型：DeepSeek V3 Quick（低延迟，适合实时事件分析的多步工具调用）
// 工具集：query_events / search_similar_events / query_subscriptions /
//
//	query_reports / query_internal_docs / get_current_time
func buildEventAnalysisAgent(ctx context.Context) (compose.Runnable[*base.UserMessage, *schema.Message], error) {
	model, err := newEventModel(ctx)
	if err != nil {
		return nil, err
	}
	return base.BuildReactAgentGraph(ctx, base.BuildConfig{
		GraphName:    "EventAnalysisAgent",
		SystemPrompt: eventSystemPrompt,
		MaxStep:      15,
		Model:        model,
		Tools: []tool.BaseTool{
			toolsevent.NewQueryEventsTool(),
			toolsevent.NewSearchSimilarEventsTool(),
			toolsevent.NewQuerySubscriptionsTool(),
			toolsreport.NewQueryReportsTool(),      // 查询历史报告，关联"该事件是否曾被报告过"
			toolssystem.NewQueryInternalDocsTool(), // 查询内部安全规范/知识库，参考历史处置方案
			toolssystem.NewGetCurrentTimeTool(),
		},
	})
}
