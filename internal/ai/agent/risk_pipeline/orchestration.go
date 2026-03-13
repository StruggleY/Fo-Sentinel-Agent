package risk_pipeline

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
	riskRunner  compose.Runnable[*base.UserMessage, *schema.Message] // 风险评估 Agent 运行器单例
	riskOnce    sync.Once                                            // 确保单例只初始化一次
	riskInitErr error                                                // 初始化过程中的错误
)

// GetRiskAgent 返回风险评估 Agent 单例（懒初始化，线程安全）。
// 使用 sync.Once 确保 DAG 只编译一次，所有并发请求复用同一个 Runnable 实例。
func GetRiskAgent(ctx context.Context) (compose.Runnable[*base.UserMessage, *schema.Message], error) {
	riskOnce.Do(func() {
		riskRunner, riskInitErr = buildRiskAgent(context.Background())
	})
	return riskRunner, riskInitErr
}

// buildRiskAgent 使用公共 DAG 构建器组装风险评估 Agent。
//
// DAG 拓扑（由 base.BuildReactAgentGraph 统一实现）：
//
//	START → [InputToRag, InputToChat] → MilvusRetriever → Template → ReactAgent → END
//
// 模型：DeepSeek V3 Think（深度推理版，适合深度分析 CVE、评估攻击路径和影响范围）
// 工具集：query_events / query_reports / search_similar_events /
//
//	query_internal_docs / query_subscriptions / get_current_time
func buildRiskAgent(ctx context.Context) (compose.Runnable[*base.UserMessage, *schema.Message], error) {
	model, err := newRiskModel(ctx)
	if err != nil {
		return nil, err
	}
	return base.BuildReactAgentGraph(ctx, base.BuildConfig{
		GraphName:    "RiskAgent",
		SystemPrompt: riskSystemPrompt,
		MaxStep:      15,
		Model:        model,
		Tools: []tool.BaseTool{
			toolsevent.NewQueryEventsTool(),
			toolsreport.NewQueryReportsTool(),
			toolsevent.NewSearchSimilarEventsTool(),
			toolssystem.NewQueryInternalDocsTool(),
			toolsevent.NewQuerySubscriptionsTool(), // 查询订阅源配置，了解事件来自哪个数据源，辅助评估情报置信度
			toolssystem.NewGetCurrentTimeTool(),
		},
	})
}
