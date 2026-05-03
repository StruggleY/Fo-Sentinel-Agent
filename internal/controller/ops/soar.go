// Package ops AI 智能运维控制器
package ops

import (
	"context"

	soarv1 "Fo-Sentinel-Agent/api/ops/v1"
	"Fo-Sentinel-Agent/internal/ai/ops/engine"
	dao "Fo-Sentinel-Agent/internal/dao/mysql"

	"github.com/gogf/gf/v2/errors/gerror"
)

type ControllerV1 struct{}

func NewV1() *ControllerV1 { return &ControllerV1{} }

func (c *ControllerV1) ListRuns(ctx context.Context, req *soarv1.ListRunsReq) (*soarv1.ListRunsRes, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	list, err := dao.ListRuns(ctx, limit)
	if err != nil {
		return nil, err
	}
	items := make([]soarv1.RunItem, 0, len(list))
	for _, r := range list {
		items = append(items, toRunItem(r, nil))
	}
	return &soarv1.ListRunsRes{Items: items}, nil
}

func (c *ControllerV1) GetRun(ctx context.Context, req *soarv1.GetRunReq) (*soarv1.GetRunRes, error) {
	r, err := dao.GetRun(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	steps, _ := dao.GetRunSteps(ctx, req.ID)
	return &soarv1.GetRunRes{Item: toRunItem(*r, steps)}, nil
}

func (c *ControllerV1) GetStats(ctx context.Context, _ *soarv1.GetStatsReq) (*soarv1.GetStatsRes, error) {
	s, err := dao.GetOpsStats(ctx)
	if err != nil {
		return nil, err
	}
	return &soarv1.GetStatsRes{
		TotalRuns: s.TotalRuns, SuccessRuns: s.SuccessRuns, FailedRuns: s.FailedRuns,
	}, nil
}

func (c *ControllerV1) ClearRuns(ctx context.Context, _ *soarv1.ClearRunsReq) (*soarv1.ClearRunsRes, error) {
	return &soarv1.ClearRunsRes{}, dao.ClearRuns(ctx)
}

func (c *ControllerV1) DeleteRun(ctx context.Context, req *soarv1.DeleteRunReq) (*soarv1.DeleteRunRes, error) {
	return &soarv1.DeleteRunRes{}, dao.DeleteRun(ctx, req.ID)
}

func (c *ControllerV1) DirectRunForEvent(ctx context.Context, req *soarv1.DirectRunForEventReq) (*soarv1.DirectRunForEventRes, error) {
	event, err := dao.GetEventByID(ctx, req.EventID)
	if err != nil {
		return nil, gerror.New("事件不存在")
	}
	runID := engine.DirectRunForEvent(ctx, event)
	return &soarv1.DirectRunForEventRes{RunID: runID}, nil
}

func toRunItem(r dao.OpsRun, steps []dao.OpsRunStep) soarv1.RunItem {
	item := soarv1.RunItem{
		ID: r.ID, PlaybookID: r.PlaybookID, EventID: r.EventID,
		EventTitle: r.EventTitle, EventSeverity: r.EventSeverity, PlanSummary: r.PlanSummary,
		Status: r.Status, ErrorMsg: r.ErrorMsg, DurationMs: r.DurationMs,
		StartedAt: r.StartedAt.Format("2006-01-02 15:04:05"),
	}
	if r.FinishedAt != nil {
		item.FinishedAt = r.FinishedAt.Format("2006-01-02 15:04:05")
	}
	for _, s := range steps {
		item.Steps = append(item.Steps, soarv1.RunStepItem{
			ID: s.ID, StepOrder: s.StepOrder, ActionType: s.ActionType,
			Status: s.Status, Output: s.Output, ErrorMsg: s.ErrorMsg,
			RetryCount: s.RetryCount, DurationMs: s.DurationMs,
			StartedAt: s.StartedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return item
}
