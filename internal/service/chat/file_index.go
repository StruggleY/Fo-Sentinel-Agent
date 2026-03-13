// file_index.go 知识库索引构建逻辑：Milvus 去重删除 + 向量索引写入。
package chatsvc

import (
	"Fo-Sentinel-Agent/internal/ai/agent/knowledge_index_pipeline"
	"Fo-Sentinel-Agent/utility/client"
	"Fo-Sentinel-Agent/utility/common"
	"Fo-Sentinel-Agent/utility/log_call_back"
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
)

// BuildFileIndex 对指定路径的文件执行 Milvus 去重 + 向量索引构建。
// 先删除同源文件的旧记录，再执行 FileLoader → MarkdownSplitter → MilvusIndexer 流水线。
func BuildFileIndex(ctx context.Context, path string) error {
	r, err := knowledge_index_pipeline.GetKnowledgeIndexing(ctx)
	if err != nil {
		return fmt.Errorf("build knowledge indexing failed: %w", err)
	}

	cli, err := client.GetMilvusClient(ctx)
	if err != nil {
		return err
	}

	// 查询并删除相同源文件的旧索引（避免重复索引）
	expr := fmt.Sprintf(`metadata["_source"] == "%s"`, path)
	queryResult, err := cli.Query(ctx, common.MilvusCollectionName, []string{}, expr, []string{"id"})
	if err != nil {
		return err
	}
	if len(queryResult) > 0 {
		var idsToDelete []string
		for _, column := range queryResult {
			if column.Name() == "id" {
				for i := 0; i < column.Len(); i++ {
					if id, err := column.GetAsString(i); err == nil {
						idsToDelete = append(idsToDelete, id)
					}
				}
			}
		}
		if len(idsToDelete) > 0 {
			deleteExpr := fmt.Sprintf(`id in ["%s"]`, strings.Join(idsToDelete, `","`))
			if err := cli.Delete(ctx, common.MilvusCollectionName, "", deleteExpr); err != nil {
				fmt.Printf("[warn] delete existing data failed: %v\n", err)
			} else {
				fmt.Printf("[info] deleted %d existing records with _source: %s\n", len(idsToDelete), path)
			}
		}
	}

	ids, err := r.Invoke(ctx, document.Source{URI: path}, compose.WithCallbacks(log_call_back.LogCallback(nil)))
	if err != nil {
		return fmt.Errorf("invoke index graph failed: %w", err)
	}
	fmt.Printf("[done] indexing file: %s, len of parts: %d\n", path, len(ids))
	return nil
}
