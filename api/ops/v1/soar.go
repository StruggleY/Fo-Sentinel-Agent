// Package v1 OPS API 定义
package v1

import "github.com/gogf/gf/v2/frame/g"

// ---- Runs ----

type RunItem struct {
	ID            string        `json:"id"`
	PlaybookID    string        `json:"playbook_id"`
	EventID       string        `json:"event_id"`
	EventTitle    string        `json:"event_title,omitempty"`
	EventSeverity string        `json:"event_severity,omitempty"`
	PlanSummary   string        `json:"plan_summary,omitempty"`
	Status        string        `json:"status"`
	ErrorMsg      string        `json:"error_msg,omitempty"`
	DurationMs    int64         `json:"duration_ms"`
	StartedAt     string        `json:"started_at"`
	FinishedAt    string        `json:"finished_at,omitempty"`
	Steps         []RunStepItem `json:"steps,omitempty"`
}

type RunStepItem struct {
	ID         string `json:"id"`
	StepOrder  int    `json:"step_order"`
	ActionType string `json:"action_type"`
	Status     string `json:"status"`
	Output     string `json:"output,omitempty"`
	ErrorMsg   string `json:"error_msg,omitempty"`
	RetryCount int    `json:"retry_count"`
	DurationMs int64  `json:"duration_ms"`
	StartedAt  string `json:"started_at"`
}

type ListRunsReq struct {
	g.Meta `path:"/ops/v1/runs" method:"get"`
	Limit  int `p:"limit" d:"20"`
}
type ListRunsRes struct {
	Items []RunItem `json:"items"`
}

type GetRunReq struct {
	g.Meta `path:"/ops/v1/runs/{id}" method:"get"`
	ID     string `p:"id" v:"required"`
}
type GetRunRes struct {
	Item RunItem `json:"item"`
}

// ---- Stats ----

type GetStatsReq struct {
	g.Meta `path:"/ops/v1/stats" method:"get"`
}
type GetStatsRes struct {
	TotalRuns   int64 `json:"total_runs"`
	SuccessRuns int64 `json:"success_runs"`
	FailedRuns  int64 `json:"failed_runs"`
}

// ---- ClearRuns ----

type ClearRunsReq struct {
	g.Meta `path:"/ops/v1/runs" method:"delete"`
}
type ClearRunsRes struct{}

// ---- DeleteRun ----

type DeleteRunReq struct {
	g.Meta `path:"/ops/v1/runs/{id}" method:"delete"`
	ID     string `p:"id" v:"required"`
}
type DeleteRunRes struct{}

// ---- DirectRunForEvent：直接对事件执行 AI 运维，不依赖 Playbook 匹配 ----

type DirectRunForEventReq struct {
	g.Meta  `path:"/ops/v1/runs/direct" method:"post"`
	EventID string `json:"event_id" v:"required"`
}
type DirectRunForEventRes struct {
	RunID string `json:"run_id"`
}
