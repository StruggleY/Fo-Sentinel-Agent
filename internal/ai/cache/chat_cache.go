package cache

import (
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

// GetChatState 从 Redis 获取指定会话的对话状态。
//
// 返回值：
//   - recent：最近的原始消息列表（不包含长期摘要提示）
//   - summary：长期摘要内容
//   - error：Redis 连接失败等网络错误（cache miss 不算错误，返回空列表和空摘要）
//
// Key 设计：
//   - {key_prefix}:{sessionID}:recent  → 最近的消息列表（JSON 序列化 []*schema.Message）
//   - {key_prefix}:{sessionID}:summary → 长期摘要字符串
func GetChatState(ctx context.Context, sessionID string) (recent []*schema.Message, summary string, err error) {
	loadChatConfig(ctx)

	redisCli, err := client.GetRedisClient(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("get redis client: %w", err)
	}

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

// SetChatState 将对话状态（最近消息 + 长期摘要）持久化到 Redis。
// 内部委托给 SetRecentState / SetSummaryState，方便调用方一次性写入。
func SetChatState(ctx context.Context, sessionID string, recent []*schema.Message, summary string) error {
	// 如果最近消息和摘要都为空，则不进行缓存写入，避免无效 key 占用
	if len(recent) == 0 && summary == "" {
		g.Log().Debugf(ctx, "[ChatCache] skip SetChatState for empty data, session_id=%s", sessionID)
		return nil
	}

	if err := SetRecentState(ctx, sessionID, recent); err != nil {
		return err
	}
	if err := SetSummaryState(ctx, sessionID, summary); err != nil {
		return err
	}
	return nil
}

// SetRecentState 仅将最近消息列表持久化到 Redis。
// 空的 recent 会被跳过，不写入缓存。
func SetRecentState(ctx context.Context, sessionID string, recent []*schema.Message) error {
	loadChatConfig(ctx)

	if len(recent) == 0 {
		g.Log().Debugf(ctx, "[ChatCache] skip SetRecentState for empty recent, session_id=%s", sessionID)
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

// SetSummaryState 仅将长期摘要内容持久化到 Redis。
// 空摘要会被跳过，不写入缓存。
func SetSummaryState(ctx context.Context, sessionID string, summary string) error {
	loadChatConfig(ctx)

	if summary == "" {
		g.Log().Debugf(ctx, "[ChatCache] skip SetSummaryState for empty summary, session_id=%s", sessionID)
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
