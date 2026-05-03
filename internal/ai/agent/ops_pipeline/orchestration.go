package ops_pipeline

import (
	"Fo-Sentinel-Agent/internal/ai/agent"
	"Fo-Sentinel-Agent/internal/ai/models"
	"Fo-Sentinel-Agent/internal/ai/prompt/agents"
)

// GetOpsAgent 运维 Agent 单例：基于事件分析结论执行通知/封禁/状态更新。
var GetOpsAgent = agent.NewSingletonAgent(agent.AgentConfig{
	GraphName:    "OpsAgent",
	SystemPrompt: agents.Ops,
	ModelFactory: models.OpenAIForDeepSeekV3Quick,
	MaxStep:      20,
	ToolNames: []string{
		"query_events",
		"trigger_ops",
		"update_event_status",
		"block_ip",
		"notify_dingtalk",
		"notify_wecom",
		"notify_email",
		"get_current_time",
	},
})
