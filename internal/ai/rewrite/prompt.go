package rewrite

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// buildRewriteMessages 构建查询改写的消息列表。
// 注入最近 3 轮对话（最多 6 条消息）作为上下文，减少代词和省略带来的歧义。
func buildRewriteMessages(query string, history []*schema.Message) []*schema.Message {
	messages := []*schema.Message{
		schema.SystemMessage(rewriteSystemPrompt),
	}

	// 注入最近 3 轮对话上下文（过多历史反而引入噪声）
	if len(history) > 0 {
		last := history
		if len(last) > 6 {
			last = history[len(history)-6:]
		}
		ctxStr := buildHistoryContext(last)
		if ctxStr != "" {
			messages = append(messages,
				schema.UserMessage(fmt.Sprintf("<history>\n%s\n</history>", ctxStr)),
				schema.AssistantMessage("已理解上下文，请提供需要改写的查询。", nil),
			)
		}
	}

	messages = append(messages, schema.UserMessage(query))
	return messages
}

// buildHistoryContext 将历史消息格式化为可读文本
func buildHistoryContext(history []*schema.Message) string {
	var parts []string
	for _, msg := range history {
		if msg.Content == "" {
			continue
		}
		role := "用户"
		if msg.Role == schema.Assistant {
			role = "助手"
		}
		parts = append(parts, fmt.Sprintf("%s: %s", role, msg.Content))
	}
	return strings.Join(parts, "\n")
}

const rewriteSystemPrompt = `你是查询改写助手，专门优化用于向量数据库语义检索的查询语句。

改写规则：
1. 消除代词歧义：将"它"、"这个漏洞"、"该事件"等替换为完整的实体名称
2. 补全时间上下文：如"最近的" → "最近7天的"，"上周" → "上周发生的"
3. 保持领域语义：安全事件、CVE编号、漏洞名称等术语保持原文不变
4. 输出简洁：只输出改写后的查询，不加任何解释或标点符号前缀

如果查询已经足够清晰自包含，直接原样返回（不要修改）。
只输出改写后的一句话查询，不加任何额外内容。`
