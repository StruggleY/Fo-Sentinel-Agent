package chat

import (
	"Fo-Sentinel-Agent/api/chat/v1"
	"Fo-Sentinel-Agent/internal/ai/agent/chat_pipeline"
	"Fo-Sentinel-Agent/utility/log_call_back"
	"Fo-Sentinel-Agent/utility/mem"
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// Chat 处理单次同步（阻塞式）对话请求，对应路由 POST /api/chat。
//
// ── 请求生命周期 ──────────────────────────────────────────────────────
//
//	HTTP 请求
//	  │
//	  ├─ 1. 读取会话历史（mem）
//	  ├─ 2. 组装 UserMessage（携带 Query + History）
//	  ├─ 3. 构建 Chat Agent（DAG Pipeline）
//	  ├─ 4. 同步 Invoke 推理（内部执行 RAG 检索 + ReAct 循环）
//	  ├─ 5. 持久化本轮消息到记忆
//	  └─ 6. 返回 HTTP 响应
//
// ── 与流式接口的区别 ──────────────────────────────────────────────────
//
//	本接口使用 runner.Invoke（同步阻塞），等待 LLM 全量生成完毕后一次性返回结果。
//	流式接口（chat_v1_chat_stream.go）使用 runner.Stream，逐 Token 推送 SSE 事件，
//	适合前端实时打字机效果。两者共用同一个 Agent 逻辑，只是输出方式不同。
func (c *ControllerV1) Chat(ctx context.Context, req *v1.ChatReq) (res *v1.ChatRes, err error) {
	// id 是会话的唯一标识（由前端生成并随每次请求携带），
	// 用于从全局 SimpleMemoryMap 中定位该用户专属的历史记录，实现多用户会话隔离
	id := req.Id
	msg := req.Question

	// 组装本次推理的输入结构体，包含三个关键字段：
	//   ID      - 会话 ID，用于 Agent 内部追踪
	//   Query   - 本轮用户提问，将同时作为 RAG 检索词和 Prompt 的 {content} 填充
	//   History - 历史消息列表（滑动窗口最多 6 条），注入 Prompt 的 {history} 占位符，
	//             使 LLM 能理解"你之前说了什么"，实现多轮对话上下文感知
	userMessage := &chat_pipeline.UserMessage{
		ID:      id,
		Query:   msg,
		History: mem.GetSimpleMemory(id).GetMessages(),
	}

	// 构建 Chat Agent（Eino DAG Pipeline）：
	// 内部包含 InputToRag、MilvusRetriever、ChatTemplate、ReactAgent 四类节点，
	// 两条并行支路（RAG + 历史）汇聚后驱动 ReAct 循环推理。
	runner, err := chat_pipeline.BuildChatAgent(ctx)
	if err != nil {
		return nil, err
	}

	// 同步调用 Agent，阻塞直到 ReAct 循环结束并输出最终 *schema.Message。
	// compose.WithCallbacks 注入 LogCallback：
	//   - Eino 框架在每个节点执行前后触发 OnStart / OnEnd 回调
	//   - LogCallback 在回调中打印 "[view start/end]:[Component:Type:Name]" 及输入内容
	out, err := runner.Invoke(ctx, userMessage, compose.WithCallbacks(log_call_back.LogCallback(nil)))
	if err != nil {
		return nil, err
	}

	res = &v1.ChatRes{
		Answer: out.Content,
	}

	// 将本轮对话写回内存，维护多轮上下文：
	//   - schema.UserMessage(msg)         → role: "user"，记录用户的提问
	//   - schema.AssistantMessage(...)    → role: "assistant"，记录模型本轮的回答
	// 顺序必须严格保持 User → Assistant 交替，否则下轮请求注入历史时会破坏角色结构，
	// 导致模型混淆对话角色（LLM 对消息角色顺序敏感）。
	mem.GetSimpleMemory(id).SetMessages(schema.UserMessage(msg))
	mem.GetSimpleMemory(id).SetMessages(schema.AssistantMessage(out.Content, nil))

	return res, nil
}
