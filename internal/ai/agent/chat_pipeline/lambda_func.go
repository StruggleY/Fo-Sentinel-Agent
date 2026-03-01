package chat_pipeline

import (
	"context"
	"time"
)

// newInputToRagLambda 是 RAG 支路的入口适配器（DAG 节点：InputToRag）。
// 职责：从 *UserMessage 中提取 Query 字符串，输出给 MilvusRetriever 作为向量检索的查询词。
// 类型转换：*UserMessage → string
// 之所以需要这一层，是因为 DAG 图的入口类型是 *UserMessage，
// 而 MilvusRetriever 只接受 string，两者之间需要一个类型适配节点。
func newInputToRagLambda(ctx context.Context, input *UserMessage, opts ...any) (output string, err error) {
	return input.Query, nil
}

// newInputToChatLambda 是对话历史支路的入口适配器（DAG 节点：InputToChat）。
// 职责：从 *UserMessage 中提取对话所需的全部变量，打包为 map 输出给 ChatTemplate 渲染 Prompt。
// 类型转换：*UserMessage → map[string]any
//
// 输出 map 的 key 与 Prompt 模板占位符严格对应：
//
//	"content" → {content}  本轮用户提问，渲染为最后一条 UserMessage
//	"history" → {history}  历史消息列表，由 MessagesPlaceholder 展开为多条消息（不做字符串替换）
//	"date"    → {date}     当前时间，注入 systemPrompt，让 LLM 感知实时时间（LLM 自身无时间感知）
func newInputToChatLambda(ctx context.Context, input *UserMessage, opts ...any) (output map[string]any, err error) {
	return map[string]any{
		"content": input.Query,
		"history": input.History,
		"date":    time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}
