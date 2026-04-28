// Package models 中的 breakerModel 是 ToolCallingChatModel 的熔断器装饰器。
//
// 设计思路：
// 采用装饰器模式包装底层模型实例，对调用方完全透明——调用方持有的仍是
// ToolCallingChatModel 接口，无需感知熔断逻辑。
// 每次 Generate/Stream 调用前查询熔断状态，失败后上报给 Registry，
// 由 Registry 决定是否触发熔断或状态转换。
//
// 与 failoverModel 的协作：
// breakerModel 负责单个候选的熔断保护；
// failoverModel 负责多候选的降级切换。
// 两者组合实现"单模型熔断 + 跨模型降级"的完整容错链路。
package models

import (
	"context"
	"fmt"

	"Fo-Sentinel-Agent/internal/ai/breaker"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// breakerModel 用熔断器装饰单个 ToolCallingChatModel。
// id 与 Registry 中的 key 对应，用于定位该模型的熔断状态。
type breakerModel struct {
	id    string
	inner model.ToolCallingChatModel
	reg   *breaker.Registry
}

// Generate 非流式调用：调用前检查熔断，失败后上报 MarkFailure，成功后上报 MarkSuccess。
func (m *breakerModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if !m.reg.AllowCall(m.id) {
		return nil, fmt.Errorf("模型 %s 熔断中，请稍后重试", m.id)
	}
	resp, err := m.inner.Generate(ctx, input, opts...)
	if err != nil {
		m.reg.MarkFailure(m.id)
		return nil, err
	}
	m.reg.MarkSuccess(m.id)
	return resp, nil
}

// Stream 流式调用：连接建立失败上报 MarkFailure；连接成功上报 MarkSuccess。
// 注意：流读取阶段（Recv）的错误发生在 Stream 返回之后，无法在此自动感知，
// 需要调用方在读流出错时主动调用 reg.MarkFailure(id) 上报。
func (m *breakerModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if !m.reg.AllowCall(m.id) {
		return nil, fmt.Errorf("模型 %s 熔断中，请稍后重试", m.id)
	}
	stream, err := m.inner.Stream(ctx, input, opts...)
	if err != nil {
		m.reg.MarkFailure(m.id)
		return nil, err
	}
	m.reg.MarkSuccess(m.id)
	return stream, nil
}

// WithTools 返回绑定工具后的新实例，保持熔断器绑定不变。
func (m *breakerModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	newInner, err := m.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &breakerModel{id: m.id, inner: newInner, reg: m.reg}, nil
}
