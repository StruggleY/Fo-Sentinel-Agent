// Package breaker 实现三态熔断器，用于保护 LLM 模型调用。
//
// 背景：
// 项目依赖外部 LLM API,当某个端点出现故障时，若不加熔断保护，会引发以下问题：
//
//  1. 无效请求堆积：每次调用都会发起网络请求，等待失败后才返回错误，
//     高并发下大量请求同时阻塞，消耗连接池和 goroutine 资源。
//  2. 响应延迟飙升：网络超时类故障需等待超时时间（通常数十秒）才能感知失败，
//     导致 Agent 执行链路整体卡死，用户长时间无响应。
//  3. 故障扩散：底层模型持续失败会触发上层重试，进一步放大对故障端点的压力，
//     加速服务雪崩。
//  4. 无法自动恢复：即使故障端点已恢复，系统也无感知，需人工介入或等待重启。
//
// 熔断器通过快速失败（OPEN 状态直接拒绝请求）和自动探测恢复（HALF_OPEN 放行探测），
// 将故障影响范围控制在单个模型，并在服务恢复后自动切回，无需人工干预。
//
// 设计思路：
// 采用经典三态熔断器模式，每个模型 ID 独立维护健康状态，互不干扰。
// 失败计数达到阈值后立即熔断，冷却期结束后放行一个探测请求验证服务是否恢复。
//
// 三态转换：
//
//	CLOSED（正常）
//	  └─ 连续失败 ≥ FailureThreshold ──→ OPEN（熔断，拒绝所有请求）
//	                                         └─ 超过 OpenDuration ──→ HALF_OPEN（半开，放行一个探测）
//	                                                                      ├─ 探测成功 ──→ CLOSED
//	                                                                      └─ 探测失败 ──→ OPEN（重新计时）
package breaker

import (
	"sync"
	"time"
)

// state 熔断器状态
type state int

const (
	stateClosed   state = iota // 正常，放行所有请求
	stateOpen                  // 熔断，拒绝所有请求，等待冷却
	stateHalfOpen              // 半开，仅放行一个探测请求
)

// Config 熔断器参数配置，零值时使用内置默认值。
type Config struct {
	// FailureThreshold 连续失败多少次触发熔断，默认 2
	FailureThreshold int
	// OpenDuration 熔断持续时长，超时后进入 HALF_OPEN，默认 30s
	OpenDuration time.Duration
}

func (c *Config) threshold() int {
	if c.FailureThreshold <= 0 {
		return 2
	}
	return c.FailureThreshold
}

func (c *Config) openDuration() time.Duration {
	if c.OpenDuration <= 0 {
		return 30 * time.Second
	}
	return c.OpenDuration
}

// entry 单个模型的熔断器状态机。
// 每个模型 ID 对应一个独立 entry，状态互不影响。
type entry struct {
	mu                  sync.Mutex
	state               state
	consecutiveFailures int       // 当前连续失败次数（OPEN 时重置为 0）
	openUntil           time.Time // 熔断结束时间（仅 OPEN 状态有效）
	halfOpenInFlight    bool      // HALF_OPEN 时是否已有探测请求在途
}

// allowCall 判断是否允许本次调用，并在需要时推进状态机。
// CLOSED → 直接放行；
// OPEN 且未超时 → 拒绝；OPEN 超时 → 转 HALF_OPEN，放行一次探测；
// HALF_OPEN 且已有在途请求 → 拒绝（避免并发探测）；否则放行。
func (e *entry) allowCall() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	switch e.state {
	case stateOpen:
		if now.Before(e.openUntil) {
			return false
		}
		// 冷却期结束，转半开，放行一个探测请求
		e.state = stateHalfOpen
		e.halfOpenInFlight = true
		return true
	case stateHalfOpen:
		if e.halfOpenInFlight {
			return false // 已有探测在途，其他请求继续等待
		}
		e.halfOpenInFlight = true
		return true
	default: // CLOSED
		return true
	}
}

// markSuccess 标记本次调用成功，重置为 CLOSED 状态。
// 无论当前处于 HALF_OPEN 还是 CLOSED，成功均清零失败计数。
func (e *entry) markSuccess() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state = stateClosed
	e.consecutiveFailures = 0
	e.halfOpenInFlight = false
}

// markFailure 标记本次调用失败，推进熔断逻辑。
// HALF_OPEN 失败：直接重新熔断（探测失败说明服务未恢复）；
// CLOSED 失败：累加计数，达阈值则熔断。
func (e *entry) markFailure(cfg *Config) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.state == stateHalfOpen {
		// 探测失败，重新熔断并重置计时
		e.state = stateOpen
		e.openUntil = time.Now().Add(cfg.openDuration())
		e.halfOpenInFlight = false
		e.consecutiveFailures = 0
		return
	}
	e.consecutiveFailures++
	if e.consecutiveFailures >= cfg.threshold() {
		e.state = stateOpen
		e.openUntil = time.Now().Add(cfg.openDuration())
		e.consecutiveFailures = 0
	}
}

// Registry 按模型 ID 管理多个熔断器实例的注册表（线程安全）。
// 使用 RWMutex：读（查询已有 entry）远多于写（首次创建 entry），性能更优。
type Registry struct {
	mu      sync.RWMutex
	entries map[string]*entry
	cfg     Config
}

// New 创建熔断器注册表，cfg 零值时使用默认参数（threshold=2, openDuration=30s）。
func New(cfg Config) *Registry {
	return &Registry{
		entries: make(map[string]*entry),
		cfg:     cfg,
	}
}

// get 返回 id 对应的 entry，不存在则懒创建（double-check 避免重复写锁）。
func (r *Registry) get(id string) *entry {
	r.mu.RLock()
	e := r.entries[id]
	r.mu.RUnlock()
	if e != nil {
		return e
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if e = r.entries[id]; e == nil {
		e = &entry{}
		r.entries[id] = e
	}
	return e
}

// AllowCall 判断 id 对应模型是否允许发起调用。
// 返回 false 时调用方应直接返回错误或切换到下一个候选模型。
func (r *Registry) AllowCall(id string) bool { return r.get(id).allowCall() }

// MarkSuccess 标记 id 对应模型的本次调用成功。
// 非流式调用：Generate 返回无错误时调用；
// 流式调用：Stream 连接建立成功时调用（流读取阶段的错误需调用方主动上报 MarkFailure）。
func (r *Registry) MarkSuccess(id string) { r.get(id).markSuccess() }

// MarkFailure 标记 id 对应模型的本次调用失败。
// 非流式调用：Generate 返回错误时调用；
// 流式调用：Stream 连接失败或流读取超时/错误时调用。
func (r *Registry) MarkFailure(id string) { r.get(id).markFailure(&r.cfg) }
