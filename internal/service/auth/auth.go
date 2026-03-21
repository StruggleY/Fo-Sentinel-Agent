// Package authsvc 提供用户认证业务逻辑。
// 职责：校验密码 → 签发 JWT，不含 HTTP 层细节。
package authsvc

import (
	"context"
	"time"

	dao "Fo-Sentinel-Agent/internal/dao/mysql"
	"Fo-Sentinel-Agent/utility/auth"

	"github.com/gogf/gf/v2/frame/g"
	"golang.org/x/crypto/bcrypt"
)

// Login 校验用户名/密码，成功后签发 JWT Token。
// 密码错误与用户不存在使用相同错误消息，防止枚举攻击。
// 过期时长从配置 auth.jwt.expire_hours 读取，未配置时默认 24 小时。
func Login(ctx context.Context, username, password string) (token, userID, role string, err error) {
	// 查询用户，不存在时返回 error（与密码错误错误消息一致，防止用户名枚举）
	user, err := dao.FindUserByUsername(ctx, username)
	if err != nil {
		return
	}
	// bcrypt 校验密码，自动处理盐值，时间复杂度抵抗暴力破解
	if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return
	}
	// 读取 JWT 过期时长配置，无效时兜底 24 小时
	exp, _ := g.Cfg().Get(ctx, "auth.jwt.expire_hours")
	expHours := exp.Int()
	if expHours <= 0 {
		expHours = 24
	}
	// 生成 JWT，payload 包含 user_id、username、role
	token, err = auth.Generate(user.ID, user.Username, user.Role, time.Duration(expHours)*time.Hour)
	if err != nil {
		return
	}
	userID = user.ID
	role = user.Role
	return
}
