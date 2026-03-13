package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

// LoginReq 登录请求
type LoginReq struct {
	g.Meta   `path:"/auth/v1/login" method:"post" summary:"登录"`
	Username string `json:"username" v:"required"`
	Password string `json:"password" v:"required"`
}

// LoginRes 登录响应
type LoginRes struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}
