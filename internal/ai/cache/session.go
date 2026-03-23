package cache

import (
	aitrace "Fo-Sentinel-Agent/internal/ai/trace"
	redisdao "Fo-Sentinel-Agent/internal/dao/redis"
	"context"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// LoadSession 从 Redis 加载指定会话的记忆状态
func LoadSession(ctx context.Context, sessionID string) (recent []*schema.Message, summary string, err error) {
	spanCtx, spanID := aitrace.StartSpan(ctx, aitrace.NodeTypeCache, "SESSION_LOAD")
	recent, summary, err = redisdao.LoadSession(spanCtx, sessionID)
	aitrace.FinishSpan(spanCtx, spanID, err, map[string]any{
		"op":           "LOAD",
		"recent_count": len(recent),
		"has_summary":  summary != "",
	})
	return recent, summary, err
}

// SaveSession 将会话记忆状态保存到 Redis
func SaveSession(ctx context.Context, sessionID string, recent []*schema.Message, summary string) error {
	if len(recent) == 0 && summary == "" {
		g.Log().Debugf(ctx, "[Session] 数据为空，跳过保存 | session=%s", sessionID)
		return nil
	}

	spanCtx, spanID := aitrace.StartSpan(ctx, aitrace.NodeTypeCache, "SESSION_SAVE")
	err := redisdao.SaveSession(spanCtx, sessionID, recent, summary)
	aitrace.FinishSpan(spanCtx, spanID, err, map[string]any{
		"op":           "SAVE",
		"recent_count": len(recent),
	})
	return err
}

// SaveRecentMessages 仅保存最近消息列表
func SaveRecentMessages(ctx context.Context, sessionID string, recent []*schema.Message) error {
	if len(recent) == 0 {
		g.Log().Debugf(ctx, "[Session] 消息列表为空，跳过保存 | session=%s", sessionID)
		return nil
	}
	return redisdao.SaveSession(ctx, sessionID, recent, "")
}

// SaveSummary 仅保存长期摘要
func SaveSummary(ctx context.Context, sessionID string, summary string) error {
	if summary == "" {
		g.Log().Debugf(ctx, "[Session] 摘要为空，跳过保存 | session=%s", sessionID)
		return nil
	}
	return redisdao.SaveSession(ctx, sessionID, nil, summary)
}
