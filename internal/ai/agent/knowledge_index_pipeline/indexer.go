package knowledge_index_pipeline

import (
	indexer2 "Fo-Sentinel-Agent/internal/ai/indexer"
	"context"

	"github.com/cloudwego/eino/components/indexer"
)

// newIndexer component initialization function of node 'RedisIndexer' in graph 'KnowledgeIndexing'
func newIndexer(ctx context.Context) (idr indexer.Indexer, err error) {
	return indexer2.NewMilvusIndexer(ctx)
}
