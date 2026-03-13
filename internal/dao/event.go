package dao

import (
	"context"
	"strings"
	"time"
)

// ListEvents 查询事件列表，支持 severity/status/keyword 过滤、排序和分页。
// orderBy 可选值：severity, status, source, created_at（默认 created_at DESC）。
func ListEvents(ctx context.Context, limit, offset int, severity, status, keyword, orderBy, orderDir string) ([]Event, int64, error) {
	db, err := DB(ctx)
	if err != nil {
		return nil, 0, err
	}
	q := db.Model(&Event{})
	if severity != "" {
		q = q.Where("severity = ?", severity)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if keyword != "" {
		q = q.Where("title LIKE ? OR cve_id LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	var total int64
	if err = q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	// 构建 ORDER BY 子句，默认按创建时间倒序
	order := "created_at DESC"
	if orderBy != "" {
		dir := "DESC"
		if strings.EqualFold(orderDir, "asc") {
			dir = "ASC"
		}
		switch orderBy {
		case "severity":
			// 严重程度自定义排序：critical > high > medium > low > info
			order = "CASE severity WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 WHEN 'low' THEN 4 ELSE 5 END " + dir
		case "status":
			order = "status " + dir
		case "source":
			order = "source " + dir
		case "created_at":
			order = "created_at " + dir
		}
	}
	var events []Event
	if err = q.Limit(limit).Offset(offset).Order(order).Find(&events).Error; err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

// UpdateEventStatus 更新单条事件状态。
func UpdateEventStatus(ctx context.Context, id, status string) error {
	db, err := DB(ctx)
	if err != nil {
		return err
	}
	return db.Model(&Event{}).Where("id = ?", id).Update("status", status).Error
}

// BatchUpdateEventStatus 批量更新事件状态。
func BatchUpdateEventStatus(ctx context.Context, ids []string, status string) error {
	db, err := DB(ctx)
	if err != nil {
		return err
	}
	return db.Model(&Event{}).Where("id IN ?", ids).Update("status", status).Error
}

// CreateEvent 将事件持久化到数据库。
func CreateEvent(ctx context.Context, e *Event) error {
	db, err := DB(ctx)
	if err != nil {
		return err
	}
	return db.Create(e).Error
}

// DeleteEvent 软删除安全事件（设置 deleted_at 字段）。
func DeleteEvent(ctx context.Context, id string) error {
	db, err := DB(ctx)
	if err != nil {
		return err
	}
	return db.Where("id = ?", id).Delete(&Event{}).Error
}

// ListIndexedEventIDsBySource 查询来自指定订阅源的所有已索引事件 ID。
// 仅返回 indexed_at IS NOT NULL 的事件，用于删除订阅时同步清理 Milvus 向量。
func ListIndexedEventIDsBySource(ctx context.Context, source string) ([]string, error) {
	db, err := DB(ctx)
	if err != nil {
		return nil, err
	}
	var ids []string
	err = db.Model(&Event{}).
		Where("source = ? AND indexed_at IS NOT NULL", source).
		Pluck("id", &ids).Error
	return ids, err
}

// DeleteEventsBySource 软删除来自指定订阅源的所有事件。
// 删除订阅时调用，清理该源产生的孤儿事件。
func DeleteEventsBySource(ctx context.Context, source string) error {
	db, err := DB(ctx)
	if err != nil {
		return err
	}
	return db.Where("source = ?", source).Delete(&Event{}).Error
}

// EventTrendItem 单日事件统计。
type EventTrendItem struct {
	Date     string
	Total    int64
	Critical int64
	High     int64
	Medium   int64
	Low      int64
}

// GetEventTrend 查询最近 days 天内按日期分组的事件计数，返回倒序（最新日期在前）。
// 使用子查询聚合各严重程度，避免多次全表扫描。
func GetEventTrend(ctx context.Context, days int) ([]EventTrendItem, error) {
	db, err := DB(ctx)
	if err != nil {
		return nil, err
	}
	if days <= 0 {
		days = 30
	}

	type row struct {
		Date     string
		Severity string
		Count    int64
	}
	var rows []row
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	if err = db.Model(&Event{}).
		Select("DATE_FORMAT(created_at, '%Y-%m-%d') AS date, severity, COUNT(*) AS count").
		Where("DATE(created_at) >= ?", since).
		Group("DATE_FORMAT(created_at, '%Y-%m-%d'), severity").
		Order("DATE_FORMAT(created_at, '%Y-%m-%d') DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	// 聚合到按日期的 map
	dayMap := make(map[string]*EventTrendItem)
	for _, r := range rows {
		item, ok := dayMap[r.Date]
		if !ok {
			item = &EventTrendItem{Date: r.Date}
			dayMap[r.Date] = item
		}
		item.Total += r.Count
		switch r.Severity {
		case "critical":
			item.Critical += r.Count
		case "high":
			item.High += r.Count
		case "medium":
			item.Medium += r.Count
		case "low":
			item.Low += r.Count
		}
	}

	// 按日期倒序收集
	items := make([]EventTrendItem, 0, len(dayMap))
	for _, v := range dayMap {
		items = append(items, *v)
	}
	// 排序：日期倒序
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].Date < items[j].Date {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	return items, nil
}

type EventStats struct {
	Total         int64            `json:"total"`
	TodayCount    int64            `json:"today_count"`
	CriticalCount int64            `json:"critical_count"`
	BySeverity    map[string]int64 `json:"by_severity"`
}

// GetEventStats 查询事件统计：总数、今日新增、高危数量、按严重程度分组。
func GetEventStats(ctx context.Context) (*EventStats, error) {
	db, err := DB(ctx)
	if err != nil {
		return nil, err
	}
	stats := &EventStats{BySeverity: make(map[string]int64)}

	// 总事件数
	if err = db.Model(&Event{}).Count(&stats.Total).Error; err != nil {
		return nil, err
	}

	// 今日新增：使用 DATE(created_at) = 今日日期字符串，与 loc=Local 驱动存储格式完全一致
	// 避免跨时区时间范围计算带来的边界误差
	todayStr := time.Now().Format("2006-01-02")
	if err = db.Model(&Event{}).Where("DATE(created_at) = ?", todayStr).Count(&stats.TodayCount).Error; err != nil {
		return nil, err
	}

	// 高危事件数（critical + high）
	if err = db.Model(&Event{}).Where("severity IN ?", []string{"critical", "high"}).Count(&stats.CriticalCount).Error; err != nil {
		return nil, err
	}

	// 按严重程度分组统计
	type row struct {
		Severity string
		Count    int64
	}
	var rows []row
	if err = db.Model(&Event{}).Select("severity, COUNT(*) as count").Group("severity").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		stats.BySeverity[r.Severity] = r.Count
	}

	return stats, nil
}
