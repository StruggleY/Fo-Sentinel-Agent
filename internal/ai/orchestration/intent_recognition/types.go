// Package intent_recognition 实现基于 LLM 意图识别的多 Agent 调度系统。
//
// 架构原理：用户查询先经 Router 节点（调用 DeepSeek 做意图识别），识别出目标 IntentType，
// 再由 Executor 节点按 IntentType 从 registry 取出对应 SubAgent 执行，最终结果经 StreamCallback
// 流式回传给调用方。整体以 Eino Graph 编排为 DAG：START → Router → Executor → END。
//
//	core ← 公共类型、注册表、共享记忆（无内部依赖）
//	subagents ← core + 各 pipeline（子 Agent 适配器）
//	intent_recognition ← core + blank import subagents（编排层）
package intent_recognition

import "Fo-Sentinel-Agent/internal/ai/orchestration/intent_recognition/core"

type (
	IntentType     = core.IntentType
	Task           = core.Task
	Result         = core.Result
	StreamCallback = core.StreamCallback
	SubAgent       = core.SubAgent
	IntentInput    = core.IntentInput
	IntentOutput   = core.IntentOutput
)

// 意图常量别名
const (
	IntentChat   = core.IntentChat
	IntentEvent  = core.IntentEvent
	IntentReport = core.IntentReport
	IntentRisk   = core.IntentRisk
	IntentPlan   = core.IntentPlan
	IntentSolve  = core.IntentSolve
	IntentStatus = core.IntentStatus
)
