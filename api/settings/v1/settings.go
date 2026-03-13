package v1

import "github.com/gogf/gf/v2/frame/g"

// GetGeneralReq 获取通用设置
type GetGeneralReq struct {
	g.Meta `path:"/settings/v1/general" method:"get" summary:"获取通用设置"`
}

// GeneralSettings 通用设置内容
type GeneralSettings struct {
	SiteName     string `json:"site_name"`
	AutoMarkRead bool   `json:"auto_mark_read"`
}

// GetGeneralRes 响应
type GetGeneralRes struct {
	Settings GeneralSettings `json:"settings"`
}

// SaveGeneralReq 保存通用设置
type SaveGeneralReq struct {
	g.Meta       `path:"/settings/v1/general" method:"post" summary:"保存通用设置"`
	SiteName     string `json:"site_name"`
	AutoMarkRead bool   `json:"auto_mark_read"`
}

// SaveGeneralRes 保存响应
type SaveGeneralRes struct{}
