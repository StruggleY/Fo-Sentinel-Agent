// Package rewrite split.go：子问题拆分服务
//
// 背景：
//   用户提问时常常把多个独立问题合并在一句话里，例如：
//   "查询本周安全事件数量和各事件的风险评分分布"
//   这个问题包含两个独立的检索目标：① 事件数量  ② 风险评分分布
//   如果用混合语义的一条查询做向量检索，Milvus 的嵌入向量会是两个语义的平均，
//   可能只命中其中一个维度，导致召回不完整。
//
// 解决方案：
//   参考 Ragent 的 MultiQuestionRewriteService，在检索前用 LLM 分析查询维度，
//   若检测到多个独立检索目标则拆分为子查询，再对每个子查询独立检索后聚合。
//
// 使用约定：
//   - 本函数应在 RewriteQuery 之后调用（代词已消歧、时间已补全）
//   - 返回值至少包含 1 个元素（最坏情况返回 []string{query} 原始查询）
//   - 调用方对每个子查询独立检索，通过 MultiRetrieve 并发执行并聚合去重
package rewrite

import (
	"context"
	"encoding/json"
	"strings"

	"Fo-Sentinel-Agent/internal/ai/models"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// SplitQuestions 检测复杂查询并分解为独立子查询（最多 3 个）。
//
// 处理逻辑：
//  1. 调用 DeepSeek V3 Quick（低延迟）分析查询是否包含多个独立检索维度
//  2. LLM 返回 JSON：{"questions": ["子查询1", "子查询2"]}
//  3. 若是单一问题，LLM 直接返回 {"questions": ["原始查询"]}（不做修改）
//  4. 过滤空字符串，确保返回列表有效
//
// 容错策略：
//   - 模型初始化失败 → 返回 []string{query}
//   - 模型调用失败或空响应 → 返回 []string{query}
//   - JSON 解析失败 → 返回 []string{query}
//   - 所有子查询均为空 → 返回 []string{query}
//   任何失败均静默回退，不向调用方传播错误，保证主流程不中断。
func SplitQuestions(ctx context.Context, query string) []string {
	m, err := models.OpenAIForDeepSeekV3Quick(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[Split] 模型初始化失败，使用原始查询: %v", err)
		return []string{query}
	}

	messages := buildSplitMessages(query)
	resp, err := m.Generate(ctx, messages)
	if err != nil || resp.Content == "" {
		g.Log().Warningf(ctx, "[Split] 子问题拆分失败，使用原始查询: %v", err)
		return []string{query}
	}

	// 解析 LLM 返回的 JSON，提取子查询列表
	content := strings.TrimSpace(resp.Content)
	var result struct {
		Questions []string `json:"questions"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		g.Log().Debugf(ctx, "[Split] JSON 解析失败，使用原始查询 | raw=%q", content)
		return []string{query}
	}

	// 过滤空字符串，防止下游向 Milvus 发送空查询
	var questions []string
	for _, q := range result.Questions {
		if q = strings.TrimSpace(q); q != "" {
			questions = append(questions, q)
		}
	}
	if len(questions) == 0 {
		return []string{query}
	}

	g.Log().Debugf(ctx, "[Split] 子问题拆分完成 | 原始=%q | 子问题数=%d", query, len(questions))
	return questions
}

// splitSystemPrompt 子问题拆分的 System Prompt。
//
// 设计说明：
//   - 限制最多 3 个子问题：避免过度拆分导致检索噪声增多、延迟激增
//   - 强调「独立检索」：每个子查询必须能单独送入向量数据库检索，不依赖其他子查询结果
//   - 保留安全领域术语：CVE 编号、漏洞名、组件名等专业术语原文保留，确保向量相似度匹配准确
const splitSystemPrompt = `你是查询分析器。分析用户查询，判断是否包含多个独立的检索维度。

规则：
- 若问题单一（只有一个检索目标），返回 {"questions": ["<原始查询>"]}
- 若问题复杂（包含2-3个独立检索维度），将每个维度分解为一个独立查询
- 最多分解为 3 个子问题，避免过度拆分
- 每个子问题必须能独立检索，不依赖其他子问题的结果
- 保留安全领域术语（CVE编号、漏洞名、组件名）原文

示例：
"最近有哪些高危漏洞，以及它们的修复方案是什么" → {"questions": ["最近有哪些高危漏洞", "高危漏洞的修复方案"]}
"查询本周安全事件数量和风险评分分布" → {"questions": ["本周安全事件数量", "安全事件风险评分分布"]}
"CVE-2024-1234 的危害等级" → {"questions": ["CVE-2024-1234 的危害等级"]}

只返回 JSON，格式：{"questions": ["问题1", "问题2"]}`

// buildSplitMessages 构建子问题拆分的消息列表。
// System Prompt 定义拆分规则，User Message 携带待分析的查询。
func buildSplitMessages(query string) []*schema.Message {
	return []*schema.Message{
		schema.SystemMessage(splitSystemPrompt),
		schema.UserMessage(query),
	}
}
