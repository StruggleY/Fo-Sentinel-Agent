package summary_pipeline

import (
	"context"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

// SummaryTemplateConfig 定义摘要 Prompt 模板的渲染配置
type SummaryTemplateConfig struct {
	FormatType schema.FormatType
	Templates  []schema.MessagesTemplate
}

// newSummaryTemplate 构建摘要 Prompt 渲染节点
//
// 渲染后 LLM 收到的 Messages：
//
//	[0] SystemMessage - 角色设定（专业的对话总结助手）
//	[1] UserMessage   - 摘要任务（包含对话内容和输出要求）
func newSummaryTemplate(ctx context.Context) (ctp prompt.ChatTemplate, err error) {
	config := &SummaryTemplateConfig{
		FormatType: schema.FString,
		Templates: []schema.MessagesTemplate{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(userPrompt),
		},
	}
	ctp = prompt.FromMessages(config.FormatType, config.Templates...)
	return ctp, nil
}

// systemPrompt 定义摘要助手的角色和能力
//
// 设计原则：
//  1. 结构化思维：分层次提取信息
//  2. 上下文保留：确保关键信息不丢失
//  3. 清晰简洁：避免冗余，保持可读性
var systemPrompt = `你是一位专业的对话总结助手，擅长从多轮对话中提取和组织关键信息。

## 核心能力
- 提取结构化信息：用户背景、技术需求、重要决策
- 识别对话主题和上下文切换
- 生成清晰、结构化的总结
- 保持事实准确性，避免添加不存在的信息

## 总结原则
1. **准确性**：确保总结内容与实际对话一致
2. **完整性**：保留所有关键信息，不遗漏重要细节
3. **清晰性**：使用清晰、专业的语言
4. **结构化**：按照指定的格式组织信息

## 需要提取的内容
- **用户背景**：技术栈、工作场景、使用习惯
- **讨论主题**：讨论的主要问题和话题
- **关键决策**：达成的结论、商定的行动项
- **重要事实**：需要记住的数据、偏好、约束条件

## 需要避免的内容
- 不要包含问候语或客套话
- 不要添加对话中不存在的信息
- 不要使用模糊的语言（如"讨论了一些话题"）
- 不要过度压缩导致信息丢失
`

// userPrompt 定义具体的总结任务
//
// 设计要点（参考 Claude/Anthropic 的结构化方法）：
//  1. 结构化输出：分段组织信息
//  2. Few-shot 示例：提供清晰的格式参考
//  3. 软性指导：建议长度但不强制限制
var userPrompt = `请对以下对话进行结构化总结。

## 对话内容
{conversation}

## 输出格式（必须严格遵守）

**背景：**
[用户的技术背景、技术栈、工作场景]

**讨论：**
[讨论的主要话题、遇到的问题、探索的解决方案]

**结论：**
[达成的关键决策、行动项、重要结论]

**备注：**
[用户偏好、约束条件、其他需要记住的重要细节]

---

## 输出示例

**背景：**
用户是 Go 后端开发者，使用 GoFrame 框架开发微服务项目。

**讨论：**
讨论了如何集成腾讯云日志服务进行故障排查。探索了向量数据库和 Embedding 模型的选型方案。

**结论：**
决定采用 Milvus 作为向量数据库，使用豆包 Embedding 模型。要求响应时间控制在 3 秒内，支持流式输出。

**备注：**
用户偏好简洁的代码风格和详细的文档说明。

---

## 注意事项
- 简洁明了，同时保留所有关键信息
- 使用准确的技术术语，避免口语化表达
- 一般控制在 100-200 字，但根据对话复杂度可以适当调整
- 确保所有关键信息都被捕获
`
