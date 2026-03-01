package knowledge_index_pipeline

import (
	"context"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
	"github.com/cloudwego/eino/components/document"
	"github.com/google/uuid"
)

// newDocumentTransformer 初始化 Markdown 文档分割器组件。
// - 基于 Markdown 标题（如 "#"）将整篇文档拆成多个逻辑小节
// - 为拆分后的每个片段生成新的唯一 ID，作为后续索引和检索的最小文档单元
// - 仅负责“分块”和元信息处理，不参与向量化，便于与下游索引器解耦
func newDocumentTransformer(ctx context.Context) (tfr document.Transformer, err error) {
	config := &markdown.HeaderConfig{
		Headers: map[string]string{
			"#": "title",
		},
		TrimHeaders: false,
		IDGenerator: func(ctx context.Context, originalID string, splitIndex int) string {
			return uuid.New().String()
		},
	}
	tfr, err = markdown.NewHeaderSplitter(ctx, config)
	if err != nil {
		return nil, err
	}
	return tfr, nil
}
