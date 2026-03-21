// Package v1 定义链路追踪 API 的请求/响应类型。
package v1

import "github.com/gogf/gf/v2/frame/g"

// ListReq 链路运行记录列表请求
type ListReq struct {
	g.Meta    `path:"/trace/v1/list" method:"GET"`
	Page      int    `json:"page"      d:"1"`
	PageSize  int    `json:"pageSize"  d:"20"`
	Status    string `json:"status"`
	TraceId   string `json:"traceId"`
	SessionId string `json:"sessionId"`
}

// ListRes 链路运行记录列表响应
type ListRes struct {
	Total int64        `json:"total"`
	List  []TraceRunVO `json:"list"`
}

// TraceRunVO 链路运行记录 VO（视图对象）
type TraceRunVO struct {
	TraceId           string  `json:"traceId"`
	TraceName         string  `json:"traceName"`
	EntryPoint        string  `json:"entryPoint"`
	Status            string  `json:"status"`
	DurationMs        int64   `json:"durationMs"`
	StartTime         string  `json:"startTime"`
	SessionId         string  `json:"sessionId"`
	QueryText         string  `json:"queryText"`
	TotalInputTokens  int     `json:"totalInputTokens"`
	TotalOutputTokens int     `json:"totalOutputTokens"`
	TotalCachedTokens int     `json:"totalCachedTokens"`
	EstimatedCostUSD  float64 `json:"estimatedCostUsd"`
	ErrorCode         string  `json:"errorCode,omitempty"`
	ErrorMessage      string  `json:"errorMessage,omitempty"`
	Tags              string  `json:"tags"`
}

// DetailReq 链路详情请求
type DetailReq struct {
	g.Meta  `path:"/trace/v1/detail" method:"GET"`
	TraceId string `json:"traceId" v:"required"`
}

// DetailRes 链路详情响应（包含节点树）
type DetailRes struct {
	TraceRunVO
	Nodes []TraceNodeVO `json:"nodes"`
}

// TraceNodeVO 链路节点 VO
type TraceNodeVO struct {
	NodeId       string `json:"nodeId"`
	ParentNodeId string `json:"parentNodeId"`
	NodeType     string `json:"nodeType"`
	NodeName     string `json:"nodeName"`
	Depth        int    `json:"depth"`
	Status       string `json:"status"`
	DurationMs   int64  `json:"durationMs"`
	StartTime    string `json:"startTime"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorType    string `json:"errorType,omitempty"`
	// LLM 专属
	ModelName      string  `json:"modelName,omitempty"`
	InputTokens    int     `json:"inputTokens,omitempty"`
	OutputTokens   int     `json:"outputTokens,omitempty"`
	CachedTokens   int     `json:"cachedTokens,omitempty"`
	CostUSD        float64 `json:"costUsd,omitempty"`
	CompletionText string  `json:"completionText,omitempty"`
	// RETRIEVER 专属
	QueryText     string `json:"queryText,omitempty"`
	RetrievedDocs string `json:"retrievedDocs,omitempty"`
	FinalTopK     int    `json:"finalTopK,omitempty"`
	CacheHit      bool   `json:"cacheHit,omitempty"`
	// 通用
	Metadata string `json:"metadata,omitempty"`
}

// StatsReq 链路统计请求
type StatsReq struct {
	g.Meta `path:"/trace/v1/stats" method:"GET"`
	Days   int `json:"days" d:"7"` // 最近 N 天
}

// StatsRes 链路统计响应
type StatsRes struct {
	TotalRuns         int64   `json:"totalRuns"`
	SuccessRuns       int64   `json:"successRuns"`
	ErrorRuns         int64   `json:"errorRuns"`
	AvgDurationMs     float64 `json:"avgDurationMs"`
	P95DurationMs     int64   `json:"p95DurationMs"`
	TotalInputTokens  int64   `json:"totalInputTokens"`
	TotalOutputTokens int64   `json:"totalOutputTokens"`
	TotalCostUSD      float64 `json:"totalCostUsd"`
	AvgCostUSD        float64 `json:"avgCostUsd"`
	ErrorRate         float64 `json:"errorRate"` // 0~1
}

// BatchDeleteReq 批量删除链路请求
type BatchDeleteReq struct {
	g.Meta   `path:"/trace/v1/batch_delete" method:"DELETE"`
	TraceIds []string `json:"traceIds" v:"required|min-length:1"`
}

// BatchDeleteRes 批量删除响应
type BatchDeleteRes struct {
	Deleted int64 `json:"deleted"` // 实际删除条数
}
