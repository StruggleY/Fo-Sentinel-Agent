// Package chatsvc 提供对话业务逻辑。
// 封装会话记忆的加载与持久化，供 chat controller 调用。
package chatsvc

import (
	"Fo-Sentinel-Agent/internal/ai/cache"
	"context"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// LoadMemory 加载会话历史：优先从进程内存读取，为空时从 Redis 冷启动恢复。
// 返回 memory 实例（供后续写回）和拼接好的历史消息列表（含长期摘要）。
func LoadMemory(ctx context.Context, id string) (*cache.SessionMemory, []*schema.Message) {
	memory := cache.GetSessionMemory(id)
	recent := memory.GetRecentMessages()
	summary := memory.GetLongTermSummary()

	g.Log().Debugf(ctx, "[Memory] load, session_id=%s, recent_len=%d, has_summary=%t", id, len(recent), summary != "")

	if len(recent) == 0 && summary == "" {
		// 进程内存为空，尝试从 Redis 冷启动恢复（服务重启场景）
		if r, s, err := cache.LoadSession(ctx, id); err == nil && (len(r) > 0 || s != "") {
			memory.SetState(r, s)
			recent = r
			summary = s
			g.Log().Infof(ctx, "[Memory] restored from redis, session_id=%s, recent_len=%d, has_summary=%t", id, len(recent), summary != "")
		} else if err != nil {
			g.Log().Warningf(ctx, "[Memory] failed to load from redis, session_id=%s, error=%v", id, err)
		}
	}

	history := cache.BuildHistoryWithSummary(recent, summary)
	return memory, history
}

// SaveMemory 将本轮对话（用户消息 + 助手回复）写入进程内存，并异步保存到 Redis。
// assistantMsg 为空时跳过写入，避免无效记录。
func SaveMemory(ctx context.Context, id string, memory *cache.SessionMemory, userMsg, assistantMsg string) {
	if assistantMsg == "" {
		return
	}
	memory.SetMessages(schema.UserMessage(userMsg))
	memory.SetMessages(schema.AssistantMessage(assistantMsg, nil))

	go func() {
		bgCtx := context.Background()
		if err := cache.SaveRecentMessages(bgCtx, id, memory.GetRecentMessages()); err != nil {
			g.Log().Errorf(bgCtx, "[Memory] failed to save to redis, session_id=%s, error=%v", id, err)
		} else {
			g.Log().Infof(bgCtx, "[Memory] saved to redis, session_id=%s, recent_len=%d", id, len(memory.GetRecentMessages()))
		}
	}()
}
