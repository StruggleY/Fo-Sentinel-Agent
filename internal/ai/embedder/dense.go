package embedder

import (
	"context"

	milvus "Fo-Sentinel-Agent/internal/dao/milvus"

	"github.com/cloudwego/eino-ext/components/embedding/dashscope"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/gogf/gf/v2/frame/g"
)

// NewDenseEmbedder 创建稠密向量嵌入器（DashScope text-embedding-v4）
//
// 用途：将文本编码为 1024 维稠密向量，用于语义相似度检索
func NewDenseEmbedder(ctx context.Context) (embedding.Embedder, error) {
	model, _ := g.Cfg().Get(ctx, "doubao_embedding_model.model")
	apiKey, _ := g.Cfg().Get(ctx, "doubao_embedding_model.api_key")
	dim := milvus.EmbeddingDim

	return dashscope.NewEmbedder(ctx, &dashscope.EmbeddingConfig{
		Model:      model.String(),
		APIKey:     apiKey.String(),
		Dimensions: &dim,
	})
}
