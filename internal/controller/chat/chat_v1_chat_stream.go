package chat

import (
	v1 "Fo-Sentinel-Agent/api/chat/v1"
	"Fo-Sentinel-Agent/internal/ai/agent/chat_pipeline"
	"Fo-Sentinel-Agent/internal/ai/cache"
	"Fo-Sentinel-Agent/utility/log_call_back"
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// ChatStream 处理流式对话请求，对应路由 POST /api/chat_stream。
// 底层使用 SSE（Server-Sent Events）协议向客户端逐 Token 推送 LLM 输出。
//
// 与同步接口 Chat 的区别：
//   - Chat      使用 runner.Invoke，阻塞等待 LLM 全量生成完毕后一次性返回
//   - ChatStream 使用 runner.Stream，LLM 每生成一个 Token 就立即推送给前端，响应更实时
//
// 整体流程：建立 SSE 连接 → 构建 Agent → 流式推理 → 逐 chunk 推送 → 结束后写回记忆
func (c *ControllerV1) ChatStream(ctx context.Context, req *v1.ChatStreamReq) (res *v1.ChatStreamRes, err error) {
	id := req.Id
	msg := req.Question

	// 将 client_id 注入 ctx，SSE Service 内部凭此 key 识别当前连接归属哪个会话
	ctx = context.WithValue(ctx, "client_id", req.Id)

	// 建立 SSE 长连接：设置响应头 Content-Type: text/event-stream，并立即发送 connected 事件。
	// SSE 是基于 HTTP 的单向推送协议，服务端可持续向客户端写数据，连接在函数返回前不会关闭。
	client, err := c.service.Create(ctx, g.RequestFromCtx(ctx))
	if err != nil {
		return nil, err
	}

	// 启动心跳
	// 每 15s 发送一次 SSE 注释行（`: keepalive\n\n`），浏览器忽略但保持 TCP 活跃
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat() // 函数退出时停止心跳 goroutine
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				client.SendHeartbeat()
			case <-heartbeatCtx.Done():
				return
			}
		}
	}()

	// 获取对话历史（两级缓存架构 + 长期摘要）
	// 优先从进程内存读取（热数据），若为空则从 Redis 冷启动恢复
	memory := cache.GetChatMemory(id)
	recent := memory.GetRecentMessages()
	summary := memory.GetLongTermSummary()

	// 记录当前内存中的对话状态
	g.Log().Debugf(ctx, "[ChatStream] load memory, session_id=%s, recent_len=%d, has_summary=%t",
		id, len(recent), summary != "")

	if len(recent) == 0 && summary == "" {
		// 内存为空，尝试从 Redis 恢复（服务重启后的冷启动场景）
		if r, s, err := cache.GetChatState(ctx, id); err == nil && (len(r) > 0 || s != "") {
			// 将 Redis 中的历史状态一次性写回进程内存（后续请求可直接从内存读取，无需再查 Redis）
			memory.SetState(r, s)
			recent = r
			summary = s

			g.Log().Infof(ctx, "[ChatStream] restored history from redis, session_id=%s, recent_len=%d, has_summary=%t",
				id, len(recent), summary != "")
		} else if err != nil {
			g.Log().Warningf(ctx, "[ChatStream] failed to load history from redis, session_id=%s, error=%v", id, err)
		}
	}

	// 构建历史消息
	history := cache.BuildHistoryWithSummary(recent, summary)

	// 组装本次推理输入，携带历史消息使模型具备多轮对话上下文感知
	userMessage := &chat_pipeline.UserMessage{
		ID:      id,
		Query:   msg,
		History: history,
	}

	// 构建 Chat Agent（RAG + ReAct DAG Pipeline）
	runner, err := chat_pipeline.GetChatAgent(ctx)

	// runner.Stream 以流式模式调用 Agent，返回 StreamReader。
	// 内部 ReAct 循环仍会完整执行（工具调用、多步推理），
	// 区别在于最终 LLM 的回答内容会被拆分为多个 chunk 逐步输出，而非等全量生成完再返回。
	sr, err := runner.Stream(ctx, userMessage, compose.WithCallbacks(log_call_back.LogCallback(nil)))
	if err != nil {
		client.SendToClient("error", err.Error())
		return nil, err
	}
	// 确保函数退出时关闭 StreamReader，释放底层连接和 goroutine 资源
	defer sr.Close()

	// fullResponse 拼接所有 chunk，用于函数结束后完整写入对话记忆。
	// 不能在 for 循环中逐 chunk 写入记忆，因为模型回复在流式传输过程中是不完整的。
	var fullResponse strings.Builder

	// 只要已收到部分或全部回复，就将本轮完整对话写入记忆，维持多轮上下文的连贯性。
	defer func() {
		completeResponse := fullResponse.String()
		if completeResponse != "" {
			memory.SetMessages(schema.UserMessage(msg))
			memory.SetMessages(schema.AssistantMessage(completeResponse, nil))

			g.Log().Debugf(ctx, "[ChatStream] append round messages, session_id=%s, user_len=%d, assistant_len=%d",
				id, len(msg), len(completeResponse))
			// 异步持久化到 Redis（不阻塞主链路）
			go func() {
				bgCtx := context.Background()
				if err := cache.SetRecentState(bgCtx, id, memory.GetRecentMessages()); err != nil {
					g.Log().Errorf(bgCtx, "failed to persist recent state to redis, session_id=%s, error=%v", id, err)
				} else {
					g.Log().Infof(bgCtx, "[ChatStream] persisted recent state to redis, session_id=%s, recent_len=%d, has_summary=%t",
						id, len(memory.GetRecentMessages()), memory.GetLongTermSummary() != "")
				}
			}()
		}
	}()

	// 持续从 StreamReader 读取 chunk，直到流结束或发生错误
	for {
		chunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			// io.EOF 表示流正常结束，LLM 已完成本轮全部输出
			client.SendToClient("done", "Stream completed")
			return &v1.ChatStreamRes{}, nil
		}
		if err != nil {
			// 非 EOF 错误（如网络中断、LLM 服务异常），通知客户端并终止
			client.SendToClient("error", err.Error())
			return &v1.ChatStreamRes{}, nil
		}
		// 将当前 chunk 追加到完整回复缓冲，并通过 SSE 实时推送给前端
		// SSE 事件格式：event: message\ndata: <chunk.Content>\n\n
		fullResponse.WriteString(chunk.Content)
		client.SendToClient("message", chunk.Content)
	}
}
