package dao

import (
	"time"

	"gorm.io/gorm"
)

// Event 安全事件
type Event struct {
	ID        string         `gorm:"column:id;primaryKey;size:64"`
	Title     string         `gorm:"column:title;size:256;not null"`
	Content   string         `gorm:"-"`                               // 完整内容仅存于 Milvus，不写入 MySQL
	EventType string         `gorm:"column:event_type;size:32;index"` // 事件来源大类：github、rss
	DedupKey  string         `gorm:"column:dedup_key;size:64;index"`  // SHA256 去重键（仅用于入库去重，不对外暴露）
	Severity  string         `gorm:"column:severity;size:32;index"`   // critical, high, medium, low
	Source    string         `gorm:"column:source;size:128;index"`
	Status    string         `gorm:"column:status;size:32;default:new"` // new, processing, resolved, ignored
	CVEID     string         `gorm:"column:cve_id;size:64;index"`       // CVE 编号，如 CVE-2024-12345
	RiskScore float64        `gorm:"column:risk_score"`                 // 风险评分 0-10，0 表示未评估
	Metadata  string         `gorm:"column:metadata;type:json"`
	IndexedAt *time.Time     `gorm:"column:indexed_at;type:datetime"`                // 向量索引完成时间，nil 表示未索引
	CreatedAt time.Time      `gorm:"column:created_at;type:datetime;autoCreateTime"` // 秒精度
	UpdatedAt time.Time      `gorm:"column:updated_at;type:datetime;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (Event) TableName() string { return "events" }

// Setting 系统配置 key-value 持久化
type Setting struct {
	Key       string    `gorm:"column:key;primaryKey;size:128"`
	Value     string    `gorm:"column:value;type:text"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;autoUpdateTime"`
}

func (Setting) TableName() string { return "settings" }

// Subscription 订阅源
type Subscription struct {
	ID          string         `gorm:"column:id;primaryKey;size:64"`
	Name        string         `gorm:"column:name;size:128;not null"`
	URL         string         `gorm:"column:url;size:512;not null"`
	Type        string         `gorm:"column:type;size:32;index"` // rss, github, webhook
	CronExpr    string         `gorm:"column:cron_expr;size:64"`  // 抓取间隔 cron 表达式
	Enabled     bool           `gorm:"column:enabled;default:true"`
	LastFetchAt *time.Time     `gorm:"column:last_fetch_at;type:datetime"`
	CreatedAt   time.Time      `gorm:"column:created_at;type:datetime;autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"column:updated_at;type:datetime;autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (Subscription) TableName() string { return "subscriptions" }

// Report 分析报告
type Report struct {
	ID        string         `gorm:"column:id;primaryKey;size:64"`
	Title     string         `gorm:"column:title;size:256;not null"`
	Content   string         `gorm:"column:content;type:longtext"`
	Type      string         `gorm:"column:type;size:32;index"` // weekly, monthly, custom
	CreatedAt time.Time      `gorm:"column:created_at;type:datetime;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:datetime;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (Report) TableName() string { return "reports" }

// FetchLog 订阅抓取日志，记录每次抓取的结果
type FetchLog struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement"`
	SubscriptionID string    `gorm:"column:subscription_id;size:64;index;not null"`
	Status         string    `gorm:"column:status;size:16;not null"` // success, failed
	FetchedCount   int       `gorm:"column:fetched_count;default:0"`
	NewCount       int       `gorm:"column:new_count;default:0"`
	DurationMs     int64     `gorm:"column:duration_ms;default:0"`
	ErrorMsg       string    `gorm:"column:error_msg;size:512"`
	CreatedAt      time.Time `gorm:"column:created_at;type:datetime;autoCreateTime;index"`
}

func (FetchLog) TableName() string { return "fetch_logs" }

// User 用户（阶段四 Auth）
type User struct {
	ID        string         `gorm:"column:id;primaryKey;size:64"`
	Username  string         `gorm:"column:username;size:64;uniqueIndex;not null"`
	Password  string         `gorm:"column:password;size:256;not null"`
	Role      string         `gorm:"column:role;size:32;default:user"` // admin, user
	CreatedAt time.Time      `gorm:"column:created_at;type:datetime;autoCreateTime"`
	UpdatedAt time.Time      `gorm:"column:updated_at;type:datetime;autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (User) TableName() string { return "users" }
