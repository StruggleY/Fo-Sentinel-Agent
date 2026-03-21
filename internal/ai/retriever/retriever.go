package retriever

import (
	"context"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"

	"Fo-Sentinel-Agent/internal/ai/cache"
	"Fo-Sentinel-Agent/internal/ai/embedder"
	milvus "Fo-Sentinel-Agent/internal/dao/milvus"
	dao "Fo-Sentinel-Agent/internal/dao/mysql"
	"Fo-Sentinel-Agent/utility/client"
)

const (
	DefaultTopK      = 5 // 每次从 Milvus 召回的候选文档数（扩大候选池）
	DefaultFinalTopK = 3 // 返回给 LLM 的最终文档数（Rerank/截断后）
)

var (
	// 全分区稠密检索（Chat Agent 通用问答）
	globalRetriever einoretriever.Retriever
	once            sync.Once
	initErr         error

	// events 分区混合检索（search_similar_events 工具）
	eventsRetriever einoretriever.Retriever
	eventsOnce      sync.Once
	eventsInitErr   error

	// documents 分区混合检索（query_internal_docs 工具）
	docsRetriever einoretriever.Retriever
	docsOnce      sync.Once
	docsInitErr   error
)

// GetRetriever 返回全局单例 cache.Retriever（全分区稠密检索，懒初始化，线程安全）。
// 供 Chat Agent 通用问答使用（同时检索事件和文档）。
func GetRetriever(ctx context.Context) (einoretriever.Retriever, error) {
	once.Do(func() {
		globalRetriever, initErr = newRetriever(ctx, "", false)
	})
	return globalRetriever, initErr
}

// GetEventsRetriever 返回安全事件分区的混合检索单例（search_similar_events 专用）。
func GetEventsRetriever(ctx context.Context) (einoretriever.Retriever, error) {
	eventsOnce.Do(func() {
		eventsRetriever, eventsInitErr = newRetriever(ctx, milvus.PartitionEvents, true)
	})
	return eventsRetriever, eventsInitErr
}

// GetDocumentsRetriever 返回知识文档分区的混合检索单例（query_internal_docs 专用）。
func GetDocumentsRetriever(ctx context.Context) (einoretriever.Retriever, error) {
	docsOnce.Do(func() {
		docsRetriever, docsInitErr = newRetriever(ctx, milvus.PartitionDocuments, true)
	})
	return docsRetriever, docsInitErr
}

// newRetriever 创建 Retriever 实例（内部函数）。
// partition 为空时检索全部分区；useHybrid=true 时启用 BM25 + 语义向量混合检索。
func newRetriever(ctx context.Context, partition string, useHybrid bool) (einoretriever.Retriever, error) {
	milvusCli, err := milvus.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	eb, err := embedder.DoubaoEmbedding(ctx)
	if err != nil {
		return nil, err
	}
	redisCli, err := client.GetRedisClient(ctx)
	if err != nil {
		return nil, err
	}
	cfg := readCacheConfig(ctx)
	return cache.NewWithPartition(milvusCli, eb, redisCli, cfg, partition, useHybrid), nil
}

// readCacheConfig 从配置文件 redis.semantic_cache 节读取缓存参数，缺失时回落到默认常量。
func readCacheConfig(ctx context.Context) cache.Config {
	ttlHours, _ := g.Cfg().Get(ctx, "redis.semantic_cache.ttl")
	threshold, _ := g.Cfg().Get(ctx, "redis.semantic_cache.threshold")
	keyPrefix, _ := g.Cfg().Get(ctx, "redis.semantic_cache.key_prefix")
	topK, _ := g.Cfg().Get(ctx, "redis.semantic_cache.topk")
	finalTopK, _ := g.Cfg().Get(ctx, "redis.semantic_cache.final_topk")
	minScore, _ := g.Cfg().Get(ctx, "redis.semantic_cache.min_score")

	cfg := cache.Config{
		TTL:       time.Duration(ttlHours.Int64()) * time.Hour,
		Threshold: threshold.Float64(),
		KeyPrefix: keyPrefix.String(),
		TopK:      topK.Int(),
		FinalTopK: finalTopK.Int(),
		MinScore:  minScore.Float64(),
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
	if cfg.TopK <= 0 {
		cfg.TopK = DefaultTopK
	}
	if cfg.FinalTopK <= 0 {
		cfg.FinalTopK = DefaultFinalTopK
	}
	if cfg.MinScore <= 0 {
		cfg.MinScore = cache.DefaultMinScore
	}
	return cfg
}

// WarmUp 在应用启动阶段主动触发单例初始化，将冷启动延迟提前到服务就绪阶段。
func WarmUp(ctx context.Context) error {
	_, err := GetRetriever(ctx)
	return err
}

// FilterDisabledDocs 过滤掉已禁用文档所属的分块。
// 从 Milvus 返回的文档 schema.Document 中，通过 metadata 字段取出 doc_id，
// 批量查询 MySQL knowledge_documents.enabled，过滤掉 enabled=false 的文档。
// 若 MySQL 不可用，则降级返回原始文档列表（不影响检索主流程）。
func FilterDisabledDocs(ctx context.Context, docs []*schema.Document) []*schema.Document {
	if len(docs) == 0 {
		return docs
	}

	// 收集 doc_id 集合
	docIDSet := make(map[string]struct{}, len(docs))
	for _, d := range docs {
		if docID, ok := d.MetaData["doc_id"]; ok {
			if s, ok := docID.(string); ok && s != "" {
				docIDSet[s] = struct{}{}
			}
		}
	}
	if len(docIDSet) == 0 {
		return docs
	}

	// 批量查询 enabled 状态
	docIDs := make([]string, 0, len(docIDSet))
	for id := range docIDSet {
		docIDs = append(docIDs, id)
	}

	db, err := dao.DB(ctx)
	if err != nil {
		// MySQL 不可用时降级，不影响 RAG 主流程
		g.Log().Warningf(ctx, "[retriever] FilterDisabledDocs: DB unavailable, skip filter: %v", err)
		return docs
	}

	var disabledDocs []dao.KnowledgeDocument
	db.Select("id").Where("id IN ? AND enabled = false", docIDs).Find(&disabledDocs)

	if len(disabledDocs) == 0 {
		return docs
	}

	// 构建禁用 doc_id 集合
	disabledSet := make(map[string]struct{}, len(disabledDocs))
	for _, d := range disabledDocs {
		disabledSet[d.ID] = struct{}{}
	}

	// 过滤结果
	filtered := make([]*schema.Document, 0, len(docs))
	for _, d := range docs {
		docID, _ := d.MetaData["doc_id"].(string)
		if _, disabled := disabledSet[docID]; !disabled {
			filtered = append(filtered, d)
		}
	}
	g.Log().Debugf(ctx, "[retriever] FilterDisabledDocs: 原始=%d 过滤后=%d（禁用文档=%d）",
		len(docs), len(filtered), len(disabledDocs))
	return filtered
}
