package dao

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// SeedAdmin 若用户表为空，创建默认 admin 用户。密码从 auth.seed.admin_password 读取，缺省为 admin123
func SeedAdmin(ctx context.Context) {
	db, err := DB(ctx)
	if err != nil {
		return
	}
	var count int64
	if db.Model(&User{}).Count(&count).Error != nil || count > 0 {
		return
	}
	pass, _ := g.Cfg().Get(ctx, "auth.seed.admin_password")
	password := "admin123"
	if pass != nil && pass.String() != "" {
		password = pass.String()
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	db.Create(&User{
		ID:       uuid.New().String(),
		Username: "admin",
		Password: string(hash),
		Role:     "admin",
	})
}

// SeedSettings 初始化默认设置项，仅在 key 不存在（值为空）时写入，不覆盖用户已修改的值。
func SeedSettings(ctx context.Context) {
	defaults := []struct{ key, value string }{
		{"general.site_name", "安全事件智能研判多智能体协同平台"},
		{"general.auto_mark_read", "true"},
	}
	for _, d := range defaults {
		if existing, _ := GetSetting(ctx, d.key); existing == "" {
			_ = SetSetting(ctx, d.key, d.value)
		}
	}
}
