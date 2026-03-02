package rediscli

import (
	"context"
	"fmt"
	"sync"

	"github.com/gogf/gf/v2/frame/g"
	goredis "github.com/redis/go-redis/v9"
)

var (
	globalClient *goredis.Client
	once         sync.Once
	initErr      error
)

// GetRedisClient 返回全局单例 Redis 客户端（懒初始化，线程安全）。
func GetRedisClient(ctx context.Context) (*goredis.Client, error) {
	once.Do(func() {
		addr, err := g.Cfg().Get(ctx, "semantic_cache.redis_addr")
		if err != nil {
			initErr = fmt.Errorf("read semantic_cache.redis_addr: %w", err)
			return
		}
		password, _ := g.Cfg().Get(ctx, "semantic_cache.redis_password")
		db, _ := g.Cfg().Get(ctx, "semantic_cache.redis_db")

		globalClient = goredis.NewClient(&goredis.Options{
			Addr:     addr.String(),
			Password: password.String(),
			DB:       db.Int(),
		})

		// 连通性检查：启动时立即发现配置错误，而非在首次查询时才暴露问题。
		if err := globalClient.Ping(ctx).Err(); err != nil {
			initErr = fmt.Errorf("redis ping failed (addr=%s): %w", addr.String(), err)
			globalClient = nil
			return
		}
	})
	return globalClient, initErr
}
