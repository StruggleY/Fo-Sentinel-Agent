// plan_subagent.go 规划执行 SubAgent：处理复杂多步骤任务，调用 plan_pipeline 进行规划与执行。
package subagents

import (
	"Fo-Sentinel-Agent/internal/ai/agent/plan_pipeline"
	"Fo-Sentinel-Agent/internal/ai/orchestration/intent_recognition/core"
	"context"
	"strings"
)

// PlanSubAgent 规划执行 SubAgent，调用 plan_pipeline.BuildPlanAgent。
type PlanSubAgent struct{}

func (a *PlanSubAgent) Name() core.IntentType { return core.IntentPlan }

// Execute 调用 plan_pipeline，按步骤通过 callback 流式推送各阶段 details。
func (a *PlanSubAgent) Execute(ctx context.Context, task *core.Task, callback core.StreamCallback) (*core.Result, error) {
	callback(core.IntentStatus, "[Plan Agent 规划执行...]")

	result, details, err := plan_pipeline.BuildPlanAgent(ctx, task.Query)
	if err != nil {
		return &core.Result{TaskID: task.ID, Intent: core.IntentPlan, Error: err}, err
	}

	for _, d := range details {
		callback(core.IntentPlan, d+"\n")
	}

	content := strings.Join(details, "\n") + "\n" + result
	return &core.Result{TaskID: task.ID, Intent: core.IntentPlan, Content: content}, nil
}

// init 在包加载时自动将 PlanSubAgent 注册到 registry
func init() { core.RegisterSubAgent(&PlanSubAgent{}) }
