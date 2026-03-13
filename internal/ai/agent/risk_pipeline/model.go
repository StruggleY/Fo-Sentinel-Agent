package risk_pipeline

import (
	"Fo-Sentinel-Agent/internal/ai/models"
	"context"

	"github.com/cloudwego/eino/components/model"
)

// newRiskModel 创建风险评估智能体使用的 LLM 模型实例
// 使用 DeepSeek V3 Think 模型（深度推理版），适合需要深度分析 CVE、评估攻击路径和影响范围的场景
// Think 模型进行更充分的推理链，风险评分和分析结论更准确，但首 Token 延迟较高
// 返回支持工具调用的聊天模型接口，用于 ReAct Agent 的推理和工具调用
func newRiskModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	return models.OpenAIForDeepSeekV31Think(ctx)
}
