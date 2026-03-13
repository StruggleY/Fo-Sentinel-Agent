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

// systemPrompt 定义摘要助手的角色与核心规则。
//
// 设计原则：
//   - 短而精：只负责角色定位和输出约束，具体任务放 userPrompt
//   - 明确输出用途（机器读取，注入下一轮 AI 上下文）
var systemPrompt = `你是安全事件研判系统的对话记忆压缩器。

你的任务：将一段多轮安全分析对话压缩为简短摘要，供 AI 在后续对话轮次中作为历史背景参考。

规则：
- 只保留对后续分析有价值的信息：分析了哪些事件/漏洞、得出了什么结论、还有哪些未解决的问题
- 输出将直接注入 AI 的上下文（机器读取，非人类阅读），无需礼貌用语和过渡性文字
- 严禁添加对话中不存在的信息；某个字段无相关内容时直接省略该字段
`

// userPrompt 定义具体的摘要任务、输出格式和领域示例。
//
// 设计要点：
//  1. 三段式结构：贴合安全分析场景（事件 → 结论 → 待跟进），去掉与领域无关的"用户背景"字段
//  2. 示例使用真实安全领域内容（CVE），让模型校准输出风格和信息密度
//  3. 输出轻格式：减少 Markdown 装饰，摘要注入上下文时减少无效 token 消耗
//  4. "待跟进"可省略：无未解决问题时不强制填写，避免模型编造内容
var userPrompt = `请将以下对话压缩为结构化摘要。

## 对话内容
{conversation}

## 输出格式

**分析的事件/漏洞：**
[涉及的 CVE、威胁、告警、安全事件及其关键属性]

**结论与决策：**
[风险评估结果、处置建议、已确认的结论]

**待跟进：**
[未解决的问题、需进一步排查的内容；无则省略此字段]

---

## 示例输出

**分析的事件/漏洞：**
CVE-2024-3094（XZ Utils 供应链后门），CVSS 10.0，影响 liblzma 5.6.0/5.6.1，攻击者可通过 systemd 注入恶意代码实现 SSH 远程未授权访问。用户环境：Ubuntu 22.04，已确认版本不受影响。

**结论与决策：**
当前环境无需修复，建议加入资产监控订阅跟踪后续补丁动态。已生成威胁简报草稿，待用户确认后提交报告系统。

**待跟进：**
用户要求补充受影响发行版的完整列表，下一轮提供。

---

## 注意事项
- 目标长度 150-300 token，对话复杂时可适当超出，宁可完整不可遗漏关键结论
- 不含无关的寒暄、重复确认、格式说明等内容
- "待跟进"无内容时整体省略，不要写"无"或"暂无"
`
