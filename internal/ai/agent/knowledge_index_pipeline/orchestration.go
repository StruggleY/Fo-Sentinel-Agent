package knowledge_index_pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	aidoc "Fo-Sentinel-Agent/internal/ai/document"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

// IndexInput 知识库索引管道的输入，包含文件路径、分块配置和元数据。
// 分块配置在运行时传入，无需重建单例 Graph。
type IndexInput struct {
	FilePath string
	BaseID   string            // 所属知识库ID（可选，metadata 注入）
	DocID    string            // 文档ID（可选，Milvus 删除依据）
	DocTitle string            // 文档主标题（metadata 注入）
	Config   aidoc.ChunkConfig // 含 hierarchical 策略参数
}

// BuildAndIndex 执行知识索引并返回 ChunkResult 列表（供知识库服务写 MySQL）。
// 该函数在 Worker Pool 中被调用，不走 Eino Graph（直接调用解析+索引），
// 以便拿到完整的 ChunkResult（含 section_title、parent_content）。
func BuildAndIndex(ctx context.Context, input IndexInput) ([]aidoc.ChunkResult, error) {
	cfg := input.Config
	if cfg.Strategy == "" {
		cfg = aidoc.DefaultChunkConfig()
	}

	var chunks []aidoc.ChunkResult
	var docs []*schema.Document

	if cfg.Strategy == aidoc.StrategyHierarchical {
		// 结构化解析 + 父子分块
		t0 := time.Now()
		parsed, err := aidoc.ParseFileStructured(input.FilePath)
		if err != nil {
			return nil, err
		}
		g.Log().Infof(ctx, "[index] ParseFileStructured: %v", time.Since(t0))
		t1 := time.Now()
		chunks = aidoc.HierarchicalChunkFromDocument(parsed, cfg)
		g.Log().Infof(ctx, "[index] HierarchicalChunk: %v, chunks=%d", time.Since(t1), len(chunks))
	} else {
		// 普通解析 + 分块
		t0 := time.Now()
		text, err := aidoc.ParseFile(input.FilePath)
		if err != nil {
			return nil, err
		}
		g.Log().Infof(ctx, "[index] ParseFile: %v", time.Since(t0))
		rawChunks := aidoc.Chunk(text, cfg)
		chunks = make([]aidoc.ChunkResult, len(rawChunks))
		for i, c := range rawChunks {
			chunks[i] = aidoc.ChunkResult{
				Content:    c,
				ChunkIndex: i,
				CharCount:  len([]rune(c)),
			}
		}
	}

	// 构建 Milvus 文档（携带完整 metadata）
	for i, c := range chunks {
		id := uuid.New().String()
		chunks[i].ID = id
		meta := map[string]any{
			"_type":         "document",
			"base_id":       input.BaseID,
			"doc_id":        input.DocID,
			"doc_title":     input.DocTitle,
			"chunk_index":   c.ChunkIndex,
			"section_title": c.SectionTitle,
		}
		if c.ParentContent != "" {
			meta["parent_content"] = aidoc.TruncateToMaxBytes(c.ParentContent, 6000)
		}
		docs = append(docs, &schema.Document{
			ID:       id,
			Content:  aidoc.TruncateToMaxBytes(c.Content, 8000),
			MetaData: meta,
		})
	}

	if len(docs) == 0 {
		return chunks, nil
	}

	// 获取 documents 分区索引器并并发分批写入（DashScope Embedding API 每批上限 10 条，最大并发 5）
	idx, err := newIndexer(ctx)
	if err != nil {
		return nil, err
	}
	const embedBatchSize = 10
	const maxConcurrent = 5

	// 切分批次
	type batch struct {
		start int
		end   int
	}
	var batches []batch
	for i := 0; i < len(docs); i += embedBatchSize {
		end := i + embedBatchSize
		if end > len(docs) {
			end = len(docs)
		}
		batches = append(batches, batch{i, end})
	}

	// 并发执行，semaphore 限流
	tEmbed := time.Now()
	sem := make(chan struct{}, maxConcurrent)
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup
	for _, b := range batches {
		b := b
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if _, e := idx.Store(ctx, docs[b.start:b.end]); e != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("store batch [%d,%d): %w", b.start, b.end, e)
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	g.Log().Infof(ctx, "[index] Embedding+Store: %v, batches=%d", time.Since(tEmbed), len(batches))
	if firstErr != nil {
		return nil, firstErr
	}
	return chunks, nil
}
