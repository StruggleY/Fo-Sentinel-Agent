package cache

import (
	aitrace "Fo-Sentinel-Agent/internal/ai/trace"
	"Fo-Sentinel-Agent/utility/client"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	goredis "github.com/redis/go-redis/v9"
)

var (
	sessionKeyPrefix string
	sessionTTL       time.Duration
	configLoaded     bool
)

func buildRecentKey(sessionID string) string {
	return fmt.Sprintf("%s:%s:recent", sessionKeyPrefix, sessionID)
}

func buildSummaryKey(sessionID string) string {
	return fmt.Sprintf("%s:%s:summary", sessionKeyPrefix, sessionID)
}

// LoadSession 从 Redis 加载指定会话的记忆状态。
//
// 返回值：
//   - recent：最近的原始消息列表（不包含长期摘要提示）
//   - summary：长期摘要内容
//   - error：Redis 连接失败等网络错误（cache miss 不算错误，返回空列表和空摘要）
//
// Key 设计：
//   - {key_prefix}:{sessionID}:recent  → 最近的消息列表（JSON 序列化 []*schema.Message）
//   - {key_prefix}:{sessionID}:summary → 长期摘要字符串
func LoadSession(ctx context.Context, sessionID string) (recent []*schema.Message, summary string, err error) {
	spanCtx, spanID := aitrace.StartSpan(ctx, aitrace.NodeTypeCache, "SESSION_LOAD")
	recent, summary, err = loadSessionCore(spanCtx, sessionID)
	aitrace.FinishSpan(spanCtx, spanID, err, map[string]any{
		"op":           "LOAD",
		"recent_count": len(recent),
		"has_summary":  summary != "",
	})
	return recent, summary, err
}

// loadSessionCore 执行实际的 Redis 读取逻辑
func loadSessionCore(ctx context.Context, sessionID string) ([]*schema.Message, string, error) {
	loadChatConfig(ctx)

	redisCli, err := client.GetRedisClient(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("get redis client: %w", err)
	}

	var recent []*schema.Message
	// 读取最近消息
	recentKey := buildRecentKey(sessionID)
	data, err := redisCli.Get(ctx, recentKey).Bytes()
	if errors.Is(err, goredis.Nil) {
		// cache miss：该会话从未存储或 TTL 已过期，返回空列表和空摘要（非错误）
		recent = []*schema.Message{}
	} else if err != nil {
		return nil, "", fmt.Errorf("redis GET %s: %w", recentKey, err)
	} else {
		if err := json.Unmarshal(data, &recent); err != nil {
			return nil, "", fmt.Errorf("unmarshal recent messages: %w", err)
		}
	}

	// 读取长期摘要
	var summary string
	summaryKey := buildSummaryKey(sessionID)
	summaryVal, err := redisCli.Get(ctx, summaryKey).Result()
	if errors.Is(err, goredis.Nil) {
		// 没有摘要也不算错误
		summary = ""
	} else if err != nil {
		return nil, "", fmt.Errorf("redis GET %s: %w", summaryKey, err)
	} else {
		summary = summaryVal
	}

	return recent, summary, nil
}

// SaveSession 将会话记忆状态（最近消息 + 长期摘要）保存到 Redis。
// 内部委托给 SaveRecentMessages / SaveSummary，方便调用方一次性写入。
func SaveSession(ctx context.Context, sessionID string, recent []*schema.Message, summary string) error {
	// 如果最近消息和摘要都为空，则不进行写入，避免无效 key 占用
	if len(recent) == 0 && summary == "" {
		g.Log().Debugf(ctx, "[Session] 数据为空，跳过保存 | session=%s", sessionID)
		return nil
	}

	spanCtx, spanID := aitrace.StartSpan(ctx, aitrace.NodeTypeCache, "SESSION_SAVE")
	err := saveSessionCore(spanCtx, sessionID, recent, summary)
	aitrace.FinishSpan(spanCtx, spanID, err, map[string]any{
		"op":           "SAVE",
		"recent_count": len(recent),
	})
	return err
}

// saveSessionCore 执行实际的 Redis 写入逻辑
func saveSessionCore(ctx context.Context, sessionID string, recent []*schema.Message, summary string) error {
	if err := SaveRecentMessages(ctx, sessionID, recent); err != nil {
		return err
	}
	if err := SaveSummary(ctx, sessionID, summary); err != nil {
		return err
	}
	return nil
}

// shortID 取 sessionID 前 8 位，用于 span 名称防止过长
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// SaveRecentMessages 仅将最近消息列表保存到 Redis。
// 空的 recent 会被跳过，不写入 Redis。
func SaveRecentMessages(ctx context.Context, sessionID string, recent []*schema.Message) error {
	loadChatConfig(ctx)

	if len(recent) == 0 {
		g.Log().Debugf(ctx, "[Session] 消息列表为空，跳过保存 | session=%s", sessionID)
		return nil
	}

	redisCli, err := client.GetRedisClient(ctx)
	if err != nil {
		return fmt.Errorf("get redis client: %w", err)
	}

	data, err := json.Marshal(recent)
	if err != nil {
		return fmt.Errorf("marshal recent messages: %w", err)
	}

	recentKey := buildRecentKey(sessionID)
	if err := redisCli.Set(ctx, recentKey, data, sessionTTL).Err(); err != nil {
		return fmt.Errorf("redis SET %s: %w", recentKey, err)
	}
	return nil
}

// SaveSummary 仅将长期摘要内容保存到 Redis。
// 空摘要会被跳过，不写入 Redis。
func SaveSummary(ctx context.Context, sessionID string, summary string) error {
	loadChatConfig(ctx)

	if summary == "" {
		g.Log().Debugf(ctx, "[Session] 摘要为空，跳过保存 | session=%s", sessionID)
		return nil
	}

	redisCli, err := client.GetRedisClient(ctx)
	if err != nil {
		return fmt.Errorf("get redis client: %w", err)
	}

	summaryKey := buildSummaryKey(sessionID)
	if err := redisCli.Set(ctx, summaryKey, summary, sessionTTL).Err(); err != nil {
		return fmt.Errorf("redis SET %s: %w", summaryKey, err)
	}
	return nil
}

// loadChatConfig 从配置文件加载对话缓存配置（懒加载，首次调用时执行）
func loadChatConfig(ctx context.Context) {
	if configLoaded {
		return
	}

	ttlHours := g.Cfg().MustGet(ctx, "redis.chat_cache.ttl").Int64()
	keyPrefix := g.Cfg().MustGet(ctx, "redis.chat_cache.key_prefix").String()

	// TTL 配置（小时转换为 time.Duration）
	sessionTTL = time.Duration(ttlHours) * time.Hour

	// Key 前缀配置
	sessionKeyPrefix = keyPrefix

	configLoaded = true
}
