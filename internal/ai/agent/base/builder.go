package base

import (
	"context"
	"time"

	retriever2 "Fo-Sentinel-Agent/internal/ai/retriever"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

// BuildConfig 公共 RAG+ReAct DAG 构建配置。
// 各 Agent 通过差异化此配置来定制自己的行为，消除重复的 DAG 搭建代码。
type BuildConfig struct {
	GraphName    string                     // DAG 图名称，用于日志追踪和性能监控
	SystemPrompt string                     // 系统提示词，须包含 {date} 和 {documents} 占位符
	MaxStep      int                        // ReAct 最大推理步数，≤0 时使用默认值 15
	Model        model.ToolCallingChatModel // 支持 Function Calling 的 LLM 实例
	Tools        []tool.BaseTool            // ReAct Agent 可调用的工具集
}

// BuildReactAgentGraph 构建标准五节点 RAG+ReAct 有向无环图并返回可执行 Runnable。
//
// 适用场景：event_analysis / report / risk 三个 Agent（DAG 拓扑完全一致，仅配置不同）。
// 不适用：
//   - chat_pipeline：InputToChat 含 Token 感知压缩逻辑，需保留自己的 Lambda
//   - summary_pipeline：线性 4 节点流水线，无 RAG 分支
//   - plan_pipeline：plan-execute-replan 架构，使用 adk.Agent
//   - knowledge_index_pipeline：文档索引管道，输入输出类型完全不同
//
// DAG 拓扑：
//
//	START
//	  ├── InputToRag  → MilvusRetriever ──┐
//	  └── InputToChat ────────────────────┤
//	                                      ▼
//	                               Template（FString 组装 System+History+User）
//	                                      ▼
//	                               ReactAgent（ReAct 循环，调用工具推理）
//	                                      ▼
//	                                    END
//
// 触发模式：AllPredecessor —— Template 节点等待两条并行支路全部完成后再触发（fan-in 屏障）。
func BuildReactAgentGraph(ctx context.Context, cfg BuildConfig) (compose.Runnable[*UserMessage, *schema.Message], error) {
	const (
		InputToRag      = "InputToRag"
		InputToChat     = "InputToChat"
		MilvusRetriever = "MilvusRetriever"
		Template        = "Template"
		ReactAgent      = "ReactAgent"
	)

	maxStep := cfg.MaxStep
	if maxStep <= 0 {
		maxStep = 15
	}

	g := compose.NewGraph[*UserMessage, *schema.Message]()

	// InputToRag：提取 Query 字符串，作为向量检索的输入文本（类型适配：*UserMessage → string）
	_ = g.AddLambdaNode(InputToRag, compose.InvokableLambda(
		func(ctx context.Context, input *UserMessage) (string, error) {
			return input.Query, nil
		},
	))

	// InputToChat：构建 Prompt 模板变量 map，注入 content / history / date 三个占位符
	_ = g.AddLambdaNode(InputToChat, compose.InvokableLambda(
		func(ctx context.Context, input *UserMessage) (map[string]any, error) {
			return map[string]any{
				"content": input.Query,
				"history": input.History,
				"date":    time.Now().Format("2006-01-02 15:04:05"),
			}, nil
		},
	))

	// MilvusRetriever：从向量数据库检索语义相关文档（含 Redis 语义缓存，TTL 24h，阈值 0.85）
	// WithOutputKey("documents")：检索结果以 "documents" 为 key 合并进 fan-in map，
	// 供 Template 节点通过 {documents} 占位符读取
	retriever, err := retriever2.GetRetriever(ctx)
	if err != nil {
		return nil, err
	}
	_ = g.AddRetrieverNode(MilvusRetriever, retriever, compose.WithOutputKey("documents"))

	// Template：使用 FString 格式将系统提示、历史消息列表、用户消息组装为完整 Message 列表
	// 渲染后输出：[SystemMessage(systemPrompt), ...history, UserMessage(content)]
	ctp := prompt.FromMessages(schema.FString,
		schema.SystemMessage(cfg.SystemPrompt),
		schema.MessagesPlaceholder("history", false),
		schema.UserMessage("{content}"),
	)
	_ = g.AddChatTemplateNode(Template, ctp)

	// ReactAgent：ReAct 循环（Reasoning + Acting），通过多步推理调用工具后生成最终结论
	// MaxStep 限制最大推理轮次，防止 LLM 陷入无限工具调用循环
	agentCfg := &react.AgentConfig{MaxStep: maxStep}
	agentCfg.ToolCallingModel = cfg.Model
	agentCfg.ToolsConfig.Tools = cfg.Tools
	agent, err := react.NewAgent(ctx, agentCfg)
	if err != nil {
		return nil, err
	}
	// AnyLambda 同时封装同步（Generate）和流式（Stream）两种调用方式
	reactLambda, err := compose.AnyLambda(agent.Generate, agent.Stream, nil, nil)
	if err != nil {
		return nil, err
	}
	_ = g.AddLambdaNode(ReactAgent, reactLambda)

	// 边定义：RAG 支路与对话历史支路并行执行，在 Template 节点 fan-in 汇聚
	_ = g.AddEdge(compose.START, InputToRag)
	_ = g.AddEdge(compose.START, InputToChat)
	_ = g.AddEdge(InputToRag, MilvusRetriever)
	_ = g.AddEdge(MilvusRetriever, Template)
	_ = g.AddEdge(InputToChat, Template)
	_ = g.AddEdge(Template, ReactAgent)
	_ = g.AddEdge(ReactAgent, compose.END)

	return g.Compile(ctx,
		compose.WithGraphName(cfg.GraphName),
		// AllPredecessor：Template 节点需等待 MilvusRetriever 和 InputToChat 全部完成才触发
		// 若使用默认 AnyPredecessor，任意一路完成即触发，另一路数据被丢弃，Prompt 渲染会出错
		compose.WithNodeTriggerMode(compose.AllPredecessor),
	)
}
