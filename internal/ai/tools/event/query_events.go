package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"Fo-Sentinel-Agent/internal/dao"
	"Fo-Sentinel-Agent/utility/client"
	"Fo-Sentinel-Agent/utility/common"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// QueryEventsInput 查询事件参数
type QueryEventsInput struct {
	Severity string `json:"severity,omitempty" jsonschema:"description=按严重程度过滤: critical, high, medium, low，空则不过滤"`
	Status   string `json:"status,omitempty"   jsonschema:"description=按处理状态过滤: new, processing, resolved, ignored，空则不过滤"`
	CveID    string `json:"cve_id,omitempty"   jsonschema:"description=按 CVE 编号精确过滤，如 CVE-2024-1234，空则不过滤"`
	Keyword  string `json:"keyword,omitempty"  jsonschema:"description=按标题关键词模糊搜索，空则不过滤"`
	TimeFrom string `json:"time_from,omitempty" jsonschema:"description=开始时间，格式 2006-01-02，空则不限"`
	TimeTo   string `json:"time_to,omitempty"   jsonschema:"description=结束时间，格式 2006-01-02，空则不限"`
	Limit    int    `json:"limit,omitempty"    jsonschema:"description=返回条数，默认 10，最大 50"`
}

// EventResult 工具返回的事件信息（含 Milvus 中的完整内容）
type EventResult struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Content   string  `json:"content"`    // 从 Milvus 获取的完整事件内容
	EventType string  `json:"event_type"` // 事件来源大类：github、rss
	Severity  string  `json:"severity"`
	Status    string  `json:"status"`
	CveID     string  `json:"cve_id"`
	RiskScore float64 `json:"risk_score"`
	Source    string  `json:"source"`
	Metadata  string  `json:"metadata"`
	CreatedAt string  `json:"created_at"`
	IndexedAt string  `json:"indexed_at"` // 是否已向量化
}

// NewQueryEventsTool 创建 query_events 工具，从 MySQL 查询事件元数据并从 Milvus 补充完整内容
func NewQueryEventsTool() tool.InvokableTool {
	t, err := utils.InferOptionableTool(
		"query_events",
		"Query security events from the database. Returns event metadata (severity, risk_score, cve_id, status) from MySQL and full content from Milvus vector store. Use this when analyzing recent events, checking severity distribution, or investigating specific CVEs.",
		func(ctx context.Context, input *QueryEventsInput, opts ...tool.Option) (output string, err error) {
			db, err := dao.DB(ctx)
			if err != nil {
				return "", fmt.Errorf("database not available: %w", err)
			}

			// 参数校验
			limit := input.Limit
			if limit <= 0 {
				limit = 10
			}
			if limit > 50 {
				limit = 50
			}

			// 构建查询条件
			q := db.Limit(limit).Order("created_at DESC")
			if input.Severity != "" {
				q = q.Where("severity = ?", input.Severity)
			}
			if input.Status != "" {
				q = q.Where("status = ?", input.Status)
			}
			if input.CveID != "" {
				q = q.Where("cve_id = ?", input.CveID)
			}
			if input.Keyword != "" {
				q = q.Where("title LIKE ?", "%"+input.Keyword+"%")
			}
			if input.TimeFrom != "" {
				if t, e := time.Parse("2006-01-02", input.TimeFrom); e == nil {
					q = q.Where("created_at >= ?", t)
				}
			}
			if input.TimeTo != "" {
				if t, e := time.Parse("2006-01-02", input.TimeTo); e == nil {
					q = q.Where("created_at < ?", t.AddDate(0, 0, 1))
				}
			}

			var events []dao.Event
			if err = q.Find(&events).Error; err != nil {
				return "", fmt.Errorf("query events: %w", err)
			}
			if len(events) == 0 {
				return "[]", nil
			}

			// 收集已向量化事件的 ID，批量从 Milvus 获取完整内容
			var indexedIDs []string
			for _, e := range events {
				if e.IndexedAt != nil {
					indexedIDs = append(indexedIDs, e.ID)
				}
			}
			contentMap := fetchMilvusContentByIDs(ctx, indexedIDs)

			// 组装结果
			results := make([]EventResult, 0, len(events))
			for _, e := range events {
				r := EventResult{
					ID:        e.ID,
					Title:     e.Title,
					Content:   contentMap[e.ID], // 已索引事件有内容，未索引为空字符串
					EventType: e.EventType,
					Severity:  e.Severity,
					Status:    e.Status,
					CveID:     e.CVEID,
					RiskScore: e.RiskScore,
					Source:    e.Source,
					Metadata:  e.Metadata,
					CreatedAt: e.CreatedAt.Format("2006-01-02 15:04:05"),
				}
				if e.IndexedAt != nil {
					r.IndexedAt = e.IndexedAt.Format("2006-01-02 15:04:05")
				}
				results = append(results, r)
			}

			b, _ := json.Marshal(results)
			return string(b), nil
		})
	if err != nil {
		log.Fatal(err)
	}
	return t
}

// fetchMilvusContentByIDs 通过事件 ID 批量从 Milvus biz 集合查询事件完整内容。
// 返回 map[eventID]content，查询失败或 ID 不在 Milvus 中时对应 key 不存在。
func fetchMilvusContentByIDs(ctx context.Context, ids []string) map[string]string {
	result := make(map[string]string)
	if len(ids) == 0 {
		return result
	}

	milvusCli, err := client.GetMilvusClient(ctx)
	if err != nil {
		return result
	}

	// 构造过滤表达式：id in ["id1", "id2"]
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = `"` + id + `"`
	}
	expr := fmt.Sprintf("id in [%s]", strings.Join(quoted, ", "))

	cols, err := milvusCli.Query(ctx, common.MilvusCollectionName, nil, expr, []string{"id", "content"})
	if err != nil {
		return result
	}

	// 解析列数据：id 列和 content 列长度一致，按下标对应
	var idData, contentData []string
	for _, col := range cols {
		vc, ok := col.(*entity.ColumnVarChar)
		if !ok {
			continue
		}
		switch col.Name() {
		case "id":
			idData = vc.Data()
		case "content":
			contentData = vc.Data()
		}
	}
	for i, id := range idData {
		if i < len(contentData) {
			result[id] = contentData[i]
		}
	}
	return result
}
