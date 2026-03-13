// Package settings 提供系统通用设置 HTTP 控制器。
package settings

import (
	"context"

	v1 "Fo-Sentinel-Agent/api/settings/v1"
	"Fo-Sentinel-Agent/internal/dao"
)

type ControllerV1 struct{}

func NewV1() *ControllerV1 { return &ControllerV1{} }

var generalKeys = []string{
	"general.site_name",
	"general.auto_mark_read",
}

// GetGeneral 读取通用设置，DB 无值时返回默认值
func (c *ControllerV1) GetGeneral(ctx context.Context, _ *v1.GetGeneralReq) (*v1.GetGeneralRes, error) {
	cfg, _ := dao.GetSettings(ctx, generalKeys)

	siteName := cfg["general.site_name"]
	if siteName == "" {
		siteName = "安全事件智能研判多智能体协同平台"
	}
	autoMarkRead := cfg["general.auto_mark_read"] != "false" // 默认 true

	return &v1.GetGeneralRes{
		Settings: v1.GeneralSettings{
			SiteName:     siteName,
			AutoMarkRead: autoMarkRead,
		},
	}, nil
}

// SaveGeneral 持久化通用设置到 DB
func (c *ControllerV1) SaveGeneral(ctx context.Context, req *v1.SaveGeneralReq) (*v1.SaveGeneralRes, error) {
	autoMarkRead := "true"
	if !req.AutoMarkRead {
		autoMarkRead = "false"
	}
	_ = dao.SetSetting(ctx, "general.site_name", req.SiteName)
	_ = dao.SetSetting(ctx, "general.auto_mark_read", autoMarkRead)
	return &v1.SaveGeneralRes{}, nil
}
