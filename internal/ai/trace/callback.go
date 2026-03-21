package trace

import (
	"context"
	"encoding/json"
	"io"
	"time"

	dao "Fo-Sentinel-Agent/internal/dao/mysql"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// ── Eino Callback 机制与集成原理 ──────────────────────────────────────────────
//
// Eino 的 callbacks 系统是其可观测性扩展点：框架在每个 DAG 节点执行的生命周期事件
//（OnStart / OnEnd / OnError / OnEndWithStreamOutput）上调用注册的 Handler，
// 无需修改 Agent / LLM / Tool 的实现代码，即可实现全链路追踪。
//
// 注册方式（main.go）：
//
//	ghttp.GetServer().AppendGlobalHandlers(trace.NewCallbackHandler())
//
// GoFrame 的 middleware 机制会将此 Handler 注入到每个请求的 ctx 中，
// Eino Graph 执行时自动从 ctx 取出并调用。
//
// ── 四个生命周期事件 ──────────────────────────────────────────────────────────
//
//  1. OnStart：节点开始执行时触发（同步）
//     - 创建 TraceNode，记录 parent_node_id（来自 SpanStack.Top()）和 depth
//     - TOOL 节点在此捕获输入参数 JSON（OnEnd 时才能写 metadata，需跨回调传递）
//     - 将 nodeID 压栈（Stack.Push）并注入 context（供 OnEnd 取回）
//
//  2. OnEnd：非流式节点成功完成时触发（同步）
//     - LLM 节点：提取 TokenUsage、模型名，累加到 ActiveTrace
//     - TOOL 节点：合并 OnStart 捕获的输入 + 本次输出，写入 metadata JSON
//     - RETRIEVER 节点：记录检索结果（top-3 序列化）和 final_top_k
//     - 出栈（Stack.Pop），更新 TraceNode 为 success 终态
//
//  3. OnError：节点执行失败时触发（同步）
//     - 分类错误码（RateLimit / Timeout / Canceled 等），写入 error_message
//     - 出栈（Stack.Pop），更新 TraceNode 为 error 终态
//
//  4. OnEndWithStreamOutput：流式节点成功完成时触发（异步）
//     - 适用于：LLM streaming（ChatModel 流式输出）以及 Eino 内部的 Lambda/ToolsNode
//     - 问题：Eino 对流式节点不调用 OnEnd，若不注册此回调则节点永远停留 running
//     - 方案：立即出栈（保证后续节点父子关系不错位），后台 goroutine 排空流读取
//       最后一个 chunk（含 TokenUsage），写入 TraceNode 终态
//     - 注意：排空流的 goroutine 不阻塞主调用链，与 SSE 推流并发执行
//
// ── SpanStack 与父子关系 ──────────────────────────────────────────────────────
//
// Eino Graph 串行执行路径（Router → Executor → SubAgent → LLM）：
//   Start(Router) → Push(R) → Start(Executor) → Push(E) → End(Router) → Pop → ...
// 此处有个微妙的顺序：Eino 的 OnStart 在节点进入前触发，OnEnd 在节点退出后触发，
// 但对于嵌套节点（如 Lambda 内调用 LLM），内部节点的 Start 在外部节点 End 之前触发，
// 形成天然的入栈/出栈配对，SpanStack 能正确维护父子关系。

// NewCallbackHandler 创建并注册 Eino 全局 callbacks.Handler。
// 注册为全局 Handler（AppendGlobalHandlers）后，所有请求的所有 Eino 节点均自动追踪。
func NewCallbackHandler() callbacks.Handler {
	builder := callbacks.NewHandlerBuilder()

	// ── OnStart：节点开始 ─────────────────────────────────────────────────────
	// 在此处确定节点的 parent_node_id 和 depth，因为 SpanStack 记录了当前调用深度。
	// 必须在 Push 之前读 Top/Depth，Push 之后新节点才成为下一层的父节点。
	builder.OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
		at := Extract(ctx)
		if at == nil || info == nil {
			return ctx
		}
		nodeID := uuid.New().String()
		parentID := at.Stack.Top() // 读栈顶作为父节点（Push 前读，保证不是自身）
		depth := at.Stack.Depth()  // 当前深度即为新节点的 depth
		nodeStartTime := time.Now()

		at.SetNodeStartTime(nodeID, nodeStartTime)

		asyncInsertNode(&dao.TraceNode{
			TraceID:      at.TraceID,
			NodeID:       nodeID,
			ParentNodeID: parentID,
			Depth:        depth,
			NodeType:     resolveNodeType(info),
			NodeName:     resolveNodeName(info),
			Status:       StatusRunning,
			StartTime:    nodeStartTime,
		})
		at.TrackNode(nodeID)

		// TOOL 节点：在 OnStart 捕获输入参数（OnEnd 时输入已不可见）
		if info.Component == components.ComponentOfTool {
			if toolIn := tool.ConvCallbackInput(input); toolIn != nil && toolIn.ArgumentsInJSON != "" {
				at.SetToolInput(nodeID, truncateText(toolIn.ArgumentsInJSON, 1000))
			}
		}

		at.Stack.Push(nodeID)
		// 将 nodeID 注入 ctx，供同一节点的 OnEnd/OnError 通过 ctx.Value(nodeIDKey{}) 取回
		return context.WithValue(ctx, nodeIDKey{}, nodeID)
	})

	// ── OnEnd：非流式节点成功 ──────────────────────────────────────────────────
	// 从 ctx 取回 nodeID（而非 Stack.Top()），原因：节点 End 时其子节点可能已入栈，
	// Stack.Top() 此时是最后一个子节点而非当前节点，会导致 Pop 错误。
	builder.OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
		at := Extract(ctx)
		if at == nil {
			return ctx
		}
		nodeID, _ := ctx.Value(nodeIDKey{}).(string)
		if nodeID == "" {
			return ctx
		}
		endTime := time.Now()
		nodeStartTime := at.GetNodeStartTime(nodeID)
		update := buildNodeUpdate(info, output, at, nodeID)
		at.UntrackNode(nodeID)
		asyncFinishNode(nodeID, StatusSuccess, "", "", "", nodeStartTime, endTime, update)
		at.Stack.Pop()
		return ctx
	})

	// ── OnError：节点失败 ─────────────────────────────────────────────────────
	builder.OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
		at := Extract(ctx)
		if at == nil {
			return ctx
		}
		nodeID, _ := ctx.Value(nodeIDKey{}).(string)
		if nodeID == "" {
			return ctx
		}
		errCode, errType := classifyError(err)
		endTime := time.Now()
		nodeStartTime := at.GetNodeStartTime(nodeID)
		at.UntrackNode(nodeID)
		asyncFinishNode(nodeID, StatusError, truncateError(err), errCode, errType, nodeStartTime, endTime, nil)
		at.Stack.Pop()
		return ctx
	})

	// ── OnEndWithStreamOutput：流式节点成功 ────────────────────────────────────
	// Eino 对 ChatModel streaming 和部分 Lambda 节点调用此回调而非 OnEnd。
	// 关键点：
	//  1. 立即出栈（同步），保证后续节点（如下一个 ReAct 步骤）能拿到正确的父节点
	//  2. 后台 goroutine 排空 StreamReader 获取最终 chunk（含 TokenUsage），再写终态
	//  3. 调用方（Eino Graph）持有 StreamReader 的另一端，两端并发读写，互不阻塞
	builder.OnEndWithStreamOutputFn(func(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
		at := Extract(ctx)
		if at == nil {
			output.Close()
			return ctx
		}
		nodeID, _ := ctx.Value(nodeIDKey{}).(string)
		// 立即出栈：此处与 OnEnd 不同，必须在 goroutine 启动前出栈，
		// 否则后续同步节点的 OnStart 会在后台 goroutine Pop 前先 Push，导致栈错乱
		at.Stack.Pop()
		if nodeID == "" {
			output.Close()
			return ctx
		}
		nodeStartTime := at.GetNodeStartTime(nodeID)
		// 后台排空流：读取所有 chunk，取最后一个（含完整 TokenUsage）写终态
		go func() {
			defer output.Close()
			var lastChunk callbacks.CallbackOutput
			for {
				chunk, err := output.Recv()
				if err != nil {
					if err != io.EOF {
						// 流异常（网络断开、LLM 报错等）：标记为错误
						errCode, errType := classifyError(err)
						at.UntrackNode(nodeID)
						asyncFinishNode(nodeID, StatusError, truncateError(err), errCode, errType, nodeStartTime, time.Now(), nil)
					} else {
						// 流正常结束：使用最后一个 chunk 提取 TokenUsage
						var update *NodeUpdate
						if lastChunk != nil {
							update = buildNodeUpdate(info, lastChunk, at, nodeID)
						}
						at.UntrackNode(nodeID)
						asyncFinishNode(nodeID, StatusSuccess, "", "", "", nodeStartTime, time.Now(), update)
					}
					return
				}
				lastChunk = chunk
			}
		}()
		return ctx
	})

	return builder.Build()
}

// resolveNodeType 根据 Eino RunInfo.Component 字段映射到 trace 节点类型常量。
// 未匹配的组件（Lambda、Prompt、Indexer 等）统一归为 LAMBDA 类型，
// 前端以统一样式展示，避免节点类型枚举膨胀。
func resolveNodeType(info *callbacks.RunInfo) string {
	switch info.Component {
	case components.ComponentOfChatModel:
		return NodeTypeLLM
	case components.ComponentOfTool:
		return NodeTypeTool
	case components.ComponentOfRetriever:
		return NodeTypeRetriever
	case components.ComponentOfEmbedding:
		return NodeTypeEmbedding
	default:
		return NodeTypeLambda
	}
}

// resolveNodeName 从 Eino RunInfo 提取可读的节点名称，优先级从高到低：
//
//  1. RunInfo.Name：通过 compose.WithNodeName("InputToRag") 显式设置的语义名称（最优）
//  2. RunInfo.Type：Eino 框架自动推断的实现类型名（如 "OpenAI"、"MilvusRetriever"）
//  3. 组件类别兜底（最低优先级，仅在两者均为空时使用）
//
// Lambda 节点通过 builder.go 中的 WithNodeName 显式设置名称，确保显示具体节点名而非通用 "Lambda"。
func resolveNodeName(info *callbacks.RunInfo) string {
	if info.Name != "" {
		return info.Name
	}
	if info.Type != "" {
		return info.Type
	}
	switch info.Component {
	case components.ComponentOfChatModel:
		return "ChatModel"
	case components.ComponentOfTool:
		return "Tool"
	case components.ComponentOfRetriever:
		return "Retriever"
	case components.ComponentOfEmbedding:
		return "Embedding"
	case components.ComponentOfIndexer:
		return "Indexer"
	case components.ComponentOfPrompt:
		return "Prompt"
	default:
		if info.Component != "" {
			return string(info.Component)
		}
		return "Lambda"
	}
}

// buildNodeUpdate 根据组件类型断言 output，提取结构化信息填充 NodeUpdate。
// 每种组件类型的输出结构不同，需分别处理：
//   - LLM（ChatModel）：提取 TokenUsage + 模型名 + 可选 Completion 文本
//   - TOOL：将 OnStart 捕获的输入参数 + 本次输出合并为 metadata JSON
//   - RETRIEVER：序列化检索结果（top-3，每条截断 500 字符）+ final_top_k
func buildNodeUpdate(info *callbacks.RunInfo, output callbacks.CallbackOutput, at *ActiveTrace, nodeID string) *NodeUpdate {
	if info == nil || output == nil {
		return nil
	}
	update := &NodeUpdate{}
	switch info.Component {
	case components.ComponentOfChatModel:
		modelOut := model.ConvCallbackOutput(output)
		if modelOut == nil {
			return nil
		}
		if modelOut.Config != nil {
			update.ModelName = modelOut.Config.Model
		}
		if modelOut.TokenUsage != nil {
			update.InputTokens = modelOut.TokenUsage.PromptTokens
			update.OutputTokens = modelOut.TokenUsage.CompletionTokens
			update.CachedTokens = modelOut.TokenUsage.PromptTokenDetails.CachedTokens
			// 累加到链路级别的 Token 累加器
			at.AddTokens(update.InputTokens, update.OutputTokens, update.CachedTokens)
		}
		// record_prompt=true 时记录 Completion 文本（含 PII 风险，默认关闭）
		if GetConfig().RecordPrompt && modelOut.Message != nil {
			update.CompletionText = truncateText(modelOut.Message.Content, 5000)
		}

	case components.ComponentOfTool:
		// 将 tool_name、OnStart 捕获的输入参数、本次输出合并为 metadata JSON
		meta := map[string]any{"tool_name": info.Name}
		if toolInput := at.GetToolInput(nodeID); toolInput != "" {
			meta["tool_input"] = toolInput
		}
		if toolOut := tool.ConvCallbackOutput(output); toolOut != nil && toolOut.Response != "" {
			meta["tool_output"] = truncateText(toolOut.Response, 2000)
		}
		if b, err := json.Marshal(meta); err == nil {
			update.Metadata = string(b)
		}

	case components.ComponentOfRetriever:
		retrieverOut := retriever.ConvCallbackOutput(output)
		if retrieverOut == nil {
			return nil
		}
		update.FinalTopK = len(retrieverOut.Docs)
		// 序列化 top-3 检索结果（每条内容截断 500 字符，防止 LONGTEXT 字段过大）
		type docInfo struct {
			Content  string         `json:"content"`
			Score    float64        `json:"score,omitempty"`
			Metadata map[string]any `json:"metadata,omitempty"`
		}
		docs := make([]docInfo, 0, min(3, len(retrieverOut.Docs)))
		for i, doc := range retrieverOut.Docs {
			if i >= 3 {
				break
			}
			di := docInfo{
				Content:  truncateText(doc.Content, 500),
				Metadata: doc.MetaData,
			}
			if doc.MetaData != nil {
				if scoreVal, ok := doc.MetaData["score"]; ok {
					if score, ok2 := scoreVal.(float64); ok2 {
						di.Score = score
					}
				}
			}
			docs = append(docs, di)
		}
		if b, err := json.Marshal(docs); err == nil {
			update.RetrievedDocs = string(b)
		}
	}
	return update
}

// truncateText 按 rune 数截断字符串（Unicode 安全），超出部分追加 "..."
func truncateText(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

// min 返回两个整数的较小值（Go 1.21 泛型 min 的兼容实现）
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
