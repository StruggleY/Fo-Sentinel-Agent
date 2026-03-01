package indexer

import (
	"context"

	embedder2 "Fo-Sentinel-Agent/internal/ai/embedder"
	"Fo-Sentinel-Agent/utility/client"
	"Fo-Sentinel-Agent/utility/common"

	"github.com/cloudwego/eino-ext/components/indexer/milvus"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// NewMilvusIndexer 创建并返回一个 Milvus 向量索引器。
// - 负责初始化 Milvus 客户端与 Embedding 组件
// - 使用统一的集合名与字段配置，保证 Embedding 与 Milvus 之间的向量维度等参数一致
func NewMilvusIndexer(ctx context.Context) (*milvus.Indexer, error) {
	cli, err := client.NewMilvusClient(ctx)
	if err != nil {
		return nil, err
	}

	// 使用 DoubaoEmbedding 作为向量生成器，保证和向量库中字段维度配置匹配
	eb, err := embedder2.DoubaoEmbedding(ctx)
	if err != nil {
		return nil, err
	}
	config := &milvus.IndexerConfig{
		Client:     cli,
		Collection: common.MilvusCollectionName,
		Fields:     fields,
		Embedding:  eb,
	}
	indexer, err := milvus.NewIndexer(ctx, config)
	if err != nil {
		return nil, err
	}
	return indexer, nil
}

var fields = []*entity.Field{
	{
		// 主键字段：用于唯一标识每条向量记录（如文档分片 ID）
		Name:     "id",
		DataType: entity.FieldTypeVarChar,
		TypeParams: map[string]string{
			"max_length": "255",
		},
		PrimaryKey: true,
	},
	{
		// 向量字段：用于存储 Embedding 结果
		// 注意：BinaryVector 的 dim 表示 bit 维度，需要与实际 Embedding 维度配置对应
		Name:     "vector", // 确保字段名与索引器/搜索端使用的字段名一致
		DataType: entity.FieldTypeBinaryVector,
		TypeParams: map[string]string{
			"dim": "65536",
		},
	},
	{
		// 文本内容字段：原始文本或片段内容
		Name:     "content",
		DataType: entity.FieldTypeVarChar,
		TypeParams: map[string]string{
			"max_length": "8192",
		},
	},
	{
		// 元信息字段：存放诸如文件名、分片位置、标签等业务元数据
		Name:     "metadata",
		DataType: entity.FieldTypeJSON,
	},
}
