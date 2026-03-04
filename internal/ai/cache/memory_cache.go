// Package cache 实现对话历史的分层记忆架构：短期记忆（最近对话）+ 长期摘要（早期总结）
//
// 核心设计：
//   - 短期记忆：保留最近 N 条消息（N 轮对话），原始内容，用于注入 Prompt
//   - 长期摘要：超过阈值时，自动总结前 M 条为摘要，释放内存
//
// 数据流：
//
//	用户消息 → SetMessages → 追加到短期记忆 → 超过阈值触发摘要 Agent → 更新长期摘要 → 删除已总结的消息
package cache

import (
	"Fo-Sentinel-Agent/internal/ai/agent/summary_pipeline"
	"context"
	"sync"
	"sync/atomic"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

var (
	// 从配置文件加载的参数（懒初始化）
	summaryTrigger   int // 触发总结的阈值
	summaryBatchSize int // 每次总结的消息数量
	configOnce       sync.Once
)

const historySummaryPrefix = "【历史对话摘要】\n"

// ChatMemory 单个会话的分层记忆容器
//
// 字段说明：
//   - RecentMessages：最近 N 轮对话的原始消息，直接注入 Prompt
//   - LongTermSummary：早期对话的总结文本
//   - summarizing：并发控制标志，防止同一会话重复触发总结
type ChatMemory struct {
	ID              string            `json:"id"`                // 聊天会话 ID
	RecentMessages  []*schema.Message `json:"recent_messages"`   // 最近的消息（动态大小）
	LongTermSummary string            `json:"long_term_summary"` // 长期摘要
	summarizing     int32             // 总结中标志
	mu              sync.Mutex        // 互斥锁（保护并发读写）
}

// MemoryMap 全局会话记忆注册表（进程内存存储，服务重启后清空）
var MemoryMap = make(map[string]*ChatMemory)

// mu 保护 MemoryMap 的全局锁（双层锁设计：外层保护 map 增删，内层保护单会话读写）
var mu sync.Mutex

// GetChatMemory 获取会话记忆实例（懒初始化），不存在则自动创建
func GetChatMemory(id string) *ChatMemory {
	loadConfig(context.Background())

	mu.Lock()
	defer mu.Unlock()

	if mem, ok := MemoryMap[id]; ok {
		return mem
	}

	// 创建新会话记忆
	newMem := &ChatMemory{
		ID:              id,
		RecentMessages:  []*schema.Message{},
		LongTermSummary: "",
		summarizing:     0, // 初始化为 0（未总结状态）
	}
	MemoryMap[id] = newMem
	return newMem
}

// GetRecentMessages 返回最近的原始消息列表
func (c *ChatMemory) GetRecentMessages() []*schema.Message {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.RecentMessages
}

// GetLongTermSummary 返回长期摘要内容
func (c *ChatMemory) GetLongTermSummary() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.LongTermSummary
}

// SetState 从外部状态恢复会话记忆，包括最近消息和长期摘要。
func (c *ChatMemory) SetState(recent []*schema.Message, summary string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.RecentMessages = recent
	c.LongTermSummary = summary
}

// BuildHistoryWithSummary 根据长期摘要和最近消息，构造用于 Prompt 的 History 列表。
// 结构为：
//   - 有摘要： [User(\"【历史对话摘要】\\n\"+summary), recent...]
//   - 无摘要： recent...
func BuildHistoryWithSummary(recent []*schema.Message, summary string) []*schema.Message {
	history := make([]*schema.Message, 0, len(recent)+1)
	if summary != "" {
		history = append(history, &schema.Message{
			Role:    schema.User,
			Content: historySummaryPrefix + summary,
		})
	}
	history = append(history, recent...)
	return history
}

// SetMessages 追加消息并触发总结机制
//
// 工作流程：
//   - 超过 summaryTrigger 条消息时触发异步总结
//   - 保留总结的消息+最近的消息
func (c *ChatMemory) SetMessages(msg *schema.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.RecentMessages = append(c.RecentMessages, msg)

	// 超过 summaryTrigger 时触发异步总结
	if len(c.RecentMessages) > summaryTrigger && atomic.CompareAndSwapInt32(&c.summarizing, 0, 1) {
		// 异步执行摘要，避免阻塞主请求（Chat / ChatStream）链路
		go c.summarizeOldMessages()
	}
}

// summarizeOldMessages 将早期对话总结为长期记忆（异步执行，调用独立的摘要 Agent）
//
// 渐进式总结策略：
//   - 触发条件：消息超过 summaryTrigger 条（从配置读取，默认 30）
//   - 总结范围：前 summaryBatchSize 条消息（从配置读取，默认 10）
//   - 保留范围：最近的消息（包括新消息）
//   - 设计目的：保留"热数据"，避免当前话题被压缩，实现平滑过渡
//
// 执行流程：
//  1. 提取前 summaryBatchSize 条消息用于总结
//  2. 调用 summary_pipeline.GetSummaryAgent() 获取摘要 Agent
//  3. 执行摘要 Agent（DAG 流水线：格式化 → 渲染 Prompt → LLM → 提取摘要）
//  4. 更新长期摘要
//  5. 删除已总结的前 summaryBatchSize 条消息，保留最近的消息
func (c *ChatMemory) summarizeOldMessages() {
	// 确保 summarizing 标志在函数退出时重置（使用 atomic 操作）
	defer atomic.StoreInt32(&c.summarizing, 0)

	c.mu.Lock()
	// 安全检查：如果消息不足 summaryBatchSize 条，不执行总结
	if len(c.RecentMessages) <= summaryBatchSize {
		c.mu.Unlock()
		return
	}

	// 渐进式总结：只提取前 summaryBatchSize 条用于总结
	// 保留最近的消息作为"热数据"缓冲区
	// 原因：避免当前正在讨论的话题被压缩丢失
	messagesToSummarize := make([]*schema.Message, summaryBatchSize)
	copy(messagesToSummarize, c.RecentMessages[:summaryBatchSize])
	sessionID := c.ID
	c.mu.Unlock()

	ctx := context.Background()

	// 步骤 1：获取摘要 Agent（懒初始化，全局单例）
	summaryAgent, err := summary_pipeline.GetSummaryAgent(ctx)
	if err != nil {
		g.Log().Errorf(ctx, "[SummaryAgent] failed to get summary agent for session %s: %v", sessionID, err)
		return
	}

	// 步骤 2：构建输入
	input := &summary_pipeline.SummaryInput{
		SessionID: sessionID,
		Messages:  messagesToSummarize,
	}

	// 步骤 3：执行摘要 Agent（DAG 流水线）
	output, err := summaryAgent.Invoke(ctx, input)
	if err != nil {
		g.Log().Errorf(ctx, "[SummaryAgent] failed to generate summary for session %s: %v", sessionID, err)
		return
	}

	// 步骤 4：更新长期记忆
	c.mu.Lock()
	defer c.mu.Unlock()

	c.LongTermSummary = output.Summary

	// 步骤 5：裁剪已总结的前 summaryBatchSize 条消息
	if len(c.RecentMessages) > summaryBatchSize {
		c.RecentMessages = c.RecentMessages[summaryBatchSize:]
	}

	// 为持久化构造当前状态快照，避免直接引用可变切片
	currentRecent := make([]*schema.Message, len(c.RecentMessages))
	copy(currentRecent, c.RecentMessages)
	currentSummary := c.LongTermSummary

	g.Log().Infof(ctx, "[SummaryAgent] summary completed for session %s, summary length: %d chars, remaining messages: %d",
		sessionID, len(output.Summary), len(c.RecentMessages))
	g.Log().Debugf(ctx, "[SummaryAgent] summary content for session %s:\n%s", sessionID, output.Summary)

	// 步骤 6：异步将最新的摘要 + 最近消息持久化到 Redis
	go func(recent []*schema.Message, summary string, sid string) {
		bgCtx := context.Background()
		if err := SetChatState(bgCtx, sid, recent, summary); err != nil {
			g.Log().Errorf(bgCtx, "[SummaryAgent] failed to persist summary state to redis, session_id=%s, error=%v", sid, err)
			return
		}
		g.Log().Infof(bgCtx, "[SummaryAgent] persisted summary state to redis, session_id=%s, recent_len=%d, has_summary=%t",
			sid, len(recent), summary != "")
	}(currentRecent, currentSummary, sessionID)
}

// loadConfig 从配置文件加载记忆管理参数（懒初始化，只执行一次）
func loadConfig(ctx context.Context) {
	configOnce.Do(func() {
		// 读取配置
		summaryTrigger = g.Cfg().MustGet(ctx, "memory.summaryTrigger").Int()
		summaryBatchSize = g.Cfg().MustGet(ctx, "memory.summaryBatchSize").Int()

		// 配置校验：确保配置值合理
		if summaryTrigger <= 0 {
			g.Log().Warningf(ctx, "[MemoryCache] invalid summaryTrigger(%d), must be > 0", summaryTrigger)
			summaryTrigger = 30 // 使用兜底值
		}

		if summaryBatchSize <= 0 {
			g.Log().Warningf(ctx, "[MemoryCache] invalid summaryBatchSize(%d), must be > 0", summaryBatchSize)
			summaryBatchSize = 10 // 使用兜底值
		}

		// 配置校验：summaryBatchSize 不应超过 summaryTrigger
		if summaryBatchSize >= summaryTrigger {
			g.Log().Warningf(ctx, "[MemoryCache] summaryBatchSize(%d) >= summaryTrigger(%d), adjusting to %d",
				summaryBatchSize, summaryTrigger, summaryTrigger/3)
			summaryBatchSize = summaryTrigger / 3
		}

		g.Log().Infof(ctx, "[MemoryCache] config loaded: summaryTrigger=%d, summaryBatchSize=%d",
			summaryTrigger, summaryBatchSize)
	})
}
