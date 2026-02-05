package chat_pipeline

import (
	retriever2 "Fo-Sentinel-Agent/internal/ai/retriever"
	"context"

	"github.com/cloudwego/eino/components/retriever"
)

func newRetriever(ctx context.Context) (rtr retriever.Retriever, err error) {
	return retriever2.NewMilvusRetriever(ctx)
}
