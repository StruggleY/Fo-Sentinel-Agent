package chatsvc

import (
	"context"
	"fmt"

	"Fo-Sentinel-Agent/internal/ai/cache"
	redisdao "Fo-Sentinel-Agent/internal/dao/redis"

	"github.com/gogf/gf/v2/frame/g"
)

// RollbackSession 回溯会话到指定消息索引
func RollbackSession(ctx context.Context, sessionID string, targetIndex int) (int, error) {
	recent, summary, err := redisdao.LoadSession(ctx, sessionID)
	if err != nil {
		return 0, fmt.Errorf("load session: %w", err)
	}

	if len(recent) == 0 {
		return 0, fmt.Errorf("session not found or empty")
	}

	if targetIndex < 0 || targetIndex >= len(recent) {
		return 0, fmt.Errorf("invalid targetIndex: %d (valid range: 0-%d)", targetIndex, len(recent)-1)
	}

	removedCount := len(recent) - targetIndex - 1
	recent = recent[:targetIndex+1]

	// 更新 redis 中对话历史内容
	if err := redisdao.SaveSession(ctx, sessionID, recent, summary); err != nil {
		return 0, fmt.Errorf("save session: %w", err)
	}

	// 同步更新内存中的 SessionMemory
	mem := cache.GetSessionMemory(sessionID)
	mem.SetState(recent, summary)

	g.Log().Infof(ctx, "[Rollback] Rollback session %s to index %d, removed %d messages", sessionID, targetIndex, removedCount)
	return removedCount, nil
}
