package report_pipeline

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
	reportRunner  compose.Runnable[*base.UserMessage, *schema.Message] // 报告生成 Agent 运行器单例
	reportOnce    sync.Once                                            // 确保单例只初始化一次
	reportInitErr error                                                // 初始化过程中的错误
)

// GetReportAgent 返回报告生成 Agent 单例（懒初始化，线程安全）。
// 使用 sync.Once 确保 DAG 只编译一次，所有并发请求复用同一个 Runnable 实例。
func GetReportAgent(ctx context.Context) (compose.Runnable[*base.UserMessage, *schema.Message], error) {
	reportOnce.Do(func() {
		reportRunner, reportInitErr = buildReportAgent(context.Background())
	})
	return reportRunner, reportInitErr
}

// buildReportAgent 使用公共 DAG 构建器组装报告生成 Agent。
//
// DAG 拓扑（由 base.BuildReactAgentGraph 统一实现）：
//
//	START → [InputToRag, InputToChat] → MilvusRetriever → Template → ReactAgent → END
//
// 模型：DeepSeek V3 Think（深度推理版，适合生成结构完整、内容丰富的长篇报告）
// 工具集：query_events / query_reports / query_report_templates /
//
//	search_similar_events / get_current_time / create_report
func buildReportAgent(ctx context.Context) (compose.Runnable[*base.UserMessage, *schema.Message], error) {
	model, err := newReportModel(ctx)
	if err != nil {
		return nil, err
	}
	return base.BuildReactAgentGraph(ctx, base.BuildConfig{
		GraphName:    "ReportAgent",
		SystemPrompt: reportSystemPrompt,
		MaxStep:      15,
		Model:        model,
		Tools: []tool.BaseTool{
			toolsevent.NewQueryEventsTool(),
			toolsreport.NewQueryReportsTool(),
			toolsreport.NewQueryReportTemplatesTool(),
			toolsevent.NewSearchSimilarEventsTool(),
			toolssystem.NewGetCurrentTimeTool(),
			toolsreport.NewCreateReportTool(), // 生成报告内容后调用，将结果持久化到数据库
		},
	})
}
