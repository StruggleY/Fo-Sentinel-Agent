// Package trace 提供链路追踪查询 HTTP 控制器。
package trace

import (
	"context"
	"sort"
	"time"

	v1 "Fo-Sentinel-Agent/api/trace/v1"
	dao "Fo-Sentinel-Agent/internal/dao/mysql"
)

// ControllerV1 链路追踪控制器
type ControllerV1 struct{}

// NewV1 创建控制器实例
func NewV1() *ControllerV1 {
	return &ControllerV1{}
}

// List 分页查询链路运行记录，支持 status/traceId/sessionId 过滤，按 created_at DESC 排序
func (c *ControllerV1) List(ctx context.Context, req *v1.ListReq) (*v1.ListRes, error) {
	db, err := dao.DB(ctx)
	if err != nil {
		return &v1.ListRes{Total: 0, List: []v1.TraceRunVO{}}, nil
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := db.Model(&dao.TraceRun{})
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.TraceId != "" {
		query = query.Where("trace_id = ?", req.TraceId)
	}
	if req.SessionId != "" {
		query = query.Where("session_id = ?", req.SessionId)
	}

	var total int64
	query.Count(&total)

	var runs []dao.TraceRun
	query.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&runs)

	list := make([]v1.TraceRunVO, 0, len(runs))
	for _, r := range runs {
		list = append(list, toTraceRunVO(&r))
	}
	return &v1.ListRes{Total: total, List: list}, nil
}

// Detail 查询单条链路详情（TraceRun + 所有 TraceNode，按 start_time ASC）
func (c *ControllerV1) Detail(ctx context.Context, req *v1.DetailReq) (*v1.DetailRes, error) {
	db, err := dao.DB(ctx)
	if err != nil {
		return nil, err
	}

	var run dao.TraceRun
	if result := db.Where("trace_id = ?", req.TraceId).First(&run); result.Error != nil {
		return nil, result.Error
	}

	var nodes []dao.TraceNode
	db.Where("trace_id = ?", req.TraceId).Order("start_time ASC").Find(&nodes)

	nodeVOs := make([]v1.TraceNodeVO, 0, len(nodes))
	for _, n := range nodes {
		nodeVOs = append(nodeVOs, toTraceNodeVO(&n))
	}

	return &v1.DetailRes{
		TraceRunVO: toTraceRunVO(&run),
		Nodes:      nodeVOs,
	}, nil
}

// Stats 聚合查询最近 N 天的统计数据（COUNT/AVG/SUM，P95 在应用层计算）
func (c *ControllerV1) Stats(ctx context.Context, req *v1.StatsReq) (*v1.StatsRes, error) {
	db, err := dao.DB(ctx)
	if err != nil {
		return &v1.StatsRes{}, nil
	}

	days := req.Days
	if days <= 0 {
		days = 7
	}
	since := time.Now().AddDate(0, 0, -days)

	// 基础统计
	type statsRow struct {
		Total     int64
		Success   int64
		Errors    int64
		AvgDur    float64
		TotalIn   int64
		TotalOut  int64
		TotalCost float64
	}

	var row statsRow
	db.Model(&dao.TraceRun{}).
		Where("created_at >= ?", since).
		Select("COUNT(*) as total, " +
			"SUM(CASE WHEN status='success' THEN 1 ELSE 0 END) as success, " +
			"SUM(CASE WHEN status='error' THEN 1 ELSE 0 END) as errors, " +
			"AVG(duration_ms) as avg_dur, " +
			"SUM(total_input_tokens) as total_in, " +
			"SUM(total_output_tokens) as total_out, " +
			"SUM(estimated_cost_usd) as total_cost").
		Scan(&row)

	// P95 耗时（应用层计算）
	var durations []int64
	db.Model(&dao.TraceRun{}).
		Where("created_at >= ? AND status = 'success'", since).
		Pluck("duration_ms", &durations)

	p95 := calcP95(durations)

	var errorRate float64
	if row.Total > 0 {
		errorRate = float64(row.Errors) / float64(row.Total)
	}
	var avgCost float64
	if row.Total > 0 {
		avgCost = row.TotalCost / float64(row.Total)
	}

	return &v1.StatsRes{
		TotalRuns:         row.Total,
		SuccessRuns:       row.Success,
		ErrorRuns:         row.Errors,
		AvgDurationMs:     row.AvgDur,
		P95DurationMs:     p95,
		TotalInputTokens:  row.TotalIn,
		TotalOutputTokens: row.TotalOut,
		TotalCostUSD:      row.TotalCost,
		AvgCostUSD:        avgCost,
		ErrorRate:         errorRate,
	}, nil
}

// BatchDelete 批量删除指定 trace_id 的链路记录（同时删除关联节点）
func (c *ControllerV1) BatchDelete(ctx context.Context, req *v1.BatchDeleteReq) (*v1.BatchDeleteRes, error) {
	db, err := dao.DB(ctx)
	if err != nil {
		return nil, err
	}
	if len(req.TraceIds) == 0 {
		return &v1.BatchDeleteRes{Deleted: 0}, nil
	}

	// 先删除关联节点（硬删除，trace 日志不需要软删除）
	db.Where("trace_id IN ?", req.TraceIds).Delete(&dao.TraceNode{})

	// 再删除 TraceRun
	result := db.Where("trace_id IN ?", req.TraceIds).Delete(&dao.TraceRun{})
	if result.Error != nil {
		return nil, result.Error
	}
	return &v1.BatchDeleteRes{Deleted: result.RowsAffected}, nil
}

// calcP95 在应用层计算第 95 百分位耗时
func calcP95(durations []int64) int64 {
	if len(durations) == 0 {
		return 0
	}
	sorted := make([]int64, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)) * 0.95)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// toTraceRunVO 将 GORM 模型转换为 VO
func toTraceRunVO(r *dao.TraceRun) v1.TraceRunVO {
	return v1.TraceRunVO{
		TraceId:           r.TraceID,
		TraceName:         r.TraceName,
		EntryPoint:        r.EntryPoint,
		Status:            r.Status,
		DurationMs:        r.DurationMs,
		StartTime:         r.StartTime.Format("2006-01-02T15:04:05.000Z07:00"),
		SessionId:         r.SessionID,
		QueryText:         r.QueryText,
		TotalInputTokens:  r.TotalInputTokens,
		TotalOutputTokens: r.TotalOutputTokens,
		TotalCachedTokens: r.TotalCachedTokens,
		EstimatedCostUSD:  r.EstimatedCostUSD,
		ErrorCode:         r.ErrorCode,
		ErrorMessage:      r.ErrorMessage,
		Tags:              r.Tags,
	}
}

// toTraceNodeVO 将 GORM 模型转换为 VO
func toTraceNodeVO(n *dao.TraceNode) v1.TraceNodeVO {
	vo := v1.TraceNodeVO{
		NodeId:         n.NodeID,
		ParentNodeId:   n.ParentNodeID,
		NodeType:       n.NodeType,
		NodeName:       n.NodeName,
		Depth:          n.Depth,
		Status:         n.Status,
		DurationMs:     n.DurationMs,
		StartTime:      n.StartTime.Format("2006-01-02T15:04:05.000Z07:00"),
		ErrorMessage:   n.ErrorMessage,
		ErrorCode:      n.ErrorCode,
		ErrorType:      n.ErrorType,
		ModelName:      n.ModelName,
		InputTokens:    n.InputTokens,
		OutputTokens:   n.OutputTokens,
		CachedTokens:   n.CachedTokens,
		CostUSD:        n.CostUSD,
		CompletionText: n.CompletionText,
		QueryText:      n.QueryText,
		RetrievedDocs:  n.RetrievedDocs,
		FinalTopK:      n.FinalTopK,
		CacheHit:       n.CacheHit,
		Metadata:       n.Metadata,
	}
	return vo
}
