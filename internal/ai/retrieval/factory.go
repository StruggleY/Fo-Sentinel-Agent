// Package retrieval 工厂函数：全局单例初始化与获取
//
// 设计：使用 sync.Once 保证线程安全的懒初始化
// 优势：应用启动时一次性初始化，避免重复创建连接
package retrieval

import (
	"context"
	"sync"

	"Fo-Sentinel-Agent/internal/ai/embedder"
	"Fo-Sentinel-Agent/internal/dao/milvus"

	"github.com/cloudwego/eino/components/embedding"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/gogf/gf/v2/frame/g"
	milvuscli "github.com/milvus-io/milvus-sdk-go/v2/client"
	goredis "github.com/redis/go-redis/v9"
)

var (
	once sync.Once

	globalEmbedder embedding.Embedder
	globalMilvus   milvuscli.Client
	globalRedis    *goredis.Client
	globalConfig   Config
)

// init 在包导入时自动执行，初始化全局单例
func init() {
	once.Do(func() {
		ctx := context.Background()

		// 初始化 Embedder（DashScope text-embedding-v4）
		eb, err := embedder.NewDenseEmbedder(ctx)
		if err != nil {
			g.Log().Fatalf(ctx, "[Retrieval] 初始化 Embedder 失败: %v", err)
		}
		globalEmbedder = eb

		// 初始化 Milvus
		cli, err := milvus.GetClient(ctx)
		if err != nil {
			g.Log().Fatalf(ctx, "[Retrieval] 初始化 Milvus 失败: %v", err)
		}
		globalMilvus = cli

		// 初始化 Redis（带连接验证）
		redisCfg, _ := g.Cfg().Get(ctx, "redis")
		globalRedis = goredis.NewClient(&goredis.Options{
			Addr: redisCfg.MapStrStr()["addr"],
		})
		// 验证 Redis 连接
		if err := globalRedis.Ping(ctx).Err(); err != nil {
			g.Log().Warningf(ctx, "[Retrieval] Redis 连接失败，缓存功能将不可用: %v", err)
		}

		// 加载配置
		globalConfig = LoadConfig(ctx)
	})
}

// GetRetriever 获取通用检索器（全分区）
// 用途：Chat Agent 通用问答，同时检索事件和文档
func GetRetriever() einoretriever.Retriever {
	return New(globalMilvus, globalEmbedder, globalRedis, globalConfig)
}

// GetEventsRetriever 获取事件检索器（events 分区，混合检索）
// 用途：search_similar_events 工具，精确匹配 CVE 编号等
func GetEventsRetriever() einoretriever.Retriever {
	cfg := globalConfig
	cfg.Partition = "events"
	return New(globalMilvus, globalEmbedder, globalRedis, cfg)
}

// GetDocumentsRetriever 获取文档检索器（documents 分区，混合检索）
// 用途：query_internal_docs 工具，检索知识库文档
func GetDocumentsRetriever() einoretriever.Retriever {
	cfg := globalConfig
	cfg.Partition = "documents"
	return New(globalMilvus, globalEmbedder, globalRedis, cfg)
}

// WarmUp 预热检索器，触发懒初始化
// 用途：应用启动时调用，将冷启动延迟提前到服务就绪阶段
func WarmUp(ctx context.Context) error {
	_ = GetRetriever()
	return nil
}
