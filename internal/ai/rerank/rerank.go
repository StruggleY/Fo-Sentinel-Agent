// Package rerank qwen3-rerank 文档重排序客户端
//
// 向量检索基于余弦相似度，擅长模糊语义匹配但不擅长精确相关性排序。
// Rerank 模型专为「查询-文档相关性」任务训练，从候选池中精选最相关文档，
// 实现「召回率+精准度」双提升。
//
// 架构位置：
//
//	MultiRetrieve → [候选文档池，~10个] → Rerank → [精选文档，3个] → LLM Prompt
//
// API 文档：https://www.alibabacloud.com/help/zh/model-studio/text-rerank-api
// 端点：POST https://dashscope.aliyuncs.com/compatible-api/v1/reranks
package rerank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// RerankResult 重排结果，携带文档和相关性分数。
type RerankResult struct {
	Doc   *schema.Document
	Score float64 // relevance_score，来自 qwen3-rerank API
}

var (
	globalRerankClient *Client
	rerankOnce         sync.Once
	rerankInitErr      error
)

// Client qwen3-rerank 文档重排序客户端（单例）。
type Client struct {
	apiKey   string
	baseURL  string
	model    string
	instruct string
}

// GetClient 返回全局单例 Client。
// retriever.rerank.enabled=false 时返回 (nil, nil)，调用方通过 nil 判断跳过重排。
func GetClient(ctx context.Context) (*Client, error) {
	rerankOnce.Do(func() {
		enabled, _ := g.Cfg().Get(ctx, "retriever.rerank.enabled")
		if !enabled.Bool() {
			return
		}

		apiKey, _ := g.Cfg().Get(ctx, "retriever.rerank.api_key")
		modelName, _ := g.Cfg().Get(ctx, "retriever.rerank.model")
		baseURL, _ := g.Cfg().Get(ctx, "retriever.rerank.base_url")
		instruct, _ := g.Cfg().Get(ctx, "retriever.rerank.instruct")

		ak := apiKey.String()
		mn := modelName.String()
		bu := baseURL.String()

		if ak == "" {
			rerankInitErr = fmt.Errorf("[Rerank] retriever.rerank.api_key 未配置")
			return
		}
		if mn == "" {
			rerankInitErr = fmt.Errorf("[Rerank] retriever.rerank.model 未配置")
			return
		}
		if bu == "" {
			rerankInitErr = fmt.Errorf("[Rerank] retriever.rerank.base_url 未配置")
			return
		}

		globalRerankClient = &Client{apiKey: ak, baseURL: bu, model: mn, instruct: instruct.String()}
		g.Log().Infof(ctx, "[Rerank] 客户端初始化成功 | model=%s | baseURL=%s", mn, bu)
	})
	return globalRerankClient, rerankInitErr
}

// ── API 请求/响应结构 ──────────────────────────────────────────────────────────
// 官方格式：扁平请求体，output.results 响应

type rerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n"`
	Instruct  string   `json:"instruct,omitempty"`
}

type rerankDocument struct {
	Text string `json:"text"`
}

type rerankResult struct {
	Index          int            `json:"index"`
	RelevanceScore float64        `json:"relevance_score"`
	Document       rerankDocument `json:"document"`
}

type rerankResponse struct {
	Results []rerankResult `json:"results"`
	Usage   struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	ID string `json:"id"`
}

type rerankErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

// Rerank 调用 qwen3-rerank API 对文档按相关性重排序，返回 topN 个最相关文档。
// 任意环节失败时降级返回原始列表前 topN 个，保证主流程不中断。
func (r *Client) Rerank(ctx context.Context, query string, docs []*schema.Document, topN int) []RerankResult {
	if len(docs) == 0 {
		fallback := make([]RerankResult, len(docs))
		for i, d := range docs {
			fallback[i] = RerankResult{Doc: d, Score: 0}
		}
		return fallback
	}

	// topN <= 0 无意义；topN >= len(docs) 无需截断也无需重排，直接返回
	if topN <= 0 || topN >= len(docs) {
		g.Log().Debugf(ctx, "[Rerank] 跳过重排 | topN=%d | docs=%d", topN, len(docs))
		fallback := make([]RerankResult, len(docs))
		for i, d := range docs {
			fallback[i] = RerankResult{Doc: d, Score: 0}
		}
		return fallback
	}

	texts := make([]string, len(docs))
	for i, d := range docs {
		texts[i] = d.Content
	}

	body, err := json.Marshal(rerankRequest{
		Model:     r.model,
		Query:     query,
		Documents: texts,
		TopN:      topN,
		Instruct:  r.instruct,
	})
	if err != nil {
		g.Log().Warningf(ctx, "[Rerank] 序列化失败，降级: %v", err)
		n := topN
		if n > len(docs) {
			n = len(docs)
		}
		fallback := make([]RerankResult, n)
		for i := range fallback {
			fallback[i] = RerankResult{Doc: docs[i], Score: 0}
		}
		return fallback
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/reranks", bytes.NewReader(body))
	if err != nil {
		g.Log().Warningf(ctx, "[Rerank] 构建请求失败，降级: %v", err)
		n := topN
		if n > len(docs) {
			n = len(docs)
		}
		fallback := make([]RerankResult, n)
		for i := range fallback {
			fallback[i] = RerankResult{Doc: docs[i], Score: 0}
		}
		return fallback
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	g.Log().Debugf(ctx, "[Rerank] 发送请求 | url=%s | body_len=%d",
		req.URL.String(), len(body))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		g.Log().Warningf(ctx, "[Rerank] 请求失败，降级: %v", err)
		n := topN
		if n > len(docs) {
			n = len(docs)
		}
		fallback := make([]RerankResult, n)
		for i := range fallback {
			fallback[i] = RerankResult{Doc: docs[i], Score: 0}
		}
		return fallback
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		g.Log().Warningf(ctx, "[Rerank] 读取响应失败，降级: %v", err)
		n := topN
		if n > len(docs) {
			n = len(docs)
		}
		fallback := make([]RerankResult, n)
		for i := range fallback {
			fallback[i] = RerankResult{Doc: docs[i], Score: 0}
		}
		return fallback
	}

	// 非 200 时打印完整响应体，便于排查问题
	if resp.StatusCode != http.StatusOK {
		var errResp rerankErrorResponse
		if json.Unmarshal(data, &errResp) == nil && errResp.Code != "" {
			g.Log().Warningf(ctx, "[Rerank] API错误，降级 | status=%d | code=%s | msg=%s | requestId=%s | rawBody=%s",
				resp.StatusCode, errResp.Code, errResp.Message, errResp.RequestID, string(data))
		} else {
			g.Log().Warningf(ctx, "[Rerank] API错误，降级 | status=%d | rawBody=%s", resp.StatusCode, string(data))
		}
		n := topN
		if n > len(docs) {
			n = len(docs)
		}
		fallback := make([]RerankResult, n)
		for i := range fallback {
			fallback[i] = RerankResult{Doc: docs[i], Score: 0}
		}
		return fallback
	}

	var rerankResp rerankResponse
	if err := json.Unmarshal(data, &rerankResp); err != nil {
		g.Log().Warningf(ctx, "[Rerank] 解析响应失败，降级: %v", err)
		n := topN
		if n > len(docs) {
			n = len(docs)
		}
		fallback := make([]RerankResult, n)
		for i := range fallback {
			fallback[i] = RerankResult{Doc: docs[i], Score: 0}
		}
		return fallback
	}

	results := rerankResp.Results
	if len(results) == 0 {
		g.Log().Warningf(ctx, "[Rerank] 响应结果为空，降级")
		n := topN
		if n > len(docs) {
			n = len(docs)
		}
		fallback := make([]RerankResult, n)
		for i := range fallback {
			fallback[i] = RerankResult{Doc: docs[i], Score: 0}
		}
		return fallback
	}

	// 按 relevance_score 降序排列（API 通常已排好序，此处做保险排序）
	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})

	// 按排序后的 index 从原始文档列表中映射回 RerankResult
	result := make([]RerankResult, 0, topN)
	for _, res := range results {
		if res.Index >= 0 && res.Index < len(docs) {
			result = append(result, RerankResult{Doc: docs[res.Index], Score: res.RelevanceScore})
		}
		if len(result) >= topN {
			break
		}
	}

	g.Log().Debugf(ctx, "[Rerank] 重排完成 | 输入=%d | 输出=%d | tokens=%d | id=%s",
		len(docs), len(result), rerankResp.Usage.TotalTokens, rerankResp.ID)
	return result
}
