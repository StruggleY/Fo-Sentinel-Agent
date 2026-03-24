package cache

import (
	"context"
	"fmt"
	"time"

	aitrace "Fo-Sentinel-Agent/internal/ai/trace"
	redisdao "Fo-Sentinel-Agent/internal/dao/redis"

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

// SaveSessionWithRetry 带重试的会话保存（最多重试 3 次，指数退避）
func SaveSessionWithRetry(ctx context.Context, sessionID string, recent []*schema.Message, summary string) error {
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		err := SaveSession(ctx, sessionID, recent, summary)
		if err == nil {
			if i > 0 {
				g.Log().Infof(ctx, "[Session] 重试保存成功 | session=%s | retry=%d", sessionID, i)
			}
			return nil
		}

		lastErr = err
		if i < maxRetries-1 {
			backoff := time.Duration(1<<uint(i)) * time.Second // 1s, 2s, 4s
			g.Log().Warningf(ctx, "[Session] 保存失败，%v 后重试 | session=%s | retry=%d/%d | err=%v",
				backoff, sessionID, i+1, maxRetries, err)
			time.Sleep(backoff)
		}
	}

	g.Log().Errorf(ctx, "[Session] 保存失败，已达最大重试次数 | session=%s | err=%v", sessionID, lastErr)
	return fmt.Errorf("save session after %d retries: %w", maxRetries, lastErr)
}
