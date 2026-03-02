package indexer

import (
	"context"

	embedder2 "Fo-Sentinel-Agent/internal/ai/embedder"
	"Fo-Sentinel-Agent/utility/client"
	"Fo-Sentinel-Agent/utility/common"

	"github.com/cloudwego/eino-ext/components/indexer/milvus"
)

// NewMilvusIndexer 创建并返回一个 Milvus 向量索引器。
//   - 负责初始化 Milvus 客户端与 Embedding 组件
//   - 字段定义统一引用 common.CollectionFields（单一事实来源），
//     与 client.go 建表时使用的 Schema 完全一致，杜绝字段漂移
func NewMilvusIndexer(ctx context.Context) (*milvus.Indexer, error) {
	cli, err := client.GetMilvusClient(ctx)
	if err != nil {
		return nil, err
	}

	eb, err := embedder2.DoubaoEmbedding(ctx)
	if err != nil {
		return nil, err
	}
	config := &milvus.IndexerConfig{
		Client:     cli,
		Collection: common.MilvusCollectionName,
		Fields:     common.CollectionFields,
		Embedding:  eb,
	}
	indexer, err := milvus.NewIndexer(ctx, config)
	if err != nil {
		return nil, err
	}
	return indexer, nil
}
