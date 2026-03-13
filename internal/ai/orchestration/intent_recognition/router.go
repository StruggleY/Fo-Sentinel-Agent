// router.go 意图路由节点：调用 LLM（DeepSeekV3 Quick）对用户查询进行意图识别，输出目标 IntentType。
// 核心原理：构造含 5 类意图示例的 prompt → 调用 LLM → 解析 JSON 响应 → 映射为 IntentType。
// 任何环节异常（模型调用失败、JSON 解析错误、未知意图值）均降级为 IntentChat，保证路由不中断。
package intent_recognition

import (
	"context"
	"encoding/json"
	"fmt"

	"Fo-Sentinel-Agent/internal/ai/models"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// RouterOutput 路由节点输出，供 Executor 节点消费。
// IntentType 为意图识别结果；识别失败时降级为 IntentChat，因此该字段始终有效。
// Input 原样透传，Executor 从中取 Query 和 Callback 构建 Task。
type RouterOutput struct {
	Input      *IntentInput // 原始用户输入与回调，Executor 从中取 Query 和 Callback
	IntentType IntentType   // LLM 意图识别结果，recognizeIntent 失败或解析异常时降级为 chat
}

// newRouterLambda 创建路由 Lambda 节点：调用 recognizeIntent 做 LLM 意图识别，
// 识别失败或解析异常时降级为 IntentChat，并通过 Callback 推送路由结果
func newRouterLambda(ctx context.Context) (*compose.Lambda, error) {
	chatModel, err := models.OpenAIForDeepSeekV3Quick(ctx)
	if err != nil {
		return nil, fmt.Errorf("create router model: %w", err)
	}

	lambda := compose.InvokableLambda(func(ctx context.Context, input *IntentInput) (*RouterOutput, error) {
		// 调用 LLM 进行意图识别；失败时降级为 IntentChat，保证路由不中断
		intentType, err := recognizeIntent(ctx, chatModel, input.Query)
		if err != nil {
			g.Log().Warningf(ctx, "[Router] recognizeIntent failed, fallback to chat: %v", err)
			intentType = IntentChat
		}

		// 通过 Callback 向调用方实时推送路由结果，供前端展示当前激活的智能体
		if input.Callback != nil {
			input.Callback(IntentStatus, fmt.Sprintf("[路由到: %s]", intentType))
		}

		return &RouterOutput{
			Input:      input,
			IntentType: intentType,
		}, nil
	})
	return lambda, nil
}

// recognizeIntent 使用 DeepSeekV3 Quick 模型（低延迟）对 query 进行意图识别。
// 采用 Quick 模型而非完整推理模型，是为了降低路由延迟，识别任务复杂度低，Quick 足够准确。
// 返回 IntentType；模型调用失败时返回 IntentChat 及 error，由调用方决定是否降级。
func recognizeIntent(ctx context.Context, m model.ToolCallingChatModel, query string) (IntentType, error) {
	messages := []*schema.Message{
		schema.SystemMessage(buildRecognitionSystemPrompt()),
		schema.UserMessage(query),
	}

	resp, err := m.Generate(ctx, messages)
	if err != nil {
		return IntentChat, fmt.Errorf("recognize intent: %w", err)
	}

	return parseIntentType(ctx, resp.Content), nil
}

// buildRecognitionSystemPrompt 构建意图识别系统提示词：定义 6 类意图的职责范围、判断边界与示例。
// 作为 SystemMessage 注入，角色定位更稳定；用户 query 作为独立 UserMessage 传入。
// 要求 LLM 仅返回 JSON（{"intent": "xxx"}），intent 值限定为枚举范围，避免干扰 parseIntentType 解析。
func buildRecognitionSystemPrompt() string {
	return `你是意图识别器。根据用户问题，从以下意图类型中选择最匹配的一个。

<intents>
- chat:  通用对话、安全咨询、知识问答、日志查看、订阅管理（不确定时默认选此项）
- event: 安全事件查询、事件分析、告警关联、事件处置建议（关注"发生了什么"）
- report: 生成报告、查看报告、报告统计分析
- risk:  风险评估、威胁建模、漏洞评分、CVE 严重性分析（关注"危险程度如何"）
- plan:  需要多步骤规划才能完成的复杂操作
- solve: 针对某条具体安全事件/CVE 生成应急响应步骤、修复方案、处置指导（关注"怎么解决"）
</intents>

<examples>
"最近有什么安全事件" → event
"昨天触发了哪些告警" → event
"帮我生成本周安全报告" → report
"上个月的报告数据怎么样" → report
"评估这个CVE的风险等级" → risk
"这个漏洞有多危险" → risk
"帮我部署一套监控系统" → plan
"查一下系统日志" → chat
"什么是SQL注入" → chat
"帮我管理一下订阅" → chat
"CVE-2024-1234 怎么修复" → solve
"这个漏洞的应急处置步骤是什么" → solve
"帮我生成这条事件的解决方案" → solve
</examples>

注意：event 关注"事件本身"，risk 关注"危险程度评估"，solve 关注"如何处置某条具体事件"，不确定时选 chat。

只返回JSON，intent 值只能是 chat/event/report/risk/plan/solve 之一：{"intent": "xxx"}`
}

// parseIntentType 解析 LLM 返回的 JSON 中的 intent 字段，映射为 IntentType 常量。
// 三类异常均回退为 IntentChat：① JSON 格式错误；② intent 字段为空；③ 未知意图值。
// 此设计保证路由层对 LLM 输出具备完全容错，任何情况下都能产出有效的 IntentType。
func parseIntentType(ctx context.Context, content string) IntentType {
	var result struct {
		Intent string `json:"intent"`
	}

	if err := json.Unmarshal([]byte(content), &result); err != nil {
		g.Log().Debugf(ctx, "[Router] parseIntentType failed, raw=%q, fallback to chat", content)
		return IntentChat
	}

	switch result.Intent {
	case "event":
		return IntentEvent
	case "report":
		return IntentReport
	case "risk":
		return IntentRisk
	case "plan":
		return IntentPlan
	case "solve":
		return IntentSolve
	default:
		return IntentChat
	}
}
