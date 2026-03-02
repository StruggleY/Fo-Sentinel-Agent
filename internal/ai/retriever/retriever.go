package retriever

import (
	"Fo-Sentinel-Agent/internal/ai/embedder"
	"Fo-Sentinel-Agent/utility/client"
	"Fo-Sentinel-Agent/utility/common"
	"context"
	"sync"

	"github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/components/retriever"
)

var (
	globalRetriever retriever.Retriever
	once            sync.Once
	initErr         error
)

// GetRetriever 返回全局单例 Retriever（懒初始化，线程安全）。
//
// 进程生命周期内只执行一次初始化：建立 Milvus TCP 连接、完成 DB/Collection
// 元数据检查、创建 Embedding HTTP 客户端。后续所有 RAG 查询直接复用此单例，
// 消除每次查询重复建连的固定开销
func GetRetriever(ctx context.Context) (retriever.Retriever, error) {
	once.Do(func() {
		globalRetriever, initErr = newMilvusRetriever(ctx)
	})
	return globalRetriever, initErr
}

// WarmUp 在应用启动阶段主动触发单例初始化，将冷启动延迟从首次查询提前到服务就绪阶段。
func WarmUp(ctx context.Context) error {
	_, err := GetRetriever(ctx)
	return err
}

// NewMilvusRetriever 每次创建全新的 Retriever 实例，适用于需要独立连接的场景
// （如知识库索引流水线的一次性批量写入）。
// 常规 RAG 查询请使用 GetRetriever 以复用单例，避免重复建连开销。
func NewMilvusRetriever(ctx context.Context) (retriever.Retriever, error) {
	return newMilvusRetriever(ctx)
}

func newMilvusRetriever(ctx context.Context) (retriever.Retriever, error) {
	cli, err := client.GetMilvusClient(ctx)
	if err != nil {
		return nil, err
	}
	eb, err := embedder.DoubaoEmbedding(ctx)
	if err != nil {
		return nil, err
	}
	r, err := milvus.NewRetriever(ctx, &milvus.RetrieverConfig{
		Client:      cli,
		Collection:  common.MilvusCollectionName,
		VectorField: "vector",
		OutputFields: []string{
			"id",
			"content",
			"metadata",
		},
		TopK:      1,
		Embedding: eb,
	})
	if err != nil {
		return nil, err
	}
	return r, nil
}
