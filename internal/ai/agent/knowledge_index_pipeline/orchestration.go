package knowledge_index_pipeline

import (
	"context"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
)

// BuildKnowledgeIndexing 构建知识库索引处理流水线
// 返回一个可执行的图编排流程，输入文档源，输出索引ID列表
func BuildKnowledgeIndexing(ctx context.Context) (r compose.Runnable[document.Source, []string], err error) {
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

	// 步骤3: 创建并添加文档分割器节点(将文档切分为块)
	markdownSplitterKeyOfDocumentTransformer, err := newDocumentTransformer(ctx)
	if err != nil {
		return nil, err
	}
	_ = g.AddDocumentTransformerNode(MarkdownSplitter, markdownSplitterKeyOfDocumentTransformer)

	// 步骤4: 创建并添加向量索引器节点(生成向量并存储到Milvus)
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
