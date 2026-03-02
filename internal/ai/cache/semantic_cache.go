package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	milvuscli "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	goredis "github.com/redis/go-redis/v9"

	"Fo-Sentinel-Agent/utility/common"
)

const (
	DefaultTopK      = 1
	DefaultTTL       = 24 * time.Hour
	DefaultThreshold = 0.85
	DefaultKeyPrefix = "rag:cache"
)

type Config struct {
	TTL       time.Duration // 单条缓存有效期，到期由 Redis 自动删除
	Threshold float64       // 余弦相似度命中阈值，取值 (0,1]
	TopK      int           // Milvus 每次召回的文档数量
	KeyPrefix string        // Redis Key 前缀，用于业务隔离
}

// cacheEntry JSON 序列化后存入 Redis。
type cacheEntry struct {
	Embedding []float64          `json:"embedding"` // 查询向量，用于相似度比对
	Docs      []*schema.Document `json:"docs"`      // 对应的 Milvus 检索结果
}

// semanticCache 基于 Redis 的语义相似缓存：查询向量与历史向量做余弦相似度比对，
// 相似度 ≥ Threshold 时直接复用已有 Milvus 检索结果，跳过 Embedding API 和 ANN 搜索。
// Key 布局：
//   - {KeyPrefix}:{uuid} → JSON(cacheEntry)，带 TTL
//   - {KeyPrefix}:idx    → Set，member = uuid
//
// 淘汰：仅依赖 TTL，到期由 Redis 自动删除条目。
type semanticCache struct {
	client    *goredis.Client
	keyPrefix string
	ttl       time.Duration
	threshold float64
}

func newSemanticCache(client *goredis.Client, cfg Config) *semanticCache {
	return &semanticCache{
		client:    client,
		keyPrefix: cfg.KeyPrefix,
		ttl:       cfg.TTL,
		threshold: cfg.Threshold,
	}
}

func (sc *semanticCache) indexKey() string {
	return sc.keyPrefix + ":idx"
}

func (sc *semanticCache) entryKey(id string) string {
	return sc.keyPrefix + ":" + id
}

// get 遍历所有未过期条目，返回余弦相似度 ≥ threshold 的第一条结果及其相似度。
// GET 返回 Nil（TTL 过期）时顺带清理索引。
func (sc *semanticCache) get(ctx context.Context, vec []float64) (docs []*schema.Document, sim float64, hit bool) {
	ids, err := sc.client.SMembers(ctx, sc.indexKey()).Result()
	if err != nil || len(ids) == 0 {
		return nil, 0, false
	}

	var bestSim float64
	for _, id := range ids {
		data, err := sc.client.Get(ctx, sc.entryKey(id)).Bytes()
		if err != nil {
			if errors.Is(err, goredis.Nil) {
				sc.client.SRem(ctx, sc.indexKey(), id)
			}
			continue
		}

		var entry cacheEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			g.Log().Warningf(ctx, "[SemanticCache] 反序列化缓存条目失败 id=%s: %v", id, err)
			continue
		}

		s := cosineSim(vec, entry.Embedding)
		if s > bestSim {
			bestSim = s
		}
		if s >= sc.threshold {
			return entry.Docs, s, true
		}
	}
	g.Log().Debugf(ctx, "[SemanticCache] 全部条目未达阈值 | 最高相似度=%.4f | 阈值=%.2f | 已扫描条目数=%d", bestSim, sc.threshold, len(ids))
	return nil, 0, false
}

// set 写入新条目；写入前做去重检查，避免并发请求产生近似重复条目。
// Pipeline 原子写入条目和索引，减少一次网络往返。
func (sc *semanticCache) set(ctx context.Context, vec []float64, docs []*schema.Document) {
	if sc.hasSimilar(ctx, vec) {
		g.Log().Debugf(ctx, "semantic_cache.set: skip, similar entry already exists")
		return
	}

	entry := cacheEntry{Embedding: vec, Docs: docs}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	id := uuid.New().String()

	pipe := sc.client.Pipeline()
	pipe.Set(ctx, sc.entryKey(id), data, sc.ttl)
	pipe.SAdd(ctx, sc.indexKey(), id)
	pipe.Exec(ctx)
	g.Log().Debugf(ctx, "[SemanticCache] 写入新缓存条目 id=%s", id)
}

// hasSimilar 检查缓存中是否已存在与 vec 相似度 >= threshold 的条目，
// 用于在 set 写入前做去重，防止 get→miss 与 set 之间的并发窗口产生近似重复。
func (sc *semanticCache) hasSimilar(ctx context.Context, vec []float64) bool {
	ids, err := sc.client.SMembers(ctx, sc.indexKey()).Result()
	if err != nil || len(ids) == 0 {
		return false
	}
	for _, id := range ids {
		data, err := sc.client.Get(ctx, sc.entryKey(id)).Bytes()
		if err != nil {
			if errors.Is(err, goredis.Nil) {
				sc.client.SRem(ctx, sc.indexKey(), id)
			}
			continue
		}
		var entry cacheEntry
		if json.Unmarshal(data, &entry) != nil {
			continue
		}
		if cosineSim(vec, entry.Embedding) >= sc.threshold {
			return true
		}
	}
	return false
}

// cosineSim 计算余弦相似度：dot(a,b) / (‖a‖₂·‖b‖₂)，返回值 [-1, 1]。
func cosineSim(a, b []float64) float64 {
	var dot, sumSqA, sumSqB float64
	for i := range a {
		dot += a[i] * b[i]
		sumSqA += a[i] * a[i]
		sumSqB += b[i] * b[i]
	}
	denom := math.Sqrt(sumSqA) * math.Sqrt(sumSqB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// Retriever 实现 einoretriever.Retriever，全程只调用一次 Embedding API：
// Embed → 缓存命中则直接返回；未命中则用已有向量直接调 Milvus Search，写入缓存后返回。
// 绕过 eino milvus.Retriever 避免内部再次 Embed 导致双重付费。
type Retriever struct {
	milvusCli milvuscli.Client
	embedder  embedding.Embedder
	cache     *semanticCache
	topK      int
}

// New 创建 Retriever，TopK ≤ 0 时回落到 DefaultTopK。
func New(cli milvuscli.Client, eb embedding.Embedder, redisCli *goredis.Client, cfg Config) *Retriever {
	topK := cfg.TopK
	if topK <= 0 {
		topK = DefaultTopK
	}
	return &Retriever{
		milvusCli: cli,
		embedder:  eb,
		cache:     newSemanticCache(redisCli, cfg),
		topK:      topK,
	}
}

// Retrieve 实现 einoretriever.Retriever 接口。
func (c *Retriever) Retrieve(ctx context.Context, query string, opts ...einoretriever.Option) ([]*schema.Document, error) {
	vecs, err := c.embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("embed returned empty result")
	}
	queryVec := vecs[0]

	if docs, sim, ok := c.cache.get(ctx, queryVec); ok {
		g.Log().Infof(ctx, "[SemanticCache] 缓存命中，跳过 Milvus 检索，直接返回 %d 条文档 | query=%q | 相似度=%.4f | 阈值=%.2f", len(docs), query, sim, c.cache.threshold)
		return docs, nil
	}
	g.Log().Infof(ctx, "[SemanticCache] 缓存未命中，回源 Milvus 检索 | query=%q", query)

	// float64 → float32，调 Milvus Search
	f32 := make([]float32, len(queryVec))
	for i, v := range queryVec {
		f32[i] = float32(v)
	}

	sp, err := entity.NewIndexAUTOINDEXSearchParam(1)
	if err != nil {
		return nil, fmt.Errorf("build search param: %w", err)
	}

	results, err := c.milvusCli.Search(
		ctx,
		common.MilvusCollectionName,
		[]string{},
		"",
		[]string{"id", "content", "metadata"},
		[]entity.Vector{entity.FloatVector(f32)},
		"vector",
		entity.COSINE,
		c.topK,
		sp,
	)
	if err != nil {
		return nil, fmt.Errorf("milvus search: %w", err)
	}
	if len(results) == 0 {
		return []*schema.Document{}, nil
	}

	docs, err := parseMilvusResult(results[0])
	if err != nil {
		return nil, err
	}

	c.cache.set(ctx, queryVec, docs)
	return docs, nil
}

// parseMilvusResult 将单条 SearchResult 映射为 []*schema.Document。
// 字段顺序与建表 CollectionFields 一致：id / content / metadata。
func parseMilvusResult(result milvuscli.SearchResult) ([]*schema.Document, error) {
	if result.Err != nil {
		return nil, fmt.Errorf("milvus search result error: %w", result.Err)
	}
	if result.IDs == nil {
		return []*schema.Document{}, nil
	}

	n := result.IDs.Len()
	docs := make([]*schema.Document, n)
	for i := range docs {
		docs[i] = &schema.Document{MetaData: make(map[string]any)}
	}

	for _, field := range result.Fields {
		for i, doc := range docs {
			switch field.Name() {
			case "id":
				// 主键列由 result.IDs 单独维护，不在 Fields 里
				id, err := result.IDs.GetAsString(i)
				if err != nil {
					return nil, fmt.Errorf("get id at %d: %w", i, err)
				}
				doc.ID = id
			case "content":
				content, err := field.GetAsString(i)
				if err != nil {
					return nil, fmt.Errorf("get content at %d: %w", i, err)
				}
				doc.Content = content
			case "metadata":
				// Milvus JSON 字段以 []byte 返回，需手动 Unmarshal
				raw, err := field.Get(i)
				if err != nil {
					return nil, fmt.Errorf("get metadata at %d: %w", i, err)
				}
				b, ok := raw.([]byte)
				if !ok {
					return nil, fmt.Errorf("metadata at %d is not []byte", i)
				}
				if err := json.Unmarshal(b, &doc.MetaData); err != nil {
					return nil, fmt.Errorf("unmarshal metadata at %d: %w", i, err)
				}
			}
		}
	}

	// 余弦相似度写入 _score，供上层过滤或展示
	for i, doc := range docs {
		if i < len(result.Scores) {
			doc.WithScore(float64(result.Scores[i]))
		}
	}

	return docs, nil
}
