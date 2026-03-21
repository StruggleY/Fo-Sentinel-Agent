package event_analysis_pipeline

import (
	"Fo-Sentinel-Agent/internal/ai/agent"
	"Fo-Sentinel-Agent/internal/ai/models"
	"Fo-Sentinel-Agent/internal/ai/prompt/agents"
)

// GetEventAnalysisAgent 返回事件分析 Agent 单例（懒初始化，线程安全）。
//
// DAG 拓扑（由 base.BuildReactAgentGraph 统一实现）：
//
//	START → [InputToRag, InputToChat] → MilvusRetriever → Template → ReactAgent → END
//
// 模型：DeepSeek V3 Quick（低延迟，适合实时事件分析的多步工具调用）
// 工具集：query_events / search_similar_events / query_subscriptions /
//
//	query_reports / query_internal_docs / get_current_time /
//	web_search / save_intelligence
var GetEventAnalysisAgent = agent.NewSingletonAgent(agent.AgentConfig{
	GraphName:      "EventAnalysisAgent",
	SystemPrompt:   agents.EventAnalysis,
	ModelFactory:   models.OpenAIForDeepSeekV3Quick,
	MaxStep:        15,
	RewriteEnabled: true,
	SplitEnabled:   true,
	ToolNames: []string{
		"query_events",
		"search_similar_events",
		"query_subscriptions",
		"query_reports",
		"query_internal_docs",
		"get_current_time",
		"web_search",
		"save_intelligence",
	},
})
