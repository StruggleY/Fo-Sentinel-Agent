package solve_pipeline

import (
	"Fo-Sentinel-Agent/internal/ai/models"
	"context"

	"github.com/cloudwego/eino/components/model"
)

// newSolveModel 创建解决方案生成 Agent 使用的 LLM 模型实例。
// 使用 DeepSeek V3 Think（深度推理版）：单事件分析需要多步推导攻击路径和修复路径，
// Think 模型的推理链更充分，方案质量优于 Quick 模型。
func newSolveModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	return models.OpenAIForDeepSeekV31Think(ctx)
}
