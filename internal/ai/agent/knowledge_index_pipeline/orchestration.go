package knowledge_index_pipeline

import (
	"context"
	"sync"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
)

var (
	knowledgeRunner  compose.Runnable[document.Source, []string]
	knowledgeOnce    sync.Once
	knowledgeInitErr error
)

// GetKnowledgeIndexing 返回全局缓存的知识库索引 Agent（懒初始化，线程安全）。
//
// 进程生命周期内只执行一次 DAG 编译（g.Compile），所有文件上传请求复用同一个 Runnable。
// 返回一个可执行的图编排流程，输入文档源，输出索引 ID 列表。
func GetKnowledgeIndexing(ctx context.Context) (compose.Runnable[document.Source, []string], error) {
	knowledgeOnce.Do(func() {
		knowledgeRunner, knowledgeInitErr = buildKnowledgeIndexing(context.Background())
	})
	return knowledgeRunner, knowledgeInitErr
}

// buildKnowledgeIndexing 构建知识库索引处理流水线（内部函数，仅被 sync.Once 调用一次）。
func buildKnowledgeIndexing(ctx context.Context) (r compose.Runnable[document.Source, []string], err error) {
	// 定义流水线各节点名称
	const (
		FileLoader       = "FileLoader"       // 文件加载器节点
		MarkdownSplitter = "MarkdownSplitter" // 文档分割器节点
		MilvusIndexer    = "MilvusIndexer"    // 向量索引器节点
	)

	// 步骤1: 创建处理流程图
	g := compose.NewGraph[document.Source, []string]()

	// 步骤2: 创建并添加文件加载器节点(负责读取文件内容)
	fileLoaderKeyOfLoader, err := newLoader(ctx)
	if err != nil {
		return nil, err
	}
	_ = g.AddLoaderNode(FileLoader, fileLoaderKeyOfLoader)

	// 步骤3: 创建并添加文档分割器节点
	// 注意：该节点只负责“如何把原始文档拆成多个语义片段”（如按 Markdown 标题分段），
	// 不做向量化，便于后续可以替换不同的切分策略，而不影响下游索引器逻辑。
	markdownSplitterKeyOfDocumentTransformer, err := newDocumentTransformer(ctx)
	if err != nil {
		return nil, err
	}
	_ = g.AddDocumentTransformerNode(MarkdownSplitter, markdownSplitterKeyOfDocumentTransformer)

	// 步骤4: 创建并添加向量索引器节点
	// 该节点会为每个“已分片文档”调用 Embedding 组件（如 DoubaoEmbedding）生成向量，
	// 并按照 Milvus 的字段 schema（id/vector/content/metadata）将数据写入向量库。
	milvusIndexerKeyOfIndexer, err := newIndexer(ctx)
	if err != nil {
		return nil, err
	}
	_ = g.AddIndexerNode(MilvusIndexer, milvusIndexerKeyOfIndexer)

	// 步骤5: 构建节点间的数据流向
	_ = g.AddEdge(compose.START, FileLoader)       // 起点 -> 文件加载器
	_ = g.AddEdge(FileLoader, MarkdownSplitter)    // 文件加载器 -> 文档分割器
	_ = g.AddEdge(MarkdownSplitter, MilvusIndexer) // 文档分割器 -> 向量索引器
	_ = g.AddEdge(MilvusIndexer, compose.END)      // 向量索引器 -> 终点

	// 步骤6: 编译流程图为可执行对象
	r, err = g.Compile(ctx, compose.WithGraphName("KnowledgeIndexing"), compose.WithNodeTriggerMode(compose.AnyPredecessor))
	if err != nil {
		return nil, err
	}
	return r, err
}
