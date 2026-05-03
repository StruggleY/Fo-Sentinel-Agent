// Package engine AI 运维对外入口，触发两阶段 AI 运维流水线。
package engine

import (
	"context"

	"Fo-Sentinel-Agent/internal/ai/agent/ops_pipeline"
	"Fo-Sentinel-Agent/internal/ai/ops/actions"


	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

func init() {
	actions.SetAnalyzeFunc(ops_pipeline.RunEventAnalysis)
}

// TriggerForEvent 对事件触发 AI 自动运维（异步，已 resolved 的事件跳过）
func TriggerForEvent(ctx context.Context, event *dao.Event) {
	if event.Status == "resolved" {
		return
	}
	DirectRunForEvent(ctx, event)
}

// DirectRunForEvent 直接对事件执行 AI 运维，返回 RunID
func DirectRunForEvent(ctx context.Context, event *dao.Event) string {
	runID := uuid.New().String()
	go func() {
		if err := ops_pipeline.ExecuteRun(context.Background(), runID, event); err != nil {
			g.Log().Warningf(context.Background(), "[ops] 运维执行失败: %v", err)
		}
	}()
	return runID
}
