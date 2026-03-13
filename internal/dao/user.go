package dao

import "context"

// FindUserByUsername 按用户名查询用户，用于登录鉴权。
// 用户不存在时返回 gorm.ErrRecordNotFound。
func FindUserByUsername(ctx context.Context, username string) (*User, error) {
	db, err := DB(ctx)
	if err != nil {
		return nil, err
	}
	var user User
	if err = db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
