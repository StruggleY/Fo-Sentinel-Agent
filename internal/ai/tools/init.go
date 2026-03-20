// 全局工具注册入口：进程启动时通过 Go 包机制 init() 自动执行，
// 将所有静态工具注册为全局单例，供各 Agent pipeline 和 Skills 执行器共享。
//
// 注册时机说明：
//   - Go 运行时在包被首次 import 时执行 init()，无需手动调用
//   - 本包被 skills/executor.go 和 agent/factory.go 等地方 import，确保启动时自动注册
//
// 未在此注册的工具类型：
//   - MCP 类工具（如腾讯云 CLS 日志查询）：依赖 context.Context 且可能返回多个动态工具，
//     由各需要它的 pipeline 在初始化时动态加载，不适合静态单例注册
package tools

import (
	toolsevent   "Fo-Sentinel-Agent/internal/ai/tools/event"
	toolsobserve "Fo-Sentinel-Agent/internal/ai/tools/observe"
	toolsreport  "Fo-Sentinel-Agent/internal/ai/tools/report"
	toolssystem  "Fo-Sentinel-Agent/internal/ai/tools/system"
)

// init 在包首次加载时自动执行，注册全部 10 个静态工具。
// 工具按功能域分组：事件类 / 报告类 / 系统类 / 监控告警类。
// 各工具实例为单例——无状态设计（只封装外部调用逻辑），并发安全，可被多个 Agent 同时使用。
func init() {
	// ── 事件类工具 ──────────────────────────────────────────────────────────
	// query_events: 按条件过滤查询 MySQL 安全事件表（支持时间、严重级别、来源等过滤）
	Register("query_events", toolsevent.NewQueryEventsTool())
	// search_similar_events: 基于 Milvus 向量相似度检索语义相近的历史事件
	Register("search_similar_events", toolsevent.NewSearchSimilarEventsTool())
	// query_subscriptions: 查询已配置的订阅源（RSS / GitHub / Webhook）
	Register("query_subscriptions", toolsevent.NewQuerySubscriptionsTool())

	// ── 报告类工具 ──────────────────────────────────────────────────────────
	// create_report: 生成并持久化安全报告（JSON 格式存入 MySQL）
	Register("create_report", toolsreport.NewCreateReportTool())
	// query_reports: 查询已生成的历史报告列表
	Register("query_reports", toolsreport.NewQueryReportsTool())
	// query_report_templates: 获取报告模板（周报、月报、自定义等）
	Register("query_report_templates", toolsreport.NewQueryReportTemplatesTool())

	// ── 系统类工具 ──────────────────────────────────────────────────────────
	// get_current_time: 获取当前时间（ReAct 推理中用于计算相对时间范围，如"最近7天"）
	Register("get_current_time", toolssystem.NewGetCurrentTimeTool())
	// query_database: 通用 MySQL CRUD 操作（Plan Agent 使用，执行复杂数据操作）
	Register("query_database", toolssystem.NewQueryDatabaseTool())
	// query_internal_docs: 查询 Milvus 内部文档知识库（Chat Agent 和 Risk Agent 使用）
	Register("query_internal_docs", toolssystem.NewQueryInternalDocsTool())

	// ── 监控告警类工具 ──────────────────────────────────────────────────────
	// query_metrics_alerts: 查询 Prometheus 监控指标和告警（Event Agent 和 Risk Agent 使用）
	Register("query_metrics_alerts", toolsobserve.NewQueryMetricsAlertsTool())
}
