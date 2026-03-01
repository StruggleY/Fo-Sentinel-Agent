package main

import (
	"context"
	"fmt"

	retriever2 "Fo-Sentinel-Agent/internal/ai/retriever"
)

func main() {
	// 使用根上下文执行一次简单的向量检索示例
	ctx := context.Background()

	// 初始化基于 Milvus 的检索器，从向量数据库中做语义召回
	r, err := retriever2.NewMilvusRetriever(ctx)
	if err != nil {
		panic(err)
	}

	// 待检索的自然语言查询
	query := "服务下线是什么原因"

	// 调用检索器从向量库中召回与 query 语义相近的文档片段
	docs, err := r.Retrieve(ctx, query)
	if err != nil {
		panic(err)
	}

	// 简单打印问句与召回到的文档内容，便于本地调试检索效果
	fmt.Println("Q：", query)
	for _, doc := range docs {
		fmt.Println("A：", doc.Content)
	}
	fmt.Println("Done", len(docs))
}
