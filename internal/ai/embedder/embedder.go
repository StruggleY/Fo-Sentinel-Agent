package embedder

import (
	"context"
	"log"

	"Fo-Sentinel-Agent/utility/common"

	"github.com/cloudwego/eino-ext/components/embedding/dashscope"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/gogf/gf/v2/frame/g"
)

// DoubaoEmbedding 基于配置中心创建一个豆包 Embedding 组件。
// - 从全局配置读取模型名与 API Key
// - 封装 dashscope 的 Embedder，为上层提供统一的向量化接口
// - 向量维度使用 common.EmbeddingDim，与 Milvus CollectionFields 中的 FloatVector dim 保持严格一致
func DoubaoEmbedding(ctx context.Context) (eb embedding.Embedder, err error) {
	model, err := g.Cfg().Get(ctx, "doubao_embedding_model.model")
	if err != nil {
		return nil, err
	}
	apiKey, err := g.Cfg().Get(ctx, "doubao_embedding_model.api_key")
	if err != nil {
		return nil, err
	}
	dim := common.EmbeddingDim
	embedder, err := dashscope.NewEmbedder(ctx, &dashscope.EmbeddingConfig{
		Model:      model.String(),
		APIKey:     apiKey.String(),
		Dimensions: &dim,
	})
	if err != nil {
		log.Printf("new embedder error: %v\n", err)
		return nil, err
	}
	return embedder, nil
}
