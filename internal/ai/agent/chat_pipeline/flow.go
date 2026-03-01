package chat_pipeline

import (
	"Fo-Sentinel-Agent/internal/ai/tools"
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
)

// newReactAgentLambda 构建 ReAct Agent 并包装为 DAG 可用的 Lambda 节点。
//
// ReAct 执行模型（Reasoning + Acting）：
//  每一步 LLM 会输出结构化的 Tool Call（而非直接回答），框架捕获后执行对应工具，
//  将工具返回结果作为 Observation 追加进 Messages，再交还给 LLM 继续推理，循环往复：
//
//   Prompt → [Thought] → Tool Call → Observation → [Thought] → ... → Final Answer
//
//  整个过程对用户透明，用户只看到最终回答。
func newReactAgentLambda(ctx context.Context) (lba *compose.Lambda, err error) {
	config := &react.AgentConfig{
		// MaxStep 限制 Thought→Action 的最大循环轮次，防止 LLM 陷入无限工具调用死循环。
		// 超过 25 步后强制终止并返回当前结果。
		MaxStep: 25,
		// ToolReturnDirectly 指定哪些工具的返回值直接作为最终答案输出，跳过后续 LLM 推理。
		// 此处为空，表示所有工具结果都要经过 LLM 二次处理后再返回给用户。
		ToolReturnDirectly: map[string]struct{}{},
	}

	// ToolCallingModel 是驱动 ReAct 循环的 LLM，需支持 Function Calling（Tool Call）能力。
	// LLM 在每步推理时从注册的工具列表中选择调用哪个工具及其入参。
	chatModelIns11, err := newChatModel(ctx)
	if err != nil {
		return nil, err
	}
	config.ToolCallingModel = chatModelIns11

	// ── 工具注册 ──────────────────────────────────────────────────────
	// 每个工具都实现了 tool.InvokableTool 接口，包含：
	//   - 工具名称（LLM 通过名称决定调用哪个工具）
	//   - 工具描述（LLM 依据描述判断何时调用该工具）
	//   - 入参 JSON Schema（框架自动校验 LLM 传入的参数格式）
	//   - 执行函数（实际业务逻辑）

	// 腾讯云 CLS 日志查询（MCP 协议）：通过 SSE 连接腾讯云 MCP Server，
	// 动态获取 SearchLog 等日志工具，供 LLM 查询生产日志。
	mcpTool, err := tools.GetLogMcpTool()
	if err != nil {
		return nil, err
	}
	config.ToolsConfig.Tools = mcpTool

	// Prometheus 告警查询：调用本地 Prometheus HTTP API，获取当前所有 firing 告警列表
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewPrometheusAlertsQueryTool())

	// MySQL CRUD：执行任意 SQL，查询前有终端二次确认交互（防止 LLM 误操作数据库）
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewMysqlCrudTool())

	// 当前时间：返回秒/毫秒/微秒时间戳，解决 LLM 不感知实时时间的问题。
	// 尤其用于 SearchLog 工具的 From/To 时间范围计算（LLM 先调此工具获取 T，再计算时间窗口）
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewGetCurrentTimeTool())

	// 内部文档检索：从本地知识库（向量索引）检索与告警名称匹配的处理方案文档
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewQueryInternalDocsTool())

	// react.NewAgent 根据上述配置实例化 ReAct Agent，
	// 内部维护消息历史，每轮将 Observation 追加后重新调用 LLM。
	ins, err := react.NewAgent(ctx, config)
	if err != nil {
		return nil, err
	}

	// compose.AnyLambda 同时封装同步（Generate）和流式（Stream）两种调用方式，
	// DAG 框架在 Invoke 时调用 Generate，在 Stream 时调用 Stream，无需手动区分。
	// 后两个 nil 分别对应 StreamToInvoke 和 InvokeToStream 的适配器，此处不需要。
	lba, err = compose.AnyLambda(ins.Generate, ins.Stream, nil, nil)
	if err != nil {
		return nil, err
	}
	return lba, nil
}
