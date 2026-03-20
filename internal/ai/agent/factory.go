// Package agent 通用 Agent 工厂：消除 event/report/risk/solve 四个 pipeline 中重复的
// sync.Once 初始化样板代码。
//
// 背景：
//   改造前，每个 pipeline 的 orchestration.go 约有 100 行几乎完全相同的代码：
//   - sync.Once + runner + initErr 三个变量声明
//   - GetXxxAgent 函数：调用 ModelFactory、组装 BuildConfig、调用 BuildReactAgentGraph
//   6 个 pipeline 共计约 600 行重复代码，改一处通常需要改六处。
//
// 改造后：
//   各 pipeline 只需声明差异化配置（AgentConfig），约 20 行；
//   NewSingletonAgent 负责所有通用逻辑（懒初始化、sync.Once、错误传播）。
//
// 架构位置：
//   orchestration.go（各 pipeline）→ NewSingletonAgent（工厂） → BuildReactAgentGraph（DAG 构建）
package agent

import (
	"context"
	"sync"

	"Fo-Sentinel-Agent/internal/ai/agent/base"
	"Fo-Sentinel-Agent/internal/ai/tools"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// AgentConfig 通用 Agent 构建配置，各 pipeline 通过此结构声明差异化参数。
//
// 只需声明「这个 Agent 与其他 Agent 不同的地方」：
//   - 用什么系统提示词（SystemPrompt）
//   - 用什么 LLM 模型（ModelFactory）
//   - 允许调用哪些工具（ToolNames，从全局注册表按名取）
//   - 是否启用 RAG 质量增强（RewriteEnabled / SplitEnabled）
type AgentConfig struct {
	// GraphName DAG 图名称，用于 Eino 链路追踪和日志定位（如 "EventAnalysisAgent"）
	GraphName string
	// SystemPrompt 系统提示词，须包含 {date}、{documents}、{content}、{history} 四个占位符
	SystemPrompt string
	// ModelFactory 创建支持工具调用的 LLM 的工厂函数。
	// 单例初始化时调用一次（context.Background()），不依赖请求级 context。
	// 不同 Agent 可使用不同模型：V3 Quick（低延迟）或 V3 Think（深度推理）
	ModelFactory func(ctx context.Context) (model.ToolCallingChatModel, error)
	// ToolNames 从全局工具注册表按名称取工具子集。
	// 不存在的名称静默跳过（开发期配置错误，不影响运行时）。
	// 工具隔离：每个 Agent 只能访问声明的工具，防止 LLM 误用不相关工具。
	ToolNames []string
	// MaxStep ReAct 最大推理步数（每步 = 一轮 Think+Act+Observe）。
	// ≤0 时使用 BuildReactAgentGraph 默认值 15。
	// 步数越多允许越深的推理，但 API 调用次数和延迟也线性增加。
	MaxStep int
	// RewriteEnabled 是否在 RAG 检索前启用查询重写（约增加 200ms 延迟）。
	// 适合多轮对话 Agent；单轮查询无对话历史时重写无效。
	RewriteEnabled bool
	// SplitEnabled 是否启用子问题拆分+并行检索+可选 Rerank（约增加 300ms 延迟）。
	// 启用时 DAG 拓扑从「双节点」切换为「统一 RetrievalNode」。
	// 适合需要处理复杂多维查询的 Agent（event/report/risk）。
	SplitEnabled bool
}

// NewSingletonAgent 返回懒初始化单例 getter，封装 sync.Once 样板代码。
//
// 返回值是一个函数（getter），签名与各 pipeline 原有 GetXxxAgent 完全兼容，
// 调用方（subagent 适配器）无需修改：
//
//	func(context.Context) (compose.Runnable[*base.UserMessage, *schema.Message], error)
//
// 初始化时机：
//   - 首次调用 getter 时触发 once.Do（懒初始化）
//   - 初始化失败时，initErr 被记录，后续每次调用都返回同一个 error（快速失败）
//   - 初始化成功后，runner 被复用，无重复编译开销（Eino DAG 编译一次约需 100ms）
//
// 线程安全：
//   - sync.Once 保证初始化逻辑只执行一次，即使多个 goroutine 同时首次调用
//   - 初始化完成后 runner 为只读引用，并发调用 Runnable.Invoke/Stream 是安全的
func NewSingletonAgent(cfg AgentConfig) func(context.Context) (compose.Runnable[*base.UserMessage, *schema.Message], error) {
	var (
		runner  compose.Runnable[*base.UserMessage, *schema.Message] // Eino 编译后的可执行 DAG
		once    sync.Once                                            // 保证初始化只执行一次
		initErr error                                                // 初始化失败时缓存错误，供后续调用快速返回
	)
	return func(ctx context.Context) (compose.Runnable[*base.UserMessage, *schema.Message], error) {
		once.Do(func() {
			// 使用 context.Background() 而非请求级 ctx：
			// 模型初始化是进程级资源，不应因某个请求的 context 取消而失败
			m, err := cfg.ModelFactory(context.Background())
			if err != nil {
				initErr = err
				return
			}
			// 通过全局注册表按名获取工具子集（工具已在 init.go 注册为单例）
			runner, initErr = base.BuildReactAgentGraph(context.Background(), base.BuildConfig{
				GraphName:      cfg.GraphName,
				SystemPrompt:   cfg.SystemPrompt,
				MaxStep:        cfg.MaxStep,
				Model:          m,
				Tools:          tools.GetMany(cfg.ToolNames), // 按名取工具，不存在的静默跳过
				RewriteEnabled: cfg.RewriteEnabled,
				SplitEnabled:   cfg.SplitEnabled,
			})
		})
		return runner, initErr
	}
}
