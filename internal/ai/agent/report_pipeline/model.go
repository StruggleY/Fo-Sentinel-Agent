package report_pipeline

import (
	"Fo-Sentinel-Agent/internal/ai/models"
	"context"

	"github.com/cloudwego/eino/components/model"
)

// newReportModel 创建报告生成智能体使用的 LLM 模型实例
// 使用 DeepSeek V3 Think 模型（深度推理版），适合需要生成结构完整、内容丰富的长篇报告场景
// Think 模型在回答前进行更长的内部推理链，输出质量更高，但首 Token 延迟较高
// 返回支持工具调用的聊天模型接口，用于 ReAct Agent 的推理和工具调用
func newReportModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	return models.OpenAIForDeepSeekV31Think(ctx)
}
