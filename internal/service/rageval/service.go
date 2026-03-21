// Package rageval RAG 检索质量评估服务：KPI 聚合 + 用户反馈 CRUD。
package rageval

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	dao "Fo-Sentinel-Agent/internal/dao/mysql"
	"gorm.io/gorm"
)

// ── Dashboard 指标 ──────────────────────────────────────────────────────────

// TrendPoint 单个时间点的趋势数据。
type TrendPoint struct {
	Timestamp    string  `json:"timestamp"`      // 格式：2006-01-02 15（小时）或 2006-01-02（天）
	SuccessRate  float64 `json:"success_rate"`   // 0-1
	AvgLatencyMs int64   `json:"avg_latency_ms"` // 平均延迟（毫秒）
	NoDocRate    float64 `json:"no_doc_rate"`    // 0-1，无文档率
}

// DashboardMetrics KPI 汇总指标。
type DashboardMetrics struct {
	// KPI 卡片
	SuccessRate      float64 `json:"success_rate"`
	AvgLatencyMs     int64   `json:"avg_latency_ms"`
	P95LatencyMs     int64   `json:"p95_latency_ms"`
	CacheHitRate     float64 `json:"cache_hit_rate"`
	NoDocRate        float64 `json:"no_doc_rate"`
	TotalRuns        int64   `json:"total_runs"`
	AvgRetrievedDocs float64 `json:"avg_retrieved_docs"` // P1: 平均召回文档数
	AvgTopScore      float64 `json:"avg_top_score"`      // P1: 平均最高相似度分

	// 阈值状态（good/warning/bad）
	SuccessRateStatus string `json:"success_rate_status"`
	LatencyStatus     string `json:"latency_status"`
	NoDocRateStatus   string `json:"no_doc_rate_status"`

	// 趋势数据
	Trends []TrendPoint `json:"trends"`
}

// GetDashboard 聚合指定时间窗口内的 RAG KPI。
// window：24h / 7d / 30d，缺省 24h。
func GetDashboard(ctx context.Context, window string) (*DashboardMetrics, error) {
	db, err := dao.DB(ctx)
	if err != nil {
		return nil, err
	}

	since, groupByHour := parseWindow(window)

	// ── 查 agent_trace_runs ──────────────────────────────────────────────────
	var runs []dao.TraceRun
	db.Where("start_time >= ?", since).Order("start_time asc").Find(&runs)

	total := int64(len(runs))
	var successCount int64
	var totalDurationMs int64
	durations := make([]int64, 0, len(runs))
	for _, r := range runs {
		if r.Status == "success" {
			successCount++
		}
		if r.DurationMs > 0 {
			totalDurationMs += r.DurationMs
			durations = append(durations, r.DurationMs)
		}
	}

	successRate := float64(0)
	avgLatencyMs := int64(0)
	p95LatencyMs := int64(0)
	if total > 0 {
		successRate = float64(successCount) / float64(total)
	}
	if len(durations) > 0 {
		avgLatencyMs = totalDurationMs / int64(len(durations))
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		p95Idx := int(float64(len(durations)) * 0.95)
		if p95Idx >= len(durations) {
			p95Idx = len(durations) - 1
		}
		p95LatencyMs = durations[p95Idx]
	}

	// ── 查 agent_trace_nodes（CACHE / RETRIEVER）──────────────────────────────
	var cacheNodes []dao.TraceNode
	db.Where("start_time >= ? AND node_type = ?", since, "CACHE").Find(&cacheNodes)

	var cacheTotal, cacheHits int64
	for _, n := range cacheNodes {
		cacheTotal++
		if n.CacheHit {
			cacheHits++
		}
	}
	cacheHitRate := float64(0)
	if cacheTotal > 0 {
		cacheHitRate = float64(cacheHits) / float64(cacheTotal)
	}

	var retrieverNodes []dao.TraceNode
	db.Where("start_time >= ? AND node_type = ?", since, "RETRIEVER").Find(&retrieverNodes)

	var retrieverTotal, noDocCount int64
	var totalDocCount int64
	var totalTopScore float64
	for _, n := range retrieverNodes {
		retrieverTotal++
		if n.FinalTopK == 0 {
			noDocCount++
		}
		totalDocCount += int64(n.DocCount)
		totalTopScore += n.MaxVectorScore
	}
	noDocRate := float64(0)
	avgRetrievedDocs := float64(0)
	avgTopScore := float64(0)
	if retrieverTotal > 0 {
		noDocRate = float64(noDocCount) / float64(retrieverTotal)
		avgRetrievedDocs = float64(totalDocCount) / float64(retrieverTotal)
		avgTopScore = totalTopScore / float64(retrieverTotal)
	}

	// ── 阈值判断 ─────────────────────────────────────────────────────────────
	metrics := &DashboardMetrics{
		SuccessRate:       successRate,
		AvgLatencyMs:      avgLatencyMs,
		P95LatencyMs:      p95LatencyMs,
		CacheHitRate:      cacheHitRate,
		NoDocRate:         noDocRate,
		TotalRuns:         total,
		AvgRetrievedDocs:  avgRetrievedDocs,
		AvgTopScore:       avgTopScore,
		SuccessRateStatus: rateStatus(successRate, 0.99, 0.95),
		LatencyStatus:     latencyStatus(p95LatencyMs),
		NoDocRateStatus:   inverseRateStatus(noDocRate, 0.10, 0.30),
		Trends:            buildTrends(runs, since, groupByHour),
	}
	return metrics, nil
}

// ── Trace 链路列表 ─────────────────────────────────────────────────────────

// TraceItem 最近 RAG 链路简要信息。
type TraceItem struct {
	TraceID      string `json:"trace_id"`
	TraceName    string `json:"trace_name"`
	SessionID    string `json:"session_id"`
	Status       string `json:"status"`
	DurationMs   int64  `json:"duration_ms"`
	CacheHit     bool   `json:"cache_hit"`
	NoDoc        bool   `json:"no_doc"`
	StartTime    string `json:"start_time"`
	FeedbackVote int    `json:"feedback_vote"` // P2: 0=无反馈 1=点赞 -1=点踩
}

// ListTraces 返回分页的 trace 记录，支持 status/noDoc/cacheHit 过滤。
func ListTraces(ctx context.Context, page, pageSize int, status, noDoc, cacheHit string) ([]TraceItem, int64, error) {
	db, err := dao.DB(ctx)
	if err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 5
	}
	offset := (page - 1) * pageSize

	q := db.Model(&dao.TraceRun{})
	if status != "" {
		q = q.Where("status = ?", status)
	}

	var total int64
	q.Count(&total)

	var runs []dao.TraceRun
	q.Order("start_time desc").Offset(offset).Limit(pageSize).Find(&runs)

	// 批量取这些 trace 的 CACHE/RETRIEVER 节点
	traceIDs := make([]string, len(runs))
	for i, r := range runs {
		traceIDs[i] = r.TraceID
	}

	cacheHitMap := map[string]bool{}
	noDocMap := map[string]bool{}
	if len(traceIDs) > 0 {
		var nodes []dao.TraceNode
		db.Where("trace_id IN ? AND node_type IN ?", traceIDs, []string{"CACHE", "RETRIEVER"}).Find(&nodes)
		for _, n := range nodes {
			if n.NodeType == "CACHE" && n.CacheHit {
				cacheHitMap[n.TraceID] = true
			}
			if n.NodeType == "RETRIEVER" && n.FinalTopK == 0 {
				noDocMap[n.TraceID] = true
			}
		}
	}

	// P2: 按 noDoc/cacheHit 在内存中过滤（节点数据已加载）
	var filtered []dao.TraceRun
	for _, r := range runs {
		if noDoc == "true" && !noDocMap[r.TraceID] {
			continue
		}
		if noDoc == "false" && noDocMap[r.TraceID] {
			continue
		}
		if cacheHit == "true" && !cacheHitMap[r.TraceID] {
			continue
		}
		if cacheHit == "false" && cacheHitMap[r.TraceID] {
			continue
		}
		filtered = append(filtered, r)
	}

	// P2: 关联 session 反馈（取每个 session 最新一条）
	sessionIDs := make([]string, 0, len(filtered))
	for _, r := range filtered {
		if r.SessionID != "" {
			sessionIDs = append(sessionIDs, r.SessionID)
		}
	}
	feedbackMap := map[string]int{}
	if len(sessionIDs) > 0 {
		var feedbacks []dao.MessageFeedback
		db.Where("session_id IN ?", sessionIDs).Order("created_at desc").Find(&feedbacks)
		for _, f := range feedbacks {
			if _, exists := feedbackMap[f.SessionID]; !exists {
				feedbackMap[f.SessionID] = f.Vote
			}
		}
	}

	items := make([]TraceItem, len(filtered))
	for i, r := range filtered {
		items[i] = TraceItem{
			TraceID:      r.TraceID,
			TraceName:    r.TraceName,
			SessionID:    r.SessionID,
			Status:       r.Status,
			DurationMs:   r.DurationMs,
			CacheHit:     cacheHitMap[r.TraceID],
			NoDoc:        noDocMap[r.TraceID],
			StartTime:    r.StartTime.Format("2006-01-02 15:04:05"),
			FeedbackVote: feedbackMap[r.SessionID],
		}
	}
	return items, total, nil
}

// DeleteTrace 删除指定 traceID 的链路记录（TraceRun + TraceNode 硬删除）。
func DeleteTrace(ctx context.Context, traceID string) error {
	db, err := dao.DB(ctx)
	if err != nil {
		return err
	}
	if err := db.Where("trace_id = ?", traceID).Delete(&dao.TraceRun{}).Error; err != nil {
		return err
	}
	return db.Unscoped().Where("trace_id = ?", traceID).Delete(&dao.TraceNode{}).Error
}

// ── 用户反馈 CRUD ──────────────────────────────────────────────────────────

// SubmitFeedback 提交消息点赞/踩。
// 同一 (session_id, message_index) 只保留最新一条，重复提交时更新 vote，
// 传入 vote=0 表示取消反馈（直接删除该条记录）。
func SubmitFeedback(ctx context.Context, sessionID string, messageIndex, vote int, reason string) error {
	db, err := dao.DB(ctx)
	if err != nil {
		return err
	}
	if vote == 0 {
		// 取消反馈：删除该条记录（硬删除，不影响软删除字段）
		return db.Unscoped().
			Where("session_id = ? AND message_index = ?", sessionID, messageIndex).
			Delete(&dao.MessageFeedback{}).Error
	}
	// Upsert：已存在则更新 vote/reason，不存在则插入
	var existing dao.MessageFeedback
	err = db.Where("session_id = ? AND message_index = ?", sessionID, messageIndex).
		First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err == nil {
		// 记录已存在，更新
		return db.Model(&existing).Updates(map[string]any{
			"vote":   vote,
			"reason": reason,
		}).Error
	}
	// 记录不存在，插入
	return db.Create(&dao.MessageFeedback{
		SessionID:    sessionID,
		MessageIndex: messageIndex,
		Vote:         vote,
		Reason:       reason,
	}).Error
}

// FeedbackStats 反馈统计结果。
type FeedbackStats struct {
	LikeRate    float64          `json:"like_rate"`    // 0-1
	DislikeRate float64          `json:"dislike_rate"` // 0-1
	NoVoteRate  float64          `json:"no_vote_rate"` // 0-1（仅基于有反馈数据计算）
	Total       int64            `json:"total"`
	Recent      []RecentFeedback `json:"recent"`
}

// RecentFeedback 单条反馈摘要。
type RecentFeedback struct {
	SessionID    string `json:"session_id"`
	MessageIndex int    `json:"message_index"`
	Vote         int    `json:"vote"`
	Reason       string `json:"reason,omitempty"`
	CreatedAt    string `json:"created_at"`
}

// GetFeedbackStats 统计反馈数据。
func GetFeedbackStats(ctx context.Context) (*FeedbackStats, error) {
	db, err := dao.DB(ctx)
	if err != nil {
		return nil, err
	}

	var feedbacks []dao.MessageFeedback
	db.Order("created_at desc").Find(&feedbacks)

	var likes, dislikes int64
	for _, f := range feedbacks {
		if f.Vote == 1 {
			likes++
		} else if f.Vote == -1 {
			dislikes++
		}
	}
	total := int64(len(feedbacks))

	likeRate, dislikeRate := float64(0), float64(0)
	if total > 0 {
		likeRate = float64(likes) / float64(total)
		dislikeRate = float64(dislikes) / float64(total)
	}

	// 按 vote 分组合并：统计各类反馈数量，取最近 3 条有 reason 的记录展示
	type voteGroup struct {
		vote    int
		count   int
		samples []dao.MessageFeedback
	}
	groups := map[int]*voteGroup{
		1:  {vote: 1},
		-1: {vote: -1},
	}
	for _, f := range feedbacks {
		if g, ok := groups[f.Vote]; ok {
			g.count++
			if len(g.samples) < 3 {
				g.samples = append(g.samples, f)
			}
		}
	}
	var recentItems []RecentFeedback
	for _, vote := range []int{1, -1} {
		g := groups[vote]
		if g.count == 0 {
			continue
		}
		label := "有帮助"
		if vote == -1 {
			label = "没帮助"
		}
		// 找第一条有 reason 的样本作为代表，否则用默认文案
		reason := ""
		for _, s := range g.samples {
			if s.Reason != "" {
				reason = s.Reason
				break
			}
		}
		if reason == "" {
			reason = label
		}
		createdAt := ""
		if len(g.samples) > 0 {
			createdAt = g.samples[0].CreatedAt.Format("2006-01-02 15:04:05")
		}
		recentItems = append(recentItems, RecentFeedback{
			Vote:      vote,
			Reason:    fmt.Sprintf("%s（共 %d 条）", reason, g.count),
			CreatedAt: createdAt,
		})
	}

	return &FeedbackStats{
		LikeRate:    likeRate,
		DislikeRate: dislikeRate,
		NoVoteRate:  1 - likeRate - dislikeRate,
		Total:       total,
		Recent:      recentItems,
	}, nil
}

// ── 辅助函数 ───────────────────────────────────────────────────────────────

func parseWindow(window string) (since time.Time, byHour bool) {
	now := time.Now()
	switch window {
	case "7d":
		return now.Add(-7 * 24 * time.Hour), false
	case "30d":
		return now.Add(-30 * 24 * time.Hour), false
	default: // 24h
		return now.Add(-24 * time.Hour), true
	}
}

func buildTrends(runs []dao.TraceRun, since time.Time, byHour bool) []TrendPoint {
	type bucket struct {
		total   int
		success int
		latSum  int64
		noDoc   int
	}
	buckets := map[string]*bucket{}

	for _, r := range runs {
		var key string
		if byHour {
			key = r.StartTime.Format("01-02 15:00")
		} else {
			key = r.StartTime.Format("2006-01-02")
		}
		b := buckets[key]
		if b == nil {
			b = &bucket{}
			buckets[key] = b
		}
		b.total++
		if r.Status == "success" {
			b.success++
		}
		b.latSum += r.DurationMs
	}

	// 填充时间轴（无数据点也输出）
	now := time.Now()
	var keys []string
	if byHour {
		for t := since; t.Before(now); t = t.Add(time.Hour) {
			keys = append(keys, t.Format("01-02 15:00"))
		}
	} else {
		for t := since; t.Before(now); t = t.Add(24 * time.Hour) {
			keys = append(keys, t.Format("2006-01-02"))
		}
	}

	points := make([]TrendPoint, 0, len(keys))
	for _, k := range keys {
		b := buckets[k]
		if b == nil {
			points = append(points, TrendPoint{Timestamp: k, SuccessRate: 0, AvgLatencyMs: 0, NoDocRate: 0})
			continue
		}
		sr := float64(0)
		if b.total > 0 {
			sr = float64(b.success) / float64(b.total)
		}
		avg := int64(0)
		if b.success > 0 {
			avg = b.latSum / int64(b.total)
		}
		points = append(points, TrendPoint{
			Timestamp:    k,
			SuccessRate:  sr,
			AvgLatencyMs: avg,
		})
	}
	return points
}

func rateStatus(v, good, warn float64) string {
	if v >= good {
		return "good"
	}
	if v >= warn {
		return "warning"
	}
	return "bad"
}

func inverseRateStatus(v, good, warn float64) string {
	if v <= good {
		return "good"
	}
	if v <= warn {
		return "warning"
	}
	return "bad"
}

func latencyStatus(p95Ms int64) string {
	if p95Ms <= 5000 {
		return "good"
	}
	if p95Ms <= 15000 {
		return "warning"
	}
	return "bad"
}

// ── Trace 详情（P0）─────────────────────────────────────────────────────────

// TraceNodeItem 节点树中的单个节点。
type TraceNodeItem struct {
	NodeID       string  `json:"node_id"`
	ParentNodeID string  `json:"parent_node_id,omitempty"`
	Depth        int     `json:"depth"`
	NodeType     string  `json:"node_type"`
	NodeName     string  `json:"node_name"`
	Status       string  `json:"status"`
	DurationMs   int64   `json:"duration_ms"`
	ErrorMessage string  `json:"error_message,omitempty"`
	ModelName    string  `json:"model_name,omitempty"`
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	CostUSD      float64 `json:"cost_usd,omitempty"`
	CacheHit     bool    `json:"cache_hit,omitempty"`
	// RETRIEVER 专属
	FinalTopK      int             `json:"final_top_k,omitempty"`
	DocCount       int             `json:"doc_count,omitempty"`
	AvgVectorScore float64         `json:"avg_vector_score,omitempty"`
	MaxVectorScore float64         `json:"max_vector_score,omitempty"`
	RerankUsed     bool            `json:"rerank_used,omitempty"`
	AvgRerankScore float64         `json:"avg_rerank_score,omitempty"`
	RetrievedDocs  string          `json:"retrieved_docs,omitempty"`
	Children       []TraceNodeItem `json:"children,omitempty"`
}

// TraceDetail Trace 详情。
type TraceDetail struct {
	TraceID           string          `json:"trace_id"`
	TraceName         string          `json:"trace_name"`
	SessionID         string          `json:"session_id"`
	QueryText         string          `json:"query_text"`
	Status            string          `json:"status"`
	DurationMs        int64           `json:"duration_ms"`
	TotalInputTokens  int             `json:"total_input_tokens"`
	TotalOutputTokens int             `json:"total_output_tokens"`
	EstimatedCostUSD  float64         `json:"estimated_cost_usd"`
	StartTime         string          `json:"start_time"`
	Nodes             []TraceNodeItem `json:"nodes"`
	FeedbackVote      int             `json:"feedback_vote"`
}

// GetTraceDetail 返回指定 traceID 的完整链路详情（含节点树）。
func GetTraceDetail(ctx context.Context, traceID string) (*TraceDetail, error) {
	db, err := dao.DB(ctx)
	if err != nil {
		return nil, err
	}

	var run dao.TraceRun
	if err := db.Where("trace_id = ?", traceID).First(&run).Error; err != nil {
		return nil, fmt.Errorf("trace not found: %w", err)
	}

	var nodes []dao.TraceNode
	db.Where("trace_id = ?", traceID).Order("depth asc, start_time asc").Find(&nodes)

	// 构建节点树
	nodeMap := map[string]*TraceNodeItem{}
	for i := range nodes {
		n := &nodes[i]
		item := &TraceNodeItem{
			NodeID:         n.NodeID,
			ParentNodeID:   n.ParentNodeID,
			Depth:          n.Depth,
			NodeType:       n.NodeType,
			NodeName:       n.NodeName,
			Status:         n.Status,
			DurationMs:     n.DurationMs,
			ErrorMessage:   n.ErrorMessage,
			ModelName:      n.ModelName,
			InputTokens:    n.InputTokens,
			OutputTokens:   n.OutputTokens,
			CostUSD:        n.CostUSD,
			CacheHit:       n.CacheHit,
			FinalTopK:      n.FinalTopK,
			DocCount:       n.DocCount,
			AvgVectorScore: n.AvgVectorScore,
			MaxVectorScore: n.MaxVectorScore,
			RerankUsed:     n.RerankUsed,
			AvgRerankScore: n.AvgRerankScore,
			RetrievedDocs:  n.RetrievedDocs,
		}
		nodeMap[n.NodeID] = item
	}

	var roots []TraceNodeItem
	for i := range nodes {
		n := &nodes[i]
		item := nodeMap[n.NodeID]
		if n.ParentNodeID == "" {
			roots = append(roots, *item)
		} else if parent, ok := nodeMap[n.ParentNodeID]; ok {
			parent.Children = append(parent.Children, *item)
		} else {
			roots = append(roots, *item)
		}
	}

	// P2: 关联反馈
	feedbackVote := 0
	if run.SessionID != "" {
		var fb dao.MessageFeedback
		err := db.Where("session_id = ?", run.SessionID).Order("created_at desc").First(&fb).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("query feedback: %w", err)
		}
		if err == nil {
			feedbackVote = fb.Vote
		}
	}

	return &TraceDetail{
		TraceID:           run.TraceID,
		TraceName:         run.TraceName,
		SessionID:         run.SessionID,
		QueryText:         run.QueryText,
		Status:            run.Status,
		DurationMs:        run.DurationMs,
		TotalInputTokens:  run.TotalInputTokens,
		TotalOutputTokens: run.TotalOutputTokens,
		EstimatedCostUSD:  run.EstimatedCostUSD,
		StartTime:         run.StartTime.Format("2006-01-02 15:04:05"),
		Nodes:             roots,
		FeedbackVote:      feedbackVote,
	}, nil
}
