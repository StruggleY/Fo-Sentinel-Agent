package dao

import "context"

// CreateFetchLog 记录一次抓取结果
func CreateFetchLog(ctx context.Context, log *FetchLog) error {
	db, err := DB(ctx)
	if err != nil {
		return err
	}
	return db.Create(log).Error
}

// ListFetchLogs 按订阅ID查询抓取日志，按时间倒序分页
func ListFetchLogs(ctx context.Context, subscriptionID string, limit, offset int) ([]FetchLog, int64, error) {
	db, err := DB(ctx)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	q := db.Model(&FetchLog{}).Where("subscription_id = ?", subscriptionID)
	if err = q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []FetchLog
	if err = q.Limit(limit).Offset(offset).Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}
