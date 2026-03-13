// chat.go 对话核心业务逻辑：同步对话与流式对话。
package chatsvc

import (
	"context"
	"errors"
	"io"

	"Fo-Sentinel-Agent/internal/ai/agent/chat_pipeline"
	"Fo-Sentinel-Agent/utility/log_call_back"

	"github.com/cloudwego/eino/compose"
	"github.com/gogf/gf/v2/frame/g"
)

// Chat 执行同步阻塞式对话：加载记忆 → 调用 Agent → 保存记忆，返回完整回答。
func Chat(ctx context.Context, sessionId, question string) (string, error) {
	memory, history := LoadMemory(ctx, sessionId)

	runner, err := chat_pipeline.GetChatAgent(ctx)
	if err != nil {
		return "", err
	}
	out, err := runner.Invoke(ctx, &chat_pipeline.UserMessage{
		ID:      sessionId,
		Query:   question,
		History: history,
	}, compose.WithCallbacks(log_call_back.LogCallback(nil)))
	if err != nil {
		return "", err
	}
	SaveMemory(ctx, sessionId, memory, question, out.Content)
	g.Log().Debugf(ctx, "[Chat] session_id=%s answer_len=%d", sessionId, len(out.Content))
	return out.Content, nil
}

// StreamChat 执行流式对话：加载记忆 → 调用 Agent → 逐 chunk 回调 → 保存记忆。
// onChunk 由调用方提供，负责将 chunk 写入 SSE 或其他输出流。
// 返回 nil 表示流正常结束（EOF），返回 error 表示中途失败。
func StreamChat(ctx context.Context, sessionId, question string, onChunk func(chunk string)) error {
	memory, history := LoadMemory(ctx, sessionId)

	runner, err := chat_pipeline.GetChatAgent(ctx)
	if err != nil {
		return err
	}
	sr, err := runner.Stream(ctx, &chat_pipeline.UserMessage{
		ID:      sessionId,
		Query:   question,
		History: history,
	}, compose.WithCallbacks(log_call_back.LogCallback(nil)))
	if err != nil {
		return err
	}
	defer sr.Close()

	// fullResponse 用于流结束后统一写入记忆，defer 确保异常路径也能触发
	var fullResponse string
	defer func() {
		SaveMemory(ctx, sessionId, memory, question, fullResponse)
	}()

	for {
		chunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if chunk != nil && chunk.Content != "" {
			fullResponse += chunk.Content
			onChunk(chunk.Content)
		}
	}
}
