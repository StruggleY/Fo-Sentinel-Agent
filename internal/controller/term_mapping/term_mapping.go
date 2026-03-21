// Package term_mapping 提供术语规则管理 HTTP 控制器。
package term_mapping

import (
	"context"

	v1 "Fo-Sentinel-Agent/api/term_mapping/v1"
	"Fo-Sentinel-Agent/internal/ai/rule"
	dao "Fo-Sentinel-Agent/internal/dao/mysql"

	"github.com/gogf/gf/v2/frame/g"
)

// ControllerV1 术语规则管理控制器
type ControllerV1 struct{}

// NewV1 返回控制器实例
func NewV1() *ControllerV1 { return &ControllerV1{} }

// List 获取规则列表
func (c *ControllerV1) List(ctx context.Context, _ *v1.ListReq) (*v1.ListRes, error) {
	records, err := dao.ListTermMappings(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[TermMapping] 查询规则列表失败: %v", err)
		return &v1.ListRes{Items: []v1.TermMappingItem{}, Total: 0}, nil
	}
	items := make([]v1.TermMappingItem, 0, len(records))
	for _, r := range records {
		items = append(items, v1.TermMappingItem{
			ID:         r.ID,
			SourceTerm: r.SourceTerm,
			TargetTerm: r.TargetTerm,
			Priority:   r.Priority,
			Enabled:    r.Enabled,
			CreatedAt:  r.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:  r.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return &v1.ListRes{Items: items, Total: len(items)}, nil
}

// Create 创建规则，写入后自动热重载
func (c *ControllerV1) Create(ctx context.Context, req *v1.CreateReq) (*v1.CreateRes, error) {
	m := &dao.QueryTermMapping{
		SourceTerm: req.SourceTerm,
		TargetTerm: req.TargetTerm,
		Priority:   req.Priority,
		Enabled:    req.Enabled,
	}
	if err := dao.CreateTermMapping(ctx, m); err != nil {
		return nil, err
	}
	rule.ReloadTermMappings(ctx)
	return &v1.CreateRes{ID: m.ID}, nil
}

// Update 更新规则，写入后自动热重载
func (c *ControllerV1) Update(ctx context.Context, req *v1.UpdateReq) (*v1.UpdateRes, error) {
	updates := map[string]any{
		"target_term": req.TargetTerm,
		"priority":    req.Priority,
		"enabled":     req.Enabled,
	}
	if err := dao.UpdateTermMapping(ctx, req.ID, updates); err != nil {
		return nil, err
	}
	rule.ReloadTermMappings(ctx)
	return &v1.UpdateRes{}, nil
}

// Delete 删除规则，写入后自动热重载
func (c *ControllerV1) Delete(ctx context.Context, req *v1.DeleteReq) (*v1.DeleteRes, error) {
	if err := dao.DeleteTermMapping(ctx, req.ID); err != nil {
		return nil, err
	}
	rule.ReloadTermMappings(ctx)
	return &v1.DeleteRes{}, nil
}

// Reload 手动触发进程内规则缓存热重载
func (c *ControllerV1) Reload(ctx context.Context, _ *v1.ReloadReq) (*v1.ReloadRes, error) {
	n := rule.ReloadTermMappings(ctx)
	return &v1.ReloadRes{Count: n}, nil
}
