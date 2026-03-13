package indexer

import (
	"context"
	"fmt"
	"sync"

	embedder2 "Fo-Sentinel-Agent/internal/ai/embedder"
	"Fo-Sentinel-Agent/utility/client"
	"Fo-Sentinel-Agent/utility/common"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino-ext/components/indexer/milvus"
	"github.com/cloudwego/eino/schema"
)

// floatVectorRow 是用于 Milvus InsertRows 的行结构体。
// Vector 字段类型为 []float32，与 CollectionFields 中 FloatVector 对应。
type floatVectorRow struct {
	ID       string    `json:"id" milvus:"name:id"`
	Content  string    `json:"content" milvus:"name:content"`
	Vector   []float32 `json:"vector" milvus:"name:vector"`
	Metadata []byte    `json:"metadata" milvus:"name:metadata"`
}

// floatVectorDocumentConverter 将 Document + Embedding 结果转为 Milvus FloatVector 行。
// eino-ext 默认转换器生成 []byte（BinaryVector），不兼容 FloatVector schema。
func floatVectorDocumentConverter(_ context.Context, docs []*schema.Document, vectors [][]float64) ([]interface{}, error) {
	rows := make([]interface{}, 0, len(docs))
	for i, doc := range docs {
		metadata, err := sonic.Marshal(doc.MetaData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		vec32 := make([]float32, len(vectors[i]))
		for j, v := range vectors[i] {
			vec32[j] = float32(v)
		}
		rows = append(rows, &floatVectorRow{
			ID:       doc.ID,
			Content:  doc.Content,
			Vector:   vec32,
			Metadata: metadata,
		})
	}
	return rows, nil
}

var (
	globalIndexer     *milvus.Indexer
	globalIndexerOnce sync.Once
	globalIndexerErr  error
)

// GetMilvusIndexer 返回全局单例 Milvus 索引器（懒初始化，线程安全）。
// milvus.NewIndexer 每次都会执行 HasCollection/DescribeCollection/GetLoadState 等 gRPC 检查，
// 缓存单例避免每次 IndexDocuments 重复触发这些初始化 round-trip。
func GetMilvusIndexer(ctx context.Context) (*milvus.Indexer, error) {
	globalIndexerOnce.Do(func() {
		globalIndexer, globalIndexerErr = NewMilvusIndexer(ctx)
	})
	return globalIndexer, globalIndexerErr
}

// NewMilvusIndexer 创建并返回一个新的 Milvus 向量索引器（每次调用都做完整初始化检查）。
// 日常使用请调用 GetMilvusIndexer 获取单例；此函数供首次初始化或测试场景使用。
//   - 字段定义统一引用 common.CollectionFields（单一事实来源），
//     与 milvus.go 建表时使用的 Schema 完全一致，杜绝字段漂移
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
		Client:            cli,
		Collection:        common.MilvusCollectionName,
		Fields:            common.CollectionFields,
		Embedding:         eb,
		DocumentConverter: floatVectorDocumentConverter,
	}
	idx, err := milvus.NewIndexer(ctx, config)
	if err != nil {
		return nil, err
	}
	return idx, nil
}
