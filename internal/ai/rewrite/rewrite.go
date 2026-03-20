// Package rewrite 查询重写服务：利用 LLM 将口语化/代词式查询改写为向量检索友好的独立查询。
// 参考 Ragent 的 QueryRewriteService 实现，在 RAG 检索前对查询进行上下文补全和去歧义化。
// 任何失败均静默回退到原始查询，不影响主流程。
package rewrite

import (
	"context"
	"strings"

	"Fo-Sentinel-Agent/internal/ai/models"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// RewriteQuery 将用户查询 + 对话历史改写为向量检索友好的独立查询。
//
// 改写目标：
//   - 消除代词歧义："它"/"这个漏洞"/"该事件" → 完整实体名称
//   - 补全时间上下文："最近的" → "最近7天的"，"上周" → "上周发生的"
//   - 保留领域语义：CVE 编号、漏洞名称、安全术语原文保留
//
// 使用快速模型（DeepSeek V3 Quick）以控制延迟（< 300ms）。
// 若改写失败（模型调用错误或结果异常），静默回退到原始查询。
func RewriteQuery(ctx context.Context, query string, history []*schema.Message) string {
	m, err := models.OpenAIForDeepSeekV3Quick(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[Rewrite] 模型初始化失败，使用原始查询: %v", err)
		return query
	}

	messages := buildRewriteMessages(query, history)

	resp, err := m.Generate(ctx, messages)
	if err != nil || resp.Content == "" {
		g.Log().Warningf(ctx, "[Rewrite] 查询改写失败，使用原始查询: %v", err)
		return query
	}

	rewritten := strings.TrimSpace(resp.Content)
	// 安全检查：过短或过长的改写结果视为异常，回退原始查询
	if len(rewritten) < 3 || len(rewritten) > 500 {
		return query
	}

	g.Log().Debugf(ctx, "[Rewrite] 查询改写完成 | 原始=%q | 改写=%q", query, rewritten)
	return rewritten
}
