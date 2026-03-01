package embedder

import (
	"context"
	"log"

	"github.com/cloudwego/eino-ext/components/embedding/dashscope"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/gogf/gf/v2/frame/g"
)

// DoubaoEmbedding 基于配置中心创建一个豆包 Embedding 组件。
// - 从全局配置读取模型名与 API Key
// - 封装 dashscope 的 Embedder，为上层提供统一的向量化接口
func DoubaoEmbedding(ctx context.Context) (eb embedding.Embedder, err error) {
	// 从配置中读取模型名称，如：doubao-embedding-v1
	model, err := g.Cfg().Get(ctx, "doubao_embedding_model.model")
	if err != nil {
		return nil, err
	}
	// 从配置中读取访问豆包服务的 API Key
	api_key, err := g.Cfg().Get(ctx, "doubao_embedding_model.api_key")
	if err != nil {
		return nil, err
	}
	// 维度需与下游向量库（如 Milvus）schema 中的向量字段配置保持一致
	dim := 2048
	embedder, err := dashscope.NewEmbedder(ctx, &dashscope.EmbeddingConfig{
		Model:      model.String(),
		APIKey:     api_key.String(),
		Dimensions: &dim,
	})
	if err != nil {
		log.Printf("new embedder error: %v\n", err)
		return nil, err
	}
	return embedder, nil
}
