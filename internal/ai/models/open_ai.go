// Package models 封装项目中所有 LLM 的初始化逻辑。
//
// 核心设计：熔断器 + 优先级降级
//
// 架构分层：
//
//	config.yaml
//	  ds_think_chat_model.candidates  ← 多候选，按 priority 排序
//	  ds_quick_chat_model.candidates
//	  model_breaker.*                 ← 熔断参数（阈值、冷却时长）
//	        ↓
//	loadCandidates()                  ← 读取并排序候选列表
//	        ↓
//	buildFailoverModel()              ← 构建降级链
//	  ├─ breaker.Registry             ← 每个模型组独立一个注册表
//	  ├─ breakerModel × N             ← 每个候选包装熔断装饰器
//	  └─ failoverModel                ← 按序尝试，第一个成功即返回
//
// 调用流程：
//
//	请求到来
//	  → failoverModel.Generate/Stream
//	    → breakerModel[0].Generate/Stream
//	      → AllowCall("deepseek-v3-1-terminus) → true（CLOSED）
//	      → 调用成功 → MarkSuccess → 返回
//	    （若失败）→ MarkFailure → 累计失败次数 → 达阈值 → OPEN
//	    → breakerModel[1].Generate/Stream（自动降级到备用）
//	      → AllowCall("deepseek-v3-terminus") → true
//	      → 调用成功 → 返回
package models

import (
	"context"
	"fmt"
	"sort"

	"Fo-Sentinel-Agent/internal/ai/breaker"

	"github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// modelCandidate 单个候选模型的配置信息
type modelCandidate struct {
	ID       string // 唯一标识，用作熔断器 Registry 的 key
	APIKey   string
	BaseURL  string
	Model    string
	Priority int // 优先级，数值越小越优先；相同 priority 时按 ID 字典序
}

// loadCandidates 从配置文件读取指定模型组的候选列表，按 priority 升序排列。
// cfgKey 对应 config.yaml 中的顶层键（如 "ds_think_chat_model"）。
func loadCandidates(ctx context.Context, cfgKey string) ([]modelCandidate, error) {
	val, err := g.Cfg().Get(ctx, cfgKey+".candidates")
	if err != nil || val.IsNil() {
		return nil, fmt.Errorf("配置 %s.candidates 不存在", cfgKey)
	}
	var list []struct {
		ID       string `json:"id"`
		APIKey   string `json:"api_key"`
		BaseURL  string `json:"base_url"`
		Model    string `json:"model"`
		Priority int    `json:"priority"`
	}
	if err := val.Scan(&list); err != nil {
		return nil, fmt.Errorf("解析 %s.candidates 失败: %w", cfgKey, err)
	}
	candidates := make([]modelCandidate, len(list))
	for i, c := range list {
		candidates[i] = modelCandidate{
			ID:       c.ID,
			APIKey:   c.APIKey,
			BaseURL:  c.BaseURL,
			Model:    c.Model,
			Priority: c.Priority,
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Priority < candidates[j].Priority
	})
	return candidates, nil
}

// buildFailoverModel 构建带熔断保护的降级链模型实例。
//
// 构建过程：
//  1. 读取候选列表（已按 priority 排序）
//  2. 从 config.yaml model_breaker.* 读取熔断参数，创建该模型组专属的 Registry
//  3. 跳过当前处于 OPEN 状态的候选（AllowCall=false）
//  4. 初始化每个可用候选的 openai.ChatModel，初始化失败则直接 MarkFailure
//  5. 用 breakerModel 包装每个候选，组成 failoverModel 降级链
//
// 注意：每个模型组（think/quick）持有独立的 Registry，熔断状态互不影响。
func buildFailoverModel(ctx context.Context, cfgKey string) (einomodel.ToolCallingChatModel, error) {
	candidates, err := loadCandidates(ctx, cfgKey)
	if err != nil {
		return nil, err
	}

	// 从配置读取熔断参数，创建该模型组专属注册表
	threshold, _ := g.Cfg().Get(ctx, "model_breaker.failure_threshold")
	openDurSec, _ := g.Cfg().Get(ctx, "model_breaker.open_duration_sec")
	reg := breaker.New(breaker.Config{
		FailureThreshold: threshold.Int(),
		OpenDuration:     openDurSec.Duration() * 1e9, // sec → time.Duration
	})

	type candidate struct {
		id string
		m  einomodel.ToolCallingChatModel
	}
	var available []candidate
	for _, c := range candidates {
		// 跳过熔断中的候选，避免在初始化阶段浪费连接
		if !reg.AllowCall(c.ID) {
			continue
		}
		m, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
			Model:   c.Model,
			APIKey:  c.APIKey,
			BaseURL: c.BaseURL,
		})
		if err != nil {
			// 初始化失败视为一次失败，累计计数
			reg.MarkFailure(c.ID)
			continue
		}
		available = append(available, candidate{id: c.ID, m: m})
	}
	if len(available) == 0 {
		return nil, fmt.Errorf("模型组 %s 所有候选均不可用", cfgKey)
	}

	// 用熔断装饰器包装每个候选
	wrapped := make([]einomodel.ToolCallingChatModel, len(available))
	for i, c := range available {
		wrapped[i] = &breakerModel{id: c.id, inner: c.m, reg: reg}
	}
	// 只有一个候选时无需降级链，直接返回
	if len(wrapped) == 1 {
		return wrapped[0], nil
	}
	return &failoverModel{candidates: wrapped}, nil
}

// failoverModel 多候选降级链：按序尝试每个候选，第一个成功的结果直接返回。
// 某个候选熔断（breakerModel 返回熔断错误）时自动跳过，切换到下一个。
// 所有候选均失败时返回最后一个错误。
type failoverModel struct {
	candidates []einomodel.ToolCallingChatModel
}

func (f *failoverModel) Generate(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.Message, error) {
	var lastErr error
	for _, m := range f.candidates {
		resp, err := m.Generate(ctx, input, opts...)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("所有候选模型均失败: %w", lastErr)
}

func (f *failoverModel) Stream(ctx context.Context, input []*schema.Message, opts ...einomodel.Option) (*schema.StreamReader[*schema.Message], error) {
	var lastErr error
	for _, m := range f.candidates {
		stream, err := m.Stream(ctx, input, opts...)
		if err == nil {
			return stream, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("所有候选模型均失败: %w", lastErr)
}

// WithTools 对所有候选绑定工具，返回新的降级链实例。
func (f *failoverModel) WithTools(tools []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	bound := make([]einomodel.ToolCallingChatModel, 0, len(f.candidates))
	for _, m := range f.candidates {
		bm, err := m.WithTools(tools)
		if err != nil {
			return nil, err
		}
		bound = append(bound, bm)
	}
	return &failoverModel{candidates: bound}, nil
}

// OpenAIForDeepSeekV31Think 初始化深度思考版 LLM（带熔断+降级链）。
// 对应配置：ds_think_chat_model.candidates
func OpenAIForDeepSeekV31Think(ctx context.Context) (einomodel.ToolCallingChatModel, error) {
	return buildFailoverModel(ctx, "ds_think_chat_model")
}

// OpenAIForDeepSeekV3Quick 初始化快速响应版 LLM（带熔断+降级链）。
// 对应配置：ds_quick_chat_model.candidates
func OpenAIForDeepSeekV3Quick(ctx context.Context) (einomodel.ToolCallingChatModel, error) {
	return buildFailoverModel(ctx, "ds_quick_chat_model")
}
