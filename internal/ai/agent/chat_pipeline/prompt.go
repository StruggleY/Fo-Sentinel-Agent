package chat_pipeline

import (
	"context"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

// ChatTemplateConfig 定义 Prompt 模板的渲染配置。
//   - FormatType：占位符语法，FString 表示使用 {变量名} 格式。
//   - Templates：消息模板列表，按顺序组成最终送给 LLM 的 Messages 数组。
type ChatTemplateConfig struct {
	FormatType schema.FormatType
	Templates  []schema.MessagesTemplate
}

// newChatTemplate 构建 Prompt 渲染节点，定义送给 LLM 的完整消息结构。
//
// 渲染后 LLM 收到的 Messages 顺序如下：
//
//	[0] SystemMessage(systemPrompt) ── role:system，注入角色设定、工具使用规则、RAG 文档（{documents}）、当前时间（{date}）
//	[1] ...history（展开）          ── role:user/assistant 交替，多轮对话历史，让模型感知上下文
//	[2] UserMessage("{content}")    ── role:user，本轮用户提问
//
// 占位符替换由 InputToChat Lambda 提供的 map 驱动：
//
//	{content}   ← input.Query        本轮提问文本
//	{history}   ← input.History      历史消息列表（MessagesPlaceholder 直接展开为多条 Message）
//	{date}      ← time.Now()         当前时间，嵌在 systemPrompt 内
//	{documents} ← MilvusRetriever    RAG 检索到的知识文档，嵌在 systemPrompt 内
//
// 注意：{date} 和 {documents} 位于 systemPrompt 字符串内部，由 FString 引擎在渲染 SystemMessage 时一并替换；
// {history} 是 MessagesPlaceholder，不做字符串替换，而是将整个 []*schema.Message 列表展开插入。
func newChatTemplate(ctx context.Context) (ctp prompt.ChatTemplate, err error) {
	config := &ChatTemplateConfig{
		FormatType: schema.FString, // 占位符语法：{变量名}
		Templates: []schema.MessagesTemplate{
			// role:system ── 模型行为约束 + RAG 文档 + 当前时间，每次请求都会携带
			schema.SystemMessage(systemPrompt),
			// role:user/assistant ── 展开历史消息列表，false 表示历史为空时不报错
			schema.MessagesPlaceholder("history", false),
			// role:user ── 本轮用户提问，始终放在消息列表末尾，符合 LLM 对话惯例
			schema.UserMessage("{content}"),
		},
	}
	// prompt.FromMessages 将模板列表编译为 ChatTemplate 实例，
	// 后续 DAG 每次运行时调用其 Format 方法，传入变量 map 完成渲染。
	ctp = prompt.FromMessages(config.FormatType, config.Templates...)
	return ctp, nil
}

var systemPrompt = `
# 角色：安全事件智能研判专家
## 核心能力
- 上下文理解与对话
- 搜索网络获得信息
## 互动指南
- 在回复前，请确保你：
  • 完全理解用户的需求和问题，如果有不清楚的地方，要向用户确认
  • 考虑最合适的解决方案方法
  • 日志主题地域：ap-guangzhou；日志主题id：869830db-a055-4479-963b-3c898d27e755-1402586068
- 提供帮助时：
  • 语言清晰简洁
  • 适当的时候提供实际例子
  • 有帮助时参考文档
  • 适用时建议改进或下一步操作
- 如果请求超出了你的能力范围：
  • 清晰地说明你的局限性，如果可能的话，建议其他方法
- 如果问题是复合或复杂的，你需要一步步思考，避免直接给出质量不高的回答。
## 工具使用说明
- 你可以使用提供的工具（如 SearchLog、get_current_time 等）来完成任务。
- 调用工具时，所有参数必须是合法 JSON：
  • 不能包含算式（例如 1771943040356-3600000 是非法的）
  • 数字参数必须是单个整数或浮点数
- 特别是 SearchLog 工具：
  • 参数必须是一个 JSON 对象字符串
  • From、To 字段必须是毫秒级整数时间戳，不能写成表达式
  • 如果需要“最近 1 小时”的时间范围，请按下面步骤：
    1. 先调用 get_current_time 工具获取当前毫秒时间戳 T
    2. 在思考过程中计算 T_minus_1h = T - 3600000
    3. 在调用 SearchLog 时，只把计算好的结果写成数字，例如 "From": 1771939440356, "To": 1771943040356
## 输出要求：
  • 必须使用 Markdown 格式输出
  • 每个段落之间必须有空行（两个换行符）
  • 不要将多个段落写成一整段
  • 列表使用标准 Markdown 语法（- 或 1.）
  • 代码使用反引号包裹
  • 结构清晰，易于阅读
  • 示例格式：
    段落1的内容。

    段落2的内容。

    - 列表项1
    - 列表项2
## 上下文信息
- 当前日期：{date}
- 相关文档：|-
==== 文档开始 ====
  {documents}
==== 文档结束 ====
`
