package main

import (
	"Fo-Sentinel-Agent/internal/ai/agent/chat_pipeline"
	"Fo-Sentinel-Agent/internal/ai/cache"
	"context"
	"fmt"

	"github.com/cloudwego/eino/schema"
)

func main() {
	ctx := context.Background()
	id := "111"

	mem := cache.GetSessionMemory(id)
	recent := mem.GetRecentMessages()
	summary := mem.GetLongTermSummary()
	history := cache.BuildHistoryWithSummary(recent, summary)

	userMessage := &chat_pipeline.UserMessage{
		ID:      id,
		Query:   "你好",
		History: history,
	}
	runner, err := chat_pipeline.GetChatAgent(ctx)
	if err != nil {
		panic(err)
	}
	// 第一次对话
	out, err := runner.Invoke(ctx, userMessage)
	if err != nil {
		panic(err)
	}
	answer := out.Content
	fmt.Println("Q: 你好")
	fmt.Println("A:", answer)
	cache.GetSessionMemory(id).SetMessages(schema.UserMessage("你好"))
	cache.GetSessionMemory(id).SetMessages(schema.AssistantMessage(out.Content, nil))

	// 重新获取一次 history（包含可能新增的长期摘要）
	recent = mem.GetRecentMessages()
	summary = mem.GetLongTermSummary()
	history = cache.BuildHistoryWithSummary(recent, summary)

	// 第二次对话
	userMessage = &chat_pipeline.UserMessage{
		ID:      id,
		Query:   "现在是几点",
		History: history,
	}
	out, err = runner.Invoke(ctx, userMessage)
	if err != nil {
		panic(err)
	}
	answer = out.Content
	fmt.Println("----------------")
	fmt.Println("Q: 现在是几点")
	fmt.Println("A:", answer)
}
