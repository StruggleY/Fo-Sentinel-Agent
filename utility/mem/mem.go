// Package mem 实现了一个基于内存的多会话对话历史管理模块。
//
// 设计背景：
// LLM 本身是无状态的（每次 API 调用相互独立），要实现多轮对话，
// 必须在每次请求时把历史消息列表一并发送给模型，由应用层自行维护"记忆"。
// 本模块就是承担这一职责的轻量化内存存储。
package mem

import (
	"sync"

	"github.com/cloudwego/eino/schema"
)

// SimpleMemoryMap 是全局的会话记忆注册表，key 为会话 ID，value 为对应的记忆实例。
// 使用进程内 map 存储，服务重启后历史消息会全部清空（无持久化）。
var SimpleMemoryMap = make(map[string]*SimpleMemory)

// mu 是保护 SimpleMemoryMap 读写的全局互斥锁。
// 注意这里采用"双层锁"设计：
//   - 外层 mu（包级别）：保护 map 的并发增删，防止多个 goroutine 同时写 map 导致 panic。
//   - 内层 SimpleMemory.mu（实例级别）：保护单个会话消息列表的并发读写。
//
// 两层锁分离的好处是：不同会话之间的消息操作完全不会互相阻塞，只有访问 map 本身时才短暂争抢外层锁。
var mu sync.Mutex

// GetSimpleMemory 根据会话 ID 返回对应的记忆实例（懒初始化单例模式）。
// 若该 ID 尚无记录，则自动创建并注册到全局 map。
// 整个函数在外层锁保护下执行，保证并发安全（即使多个请求同时首次访问同一 ID，也只会创建一个实例）。
func GetSimpleMemory(id string) *SimpleMemory {
	mu.Lock()
	defer mu.Unlock()
	// 如果存在就返回，不存在就创建
	if mem, ok := SimpleMemoryMap[id]; ok {
		return mem
	} else {
		newMem := &SimpleMemory{
			ID:            id,
			Messages:      []*schema.Message{},
			MaxWindowSize: 6, // 默认保留最近 6 条消息，即 3 轮对话（1 User + 1 Assistant = 1 轮）
		}
		SimpleMemoryMap[id] = newMem
		return newMem
	}
}

// SimpleMemory 是单个会话的对话历史容器。
//
// 字段说明：
//   - ID：会话唯一标识，与 HTTP 请求中的 req.Id 一一对应。
//   - Messages：消息列表，按时间顺序存储，格式遵循 OpenAI Messages 规范
//     （role: "user" / "assistant" / "system"），LLM 调用时直接传入。
//   - MaxWindowSize：上下文窗口容量上限，超出后滑动丢弃最旧的消息对。
//   - mu：实例级读写锁，保护本会话的并发安全。
type SimpleMemory struct {
	ID            string            `json:"id"`
	Messages      []*schema.Message `json:"messages"`
	MaxWindowSize int
	mu            sync.Mutex
}

// SetMessages 向当前会话追加一条新消息，并在超出窗口时执行滑动淘汰。
//
// 滑动窗口（Sliding Window）原理：
// LLM 的上下文长度（Context Length）是有限的，且 Token 消耗与历史长度正相关。
// 通过限制 MaxWindowSize，可以在保留近期对话连贯性的同时，控制每次请求的 Token 成本。
//
// 成对淘汰策略：
// 对话历史由 User/Assistant 消息交替构成，若只丢弃奇数条消息，会导致角色顺序错乱
// （例如连续出现两条 User 消息），使模型产生混淆。因此 excess 必须向上取偶数，
// 确保每次都完整丢弃一个或多个"User+Assistant"消息对，始终保持消息结构的完整性。
func (c *SimpleMemory) SetMessages(msg *schema.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Messages = append(c.Messages, msg)
	if len(c.Messages) > c.MaxWindowSize {
		// 计算超出窗口的消息数量
		excess := len(c.Messages) - c.MaxWindowSize
		if excess%2 != 0 {
			excess++ // 向上取偶，保证成对丢弃，维持 User/Assistant 交替结构
		}
		// 丢弃切片头部最旧的 excess 条消息，保留尾部最新的消息
		c.Messages = c.Messages[excess:]
	}
}

// GetMessages 返回当前会话的完整历史消息列表（只读快照）。
// 调用方（如 chat_pipeline）将此列表注入 Prompt Template 的 {history} 占位符，
// 列表中消息角色严格交替为 User → Assistant → User → Assistant...，
// 使 LLM 在回答时能感知到之前的对话上下文。
func (c *SimpleMemory) GetMessages() []*schema.Message {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.Messages
}
