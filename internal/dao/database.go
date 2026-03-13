package dao

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var (
	globalDB *gorm.DB
	dbOnce   sync.Once
	initErr  error
)

// Init 初始化 MySQL 连接并执行 GORM 迁移，main 启动时调用；dsn 未配置时跳过
func Init(ctx context.Context) error {
	dbOnce.Do(func() {
		dsn, err := g.Cfg().Get(ctx, "database.mysql.dsn")
		if err != nil || dsn.String() == "" {
			return // 未配置则跳过，不阻塞启动
		}
		rawDSN := dsn.String()
		// 若 DSN 未指定 loc，追加 loc=Local 确保 Go 与 MySQL 时区一致（autoCreateTime 使用本地时间）
		if !strings.Contains(rawDSN, "loc=") {
			if strings.Contains(rawDSN, "?") {
				rawDSN += "&loc=Local"
			} else {
				rawDSN += "?loc=Local"
			}
		}
		db, err := gorm.Open(mysql.Open(rawDSN), &gorm.Config{})
		if err != nil {
			initErr = fmt.Errorf("open mysql: %w", err)
			return
		}
		if err = db.WithContext(ctx).AutoMigrate(&Event{}, &Subscription{}, &Report{}, &FetchLog{}, &User{}, &Setting{}); err != nil {
			initErr = fmt.Errorf("auto migrate: %w", err)
			return
		}
		// 配置连接池：调度器场景下多个订阅 goroutine 并发读写，需合理控制连接数
		// MaxOpenConns：限制最大并发连接数，防止打爆 MySQL server
		// MaxIdleConns：保持空闲连接复用，避免频繁握手
		// ConnMaxLifetime：定期轮换连接，防止 MySQL server 端因超时强制断开
		if sqlDB, e := db.DB(); e == nil {
			sqlDB.SetMaxOpenConns(20)
			sqlDB.SetMaxIdleConns(5)
			sqlDB.SetConnMaxLifetime(time.Hour)
		}
		globalDB = db
	})
	return initErr
}

// DB 返回全局 DB 实例，Init 成功后使用
func DB(ctx context.Context) (*gorm.DB, error) {
	if initErr != nil {
		return nil, initErr
	}
	return globalDB.WithContext(ctx), nil
}
