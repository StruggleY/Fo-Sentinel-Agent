// Package retriever rerank.go：DashScope 文档重排序客户端
//
// 背景：
//   向量检索基于语义嵌入的余弦相似度，擅长「模糊语义匹配」，但不擅长精确的「相关性排序」。
//   多路并行检索（MultiRetrieve）扩大了候选池，但候选文档数量增多后，
//   哪几篇最终传给 LLM 就变得更重要——传错了会导致 LLM 生成错误结论。
//
//   Rerank 模型（如 gte-rerank）专为「查询-文档相关性」任务训练，
//   对「给定查询，哪篇文档最相关」的判断精准度远高于向量相似度。
//   通过 Rerank 从候选池中精选 TopN 文档，实现「召回率+精准度」双提升。
//
// 架构位置：
//   MultiRetrieve → [候选文档池，约 10 个] → Rerank → [精选文档，3 个] → LLM Prompt
//
// 默认禁用：
//   Rerank 需要额外的 API 调用（约 200ms），对于单一问题收益有限。
//   在 config.yaml 中设置 retriever.rerank.enabled=true 后才会生效。
//   禁用时 GetRerankClient 返回 (nil, nil)，调用方跳过重排步骤。
package retriever

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

var (
	// globalRerankClient 全局单例，确保只创建一个 HTTP 连接池复用
	globalRerankClient *RerankClient
	// rerankOnce 保证 GetRerankClient 的初始化逻辑只执行一次（线程安全）
	rerankOnce    sync.Once
	rerankInitErr error
)

// RerankClient DashScope 文档重排序客户端（单例）。
//
// 通过 DashScope compatible-mode API 对文档列表进行语义相关性重排序。
// compatible-mode 端点与 OpenAI 风格兼容，认证方式相同（Bearer Token）。
//
// API 端点：POST {baseURL}/rerank
// 认证：Authorization: Bearer {apiKey}
// 参考：https://help.aliyun.com/zh/model-studio/developer-reference/rerank-api
type RerankClient struct {
	apiKey  string // DashScope API Key（Bearer Token 认证）
	baseURL string // API Base URL，默认 https://dashscope.aliyuncs.com/compatible-mode/v1
	model   string // Rerank 模型名称，默认 gte-rerank
}

// GetRerankClient 返回全局单例 RerankClient。
//
// 初始化行为：
//   - retriever.rerank.enabled=false（或未配置）：返回 (nil, nil)，表示 Rerank 未启用
//   - enabled=true 但 api_key 为空：返回 (nil, error)，告知配置错误
//   - enabled=true 且配置完整：初始化并返回单例客户端
//
// 调用方处理模式：
//
//	if rc, _ := retriever.GetRerankClient(ctx); rc != nil && len(docs) > 0 {
//	    docs = rc.Rerank(ctx, query, docs, topN)
//	}
func GetRerankClient(ctx context.Context) (*RerankClient, error) {
	rerankOnce.Do(func() {
		enabled, _ := g.Cfg().Get(ctx, "retriever.rerank.enabled")
		if !enabled.Bool() {
			// 未启用，globalRerankClient 保持 nil，调用方通过 nil 判断跳过重排
			return
		}

		apiKey, _ := g.Cfg().Get(ctx, "retriever.rerank.api_key")
		modelName, _ := g.Cfg().Get(ctx, "retriever.rerank.model")
		baseURL, _ := g.Cfg().Get(ctx, "retriever.rerank.base_url")

		ak := apiKey.String()
		if ak == "" {
			rerankInitErr = fmt.Errorf("[Rerank] api_key 未配置，rerank 不可用")
			return
		}

		// 使用配置值，缺省时回落到推荐默认值
		mn := modelName.String()
		if mn == "" {
			mn = "gte-rerank" // DashScope 通用重排序模型
		}
		bu := baseURL.String()
		if bu == "" {
			bu = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		}

		globalRerankClient = &RerankClient{apiKey: ak, baseURL: bu, model: mn}
		g.Log().Infof(ctx, "[Rerank] 客户端初始化成功 | model=%s", mn)
	})
	return globalRerankClient, rerankInitErr
}

// rerankRequest DashScope Rerank API 请求体结构。
//
// ReturnDocs=false：服务端只返回文档索引和分数，不返回原文（节省带宽，文档原文由客户端持有）。
type rerankRequest struct {
	Model      string   `json:"model"`            // Rerank 模型名称
	Query      string   `json:"query"`            // 原始用户查询（作为相关性判断的基准）
	Documents  []string `json:"documents"`        // 待排序的文档文本列表
	TopN       int      `json:"top_n"`            // 返回前 N 个最相关文档
	ReturnDocs bool     `json:"return_documents"` // 是否返回文档原文（false 可节省带宽）
}

// rerankResponse DashScope Rerank API 响应体结构。
// results 列表中每个元素包含文档在原始列表中的下标（index）和相关性分数（relevance_score）。
type rerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`           // 文档在原始 documents 列表中的下标
		RelevanceScore float64 `json:"relevance_score"` // 相关性分数，值越高越相关（无固定范围）
	} `json:"results"`
}

// Rerank 调用 DashScope Rerank API 对文档列表按相关性排序，返回 topN 个最相关文档。
//
// 调用流程：
//  1. 提取所有文档的文本内容，构建请求体
//  2. POST 到 DashScope Rerank API（Bearer Token 认证）
//  3. 解析响应，按 relevance_score 降序排列
//  4. 按 index 映射回原始 []*schema.Document 列表
//
// 降级策略（API 调用任意环节失败时）：
//   返回原始文档列表前 topN 个（相当于「不重排」），保证主流程不中断。
//   这是合理的降级：即使不重排，向量检索的结果也有基本的相关性保障。
//
// 边界处理：
//   - docs 为空：直接返回，不调用 API
//   - topN ≤ 0 或 topN ≥ len(docs)：直接返回原始列表（无需重排）
func (r *RerankClient) Rerank(ctx context.Context, query string, docs []*schema.Document, topN int) []*schema.Document {
	if len(docs) == 0 {
		return docs
	}
	if topN <= 0 || topN >= len(docs) {
		// 无需重排：docs 数量已在 topN 范围内，直接返回
		return docs
	}

	// 提取文档文本（API 只需文本内容，不需要 ID 和元数据）
	texts := make([]string, len(docs))
	for i, d := range docs {
		texts[i] = d.Content
	}

	reqBody := rerankRequest{
		Model:      r.model,
		Query:      query,
		Documents:  texts,
		TopN:       topN,
		ReturnDocs: false, // 文档原文已在本地，无需服务端返回
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		g.Log().Warningf(ctx, "[Rerank] 序列化失败，降级返回原始列表: %v", err)
		return docs[:topN]
	}

	// 构建 HTTP 请求，携带 Context 以支持超时取消
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		r.baseURL+"/rerank", bytes.NewReader(body))
	if err != nil {
		g.Log().Warningf(ctx, "[Rerank] 构建请求失败，降级: %v", err)
		return docs[:topN]
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		g.Log().Warningf(ctx, "[Rerank] 请求失败，降级: %v", err)
		return docs[:topN]
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		g.Log().Warningf(ctx, "[Rerank] 响应异常 status=%d，降级: %v", resp.StatusCode, err)
		return docs[:topN]
	}

	var rerankResp rerankResponse
	if err := json.Unmarshal(data, &rerankResp); err != nil {
		g.Log().Warningf(ctx, "[Rerank] 解析响应失败，降级: %v", err)
		return docs[:topN]
	}

	// 按 relevance_score 降序排列，确保最相关的文档排在最前
	sort.Slice(rerankResp.Results, func(i, j int) bool {
		return rerankResp.Results[i].RelevanceScore > rerankResp.Results[j].RelevanceScore
	})

	// 按排序后的 index 从原始文档列表中取出对应文档
	result := make([]*schema.Document, 0, topN)
	for _, res := range rerankResp.Results {
		if res.Index >= 0 && res.Index < len(docs) {
			result = append(result, docs[res.Index])
		}
		if len(result) >= topN {
			break
		}
	}

	g.Log().Debugf(ctx, "[Rerank] 重排完成 | 输入=%d | 输出=%d", len(docs), len(result))
	return result
}
