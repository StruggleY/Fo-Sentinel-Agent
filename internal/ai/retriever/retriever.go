package retriever

import (
	"context"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	einoretriever "github.com/cloudwego/eino/components/retriever"

	"Fo-Sentinel-Agent/internal/ai/cache"
	"Fo-Sentinel-Agent/internal/ai/embedder"
	"Fo-Sentinel-Agent/utility/client"
	"Fo-Sentinel-Agent/utility/rediscli"
)

var (
	globalRetriever einoretriever.Retriever
	once            sync.Once
	initErr         error
)

// GetRetriever 返回全局单例 cache.Retriever（懒初始化，线程安全）。
//
// 进程生命周期内只执行一次初始化：建立 Milvus TCP 连接、建立 Redis 连接、
// 创建 Embedding 客户端、从配置文件读取语义缓存参数。
//
// 语义缓存参数从 config.yaml semantic_cache 节读取，常量作为缺省兜底值：
//   - max_size   cache.DefaultMaxSize  = 200 条
//   - ttl_hours  cache.DefaultTTL      = 24 小时
//   - threshold  cache.DefaultThreshold = 0.85
//   - key_prefix cache.DefaultKeyPrefix = "rag:cache"
//   - topK       cache.DefaultTopK     = 1
func GetRetriever(ctx context.Context) (einoretriever.Retriever, error) {
	once.Do(func() {
		// 复用全局单例 Milvus 客户端，避免重复建立 TCP 连接。
		milvusCli, err := client.GetMilvusClient(ctx)
		if err != nil {
			initErr = err
			return
		}
		// DoubaoEmbedding 创建 DashScope Embedding 客户端，
		// 注入 cache.Retriever 后用于：缓存比对时向量化查询（单次调用），以及缓存未命中时向量化后直接搜索。
		eb, err := embedder.DoubaoEmbedding(ctx)
		if err != nil {
			initErr = err
			return
		}
		// Redis 单例客户端，语义缓存的持久化存储。
		redisCli, err := rediscli.GetRedisClient(ctx)
		if err != nil {
			initErr = err
			return
		}
		// cache.Retriever 持有 milvusCli 和 eb，全程只调用一次 Embedding API：
		// 命中缓存 → 直接返回；未命中 → 用已有向量直接调 Milvus Search，不再二次 Embed。
		globalRetriever = cache.New(milvusCli, eb, redisCli, readCacheConfig(ctx))
	})
	return globalRetriever, initErr
}

// readCacheConfig 从配置文件 semantic_cache 节读取缓存参数，缺失时回落到默认常量。
func readCacheConfig(ctx context.Context) cache.Config {
	ttlHours, _ := g.Cfg().Get(ctx, "semantic_cache.ttl_hours")
	threshold, _ := g.Cfg().Get(ctx, "semantic_cache.threshold")
	keyPrefix, _ := g.Cfg().Get(ctx, "semantic_cache.key_prefix")

	cfg := cache.Config{
		TTL:       time.Duration(ttlHours.Int64()) * time.Hour,
		Threshold: threshold.Float64(),
		KeyPrefix: keyPrefix.String(),
		TopK:      cache.DefaultTopK,
	}
	if cfg.TTL <= 0 {
		cfg.TTL = cache.DefaultTTL
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = cache.DefaultThreshold
	}
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = cache.DefaultKeyPrefix
	}
	return cfg
}

// WarmUp 在应用启动阶段主动触发单例初始化，将冷启动延迟提前到服务就绪阶段。
func WarmUp(ctx context.Context) error {
	_, err := GetRetriever(ctx)
	return err
}
