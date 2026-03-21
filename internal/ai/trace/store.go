package trace

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	dao "Fo-Sentinel-Agent/internal/dao/mysql"

	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/gorm"
)

// retryOnDeadlock 在遇到 MySQL 死锁（Error 1213）时自动重试，最多重试 maxRetries 次。
// 每次重试前等待指数退避时间（50ms、100ms、200ms）。
func retryOnDeadlock(maxRetries int, fn func() *gorm.DB) error {
	delay := 50 * time.Millisecond
	for i := 0; i <= maxRetries; i++ {
		result := fn()
		if result.Error == nil {
			return nil
		}
		errStr := result.Error.Error()
		// MySQL Error 1213: Deadlock found when trying to get lock
		if strings.Contains(errStr, "1213") || strings.Contains(errStr, "Deadlock") {
			if i < maxRetries {
				time.Sleep(delay)
				delay *= 2
				continue
			}
		}
		return result.Error
	}
	return nil
}

// ── 配置设计 ──────────────────────────────────────────────────────────────────
//
// trace 系统的配置仅在首次请求时从 g.Cfg() 读取一次（sync.Once 单例），
// 避免每次请求都走配置热读路径，同时保持与 GoFrame 配置体系的兼容性。
//
// 配置项（manifest/config/config.yaml 中的 trace: 块）：
//
//	trace:
//	  enabled: true              # 总开关，false 时所有 trace API 为空操作
//	  max_error_length: 1000     # 错误消息截断长度（防止超长错误撑大 DB 行）
//	  record_prompt: false       # 是否记录 LLM completion 文本（含 PII 风险，默认关）

// Config trace 系统运行时配置，由 GetConfig() 初始化并缓存
type Config struct {
	Enabled        bool
	MaxErrorLength int
	RecordPrompt   bool
}

var (
	cfgOnce sync.Once
	cfg     *Config
)

// GetConfig 返回 trace 配置单例，首次调用时从 GoFrame 配置中心初始化。
// 后续调用直接返回缓存值，无锁读（sync.Once 保证初始化完成后只读）。
func GetConfig() *Config {
	cfgOnce.Do(func() {
		ctx := context.Background()
		enabled, _ := g.Cfg().Get(ctx, "trace.enabled")
		maxErrLen, _ := g.Cfg().Get(ctx, "trace.max_error_length")
		recordPrompt, _ := g.Cfg().Get(ctx, "trace.record_prompt")

		maxLen := maxErrLen.Int()
		if maxLen <= 0 {
			maxLen = 1000
		}

		cfg = &Config{
			Enabled:        enabled.Bool(),
			MaxErrorLength: maxLen,
			RecordPrompt:   recordPrompt.Bool(),
		}
	})
	return cfg
}

// IsEnabled 快速判断 trace 是否启用，供各埋点入口做快速路径判断（inline 友好）
func IsEnabled() bool {
	return GetConfig().Enabled
}

// ── 错误分类 ──────────────────────────────────────────────────────────────────
//
// classifyError 通过字符串匹配将原始错误归类为标准错误码。
// 目的：前端 Traces 页面可按 error_code 过滤，快速定位限流 / 超时 / 取消等模式。
func classifyError(err error) (code, errType string) {
	if err == nil {
		return ErrCodeUnknown, "NilError"
	}
	errStr := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429"):
		return ErrCodeRateLimit, "RateLimitError"
	case strings.Contains(errStr, "context canceled") || strings.Contains(errStr, "context cancelled"):
		return ErrCodeCanceled, "CanceledError"
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded"):
		return ErrCodeTimeout, "TimeoutError"
	case strings.Contains(errStr, "invalid") || strings.Contains(errStr, "bad request") || strings.Contains(errStr, "400"):
		return ErrCodeInvalidParam, "ValidationError"
	default:
		return ErrCodeInternal, "InternalError"
	}
}

// truncateError 截断错误消息到配置的最大长度，防止超长消息撑大数据库行
func truncateError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	maxLen := GetConfig().MaxErrorLength
	if len(msg) > maxLen {
		return msg[:maxLen]
	}
	return msg
}

// ── 异步写库 ──────────────────────────────────────────────────────────────────
//
// 设计原则：trace 写库不阻塞主请求链路。
//
// 所有 async* 函数启动独立 goroutine 执行数据库操作，主 goroutine 立即返回。
// 每个 goroutine 包含 recover() 防护，防止 DB 连接失败等异常向上 panic。

// asyncInsertRun 异步写入 TraceRun 初始记录（status=running）
func asyncInsertRun(run *dao.TraceRun) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[trace] asyncInsertRun panic: %v\n", r)
			}
		}()
		ctx := context.Background()
		db, err := dao.DB(ctx)
		if err != nil {
			return
		}
		db.Create(run)
	}()
}

// asyncInsertNode 异步写入 TraceNode 初始记录（status=running）
func asyncInsertNode(node *dao.TraceNode) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[trace] asyncInsertNode panic: %v\n", r)
			}
		}()
		ctx := context.Background()
		db, err := dao.DB(ctx)
		if err != nil {
			return
		}
		db.Create(node)
	}()
}

// NodeUpdate 封装节点结束时需要 UPDATE 的字段，按组件类型选填。
// 设计为值语义：字段零值表示"不更新此字段"，asyncFinishNode 只 UPDATE 非零字段。
//
// 字段分组：
//   - LLM 专属：模型名、Token 数、Prompt / Completion 文本（record_prompt=true 时记录）
//   - RETRIEVER 专属：查询文本、检索结果（top-3 截断序列化）、最终返回文档数、缓存命中标志
//   - 通用：Metadata JSON（TOOL 节点的 tool_name / tool_input / tool_output 等）
type NodeUpdate struct {
	// LLM 专属
	ModelName      string
	InputTokens    int
	OutputTokens   int
	CachedTokens   int
	PromptText     string
	CompletionText string
	// RETRIEVER 专属
	QueryText     string
	RetrievedDocs string
	FinalTopK     int
	CacheHit      bool
	// 通用
	Metadata string
}

// asyncFinishNode 异步将 TraceNode 由 running 更新为终态（success/error），
// 同时写入耗时、错误信息和节点专属字段（通过 NodeUpdate）。
// 只 UPDATE 非零值字段，避免覆盖 INSERT 时已写入的静态字段（node_type、node_name 等）。
func asyncFinishNode(nodeID, status, errMsg, errCode, errType string, startTime, endTime time.Time, update *NodeUpdate) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[trace] asyncFinishNode panic: %v\n", r)
			}
		}()
		ctx := context.Background()
		db, err := dao.DB(ctx)
		if err != nil {
			return
		}
		durationMs := endTime.Sub(startTime).Milliseconds()
		updates := map[string]any{
			"status":      status,
			"end_time":    endTime,
			"duration_ms": durationMs,
		}
		if errMsg != "" {
			updates["error_message"] = errMsg
		}
		if errCode != "" {
			updates["error_code"] = errCode
		}
		if errType != "" {
			updates["error_type"] = errType
		}
		if update != nil {
			if update.ModelName != "" {
				updates["model_name"] = update.ModelName
			}
			if update.InputTokens > 0 {
				updates["input_tokens"] = update.InputTokens
			}
			if update.OutputTokens > 0 {
				updates["output_tokens"] = update.OutputTokens
			}
			if update.CachedTokens > 0 {
				updates["cached_tokens"] = update.CachedTokens
			}
			if update.PromptText != "" {
				updates["prompt_text"] = update.PromptText
			}
			if update.CompletionText != "" {
				updates["completion_text"] = update.CompletionText
			}
			if update.QueryText != "" {
				updates["query_text"] = update.QueryText
			}
			if update.RetrievedDocs != "" {
				updates["retrieved_docs"] = update.RetrievedDocs
			}
			if update.FinalTopK > 0 {
				updates["final_top_k"] = update.FinalTopK
			}
			if update.CacheHit {
				updates["cache_hit"] = true
			}
			if update.Metadata != "" {
				updates["metadata"] = update.Metadata
			}
		}
		if err := retryOnDeadlock(3, func() *gorm.DB {
			return db.Model(&dao.TraceNode{}).Where("node_id = ? AND status = ?", nodeID, StatusRunning).Updates(updates)
		}); err != nil {
			fmt.Printf("[trace] asyncFinishNode update failed: %v\n", err)
		}
	}()
}

// asyncFinishRun 异步将 TraceRun 由 running 更新为终态，汇总全链路 Token 消耗。
// 由 FinishRun（tracer.go）在 Controller 层的 defer 中调用，保证请求结束时必然执行。
//
// 延迟清理：TraceRun 更新后等待 3s，强制将该链路下所有仍处于 running 状态的节点
// 更新为最终状态。这解决了 Eino streaming Lambda 节点的 OnEndWithStreamOutput goroutine
// 可能因流未关闭而永久阻塞，导致节点 status 卡在 running 的问题。
func asyncFinishRun(at *ActiveTrace, status, errMsg, errCode string, endTime time.Time) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[trace] asyncFinishRun panic: %v\n", r)
			}
		}()
		ctx := context.Background()
		db, err := dao.DB(ctx)
		if err != nil {
			return
		}
		durationMs := endTime.Sub(at.StartTime).Milliseconds()
		at.mu.Lock()
		totalIn := at.TotalInputTokens
		totalOut := at.TotalOutputTokens
		totalCached := at.TotalCachedTokens
		at.mu.Unlock()

		updates := map[string]any{
			"status":              status,
			"end_time":            endTime,
			"duration_ms":         durationMs,
			"total_input_tokens":  totalIn,
			"total_output_tokens": totalOut,
			"total_cached_tokens": totalCached,
		}
		if errMsg != "" {
			updates["error_message"] = errMsg
		}
		if errCode != "" {
			updates["error_code"] = errCode
		}
		if err := retryOnDeadlock(3, func() *gorm.DB {
			return db.Model(&dao.TraceRun{}).Where("trace_id = ?", at.TraceID).Updates(updates)
		}); err != nil {
			fmt.Printf("[trace] asyncFinishRun update failed: %v\n", err)
		}

		// 延迟 3s 强制将残留 running 节点更新为终态。
		// 背景：Eino streaming Lambda 节点的 OnEndWithStreamOutput 回调启动后台 goroutine 读流，
		// 若流未关闭该 goroutine 会永久阻塞，asyncFinishNode 永远不被调用，节点状态卡在 running。
		// 3s 给各 goroutine 足够时间正常完成，超时后强制兜底，确保链路视图显示正确状态。
		//
		// 关键优化：使用 at.DrainPendingNodeIDs() 获取确实未完成的节点 ID，
		// 用 WHERE node_id IN (...) 点更新替代原来的 WHERE trace_id = ? AND status = 'running' 范围扫描。
		// 范围扫描会触发 InnoDB Next-Key Lock，与并发的 asyncFinishNode 点更新产生死锁（Error 1213）。
		// 改为点更新后两侧均持有独立行锁，不再产生锁顺序冲突。
		time.AfterFunc(3*time.Second, func() {
			defer func() { recover() }()
			pendingIDs := at.DrainPendingNodeIDs()
			if len(pendingIDs) == 0 {
				return
			}
			ctx2 := context.Background()
			db2, err2 := dao.DB(ctx2)
			if err2 != nil {
				return
			}
			if err := retryOnDeadlock(3, func() *gorm.DB {
				return db2.Model(&dao.TraceNode{}).
					Where("node_id IN ? AND status = ?", pendingIDs, StatusRunning).
					Updates(map[string]any{
						"status":   status,
						"end_time": endTime,
					})
			}); err != nil {
				fmt.Printf("[trace] asyncFinishRun cleanup nodes failed: %v\n", err)
			}
		})
	}()
}
