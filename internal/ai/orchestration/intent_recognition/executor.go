// Package intent_recognition executor.go 执行器节点：接收 RouterOutput，按 IntentType 从 registry 取出 SubAgent 并执行。
// 执行结果（含错误）封装进 IntentOutput 而不向 Graph 上抛，保证 DAG 始终正常结束。
package intent_recognition

import (
	"context"
	"fmt"
	"sync/atomic"

	"Fo-Sentinel-Agent/internal/ai/orchestration/intent_recognition/core"

	"github.com/cloudwego/eino/compose"
)

// taskIDCounter 全局任务 ID 自增计数器，generateTaskID 调用时原子递增，保证跨请求唯一性
var taskIDCounter atomic.Int64

// newExecutorLambda 构建 Executor Lambda 节点。
// 分发逻辑：按 RouterOutput.IntentType 从 registry 获取 SubAgent；未注册时降级为 IntentChat（双重兜底，
// 补充 Router 层已做的第一重降级）。构建 Task 并调用 agent.Execute，Execute 内部错误
// 不向 Graph 上抛，而是封装在 IntentOutput.Error，由 Intent.Execute 调用方处理。
func newExecutorLambda() *compose.Lambda {
	lambda := compose.InvokableLambda(func(ctx context.Context, input *RouterOutput) (*IntentOutput, error) {
		// 按意图类型从注册表获取子 Agent，未注册时降级为 IntentChat（第二重兜底）
		agent := core.GetSubAgent(input.IntentType)
		if agent == nil {
			agent = core.GetSubAgent(IntentChat)
		}

		// 构建任务，携带全局唯一 ID 便于日志追踪
		task := &Task{
			ID:        generateTaskID(),
			Query:     input.Input.Query,
			Intent:    input.IntentType,
			SessionId: input.Input.SessionId, // 透传会话 ID，SubAgent 按此加载会话级记忆
		}

		// 执行子 Agent，错误封装进 IntentOutput 而不向 Graph 上抛，保证 DAG 始终正常结束
		result, err := agent.Execute(ctx, task, input.Input.Callback)

		var out *IntentOutput
		if err != nil {
			// 执行失败：保留任务 ID 和意图类型，Content 置空，Error 记录原因
			out = &IntentOutput{
				TaskID:  task.ID,
				Intent:  input.IntentType,
				Content: "",
				Error:   err,
			}
		} else {
			// 执行成功：透传子 Agent 返回的完整结果
			out = &IntentOutput{
				TaskID:  result.TaskID,
				Intent:  result.Intent,
				Content: result.Content,
				Error:   result.Error,
			}
		}
		return out, nil
	})
	return lambda
}

// generateTaskID 生成唯一任务 ID，格式为 task_<自增序号>，用于追踪与日志关联
func generateTaskID() string {
	return fmt.Sprintf("task_%d", taskIDCounter.Add(1))
}
