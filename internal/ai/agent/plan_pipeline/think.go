package plan_pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"Fo-Sentinel-Agent/internal/ai/models"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// thinkingSystemPrompt 指导模型以安全专家视角深度思考用户问题。
// 只输出推理过程，不给出最终结论，为后续 Plan Agent 规划奠定分析基础。
const thinkingSystemPrompt = `你是一位资深网络安全专家和系统分析师。
用户即将提出一个安全研判相关的问题，在制定分析计划之前，请先输出你的完整思考过程：

思考要求：
1. 理解问题本质：用户真正想知道什么？问题背后的业务场景是什么？
2. 分析关键维度：涉及哪些安全领域（事件分析/风险评估/威胁情报/应急响应）？
3. 识别约束条件：时间范围、严重程度、资产范围等关键约束
4. 确定分析路径：应该按什么顺序分析，为什么这个顺序最优？
5. 预判关键挑战：分析过程中可能遇到的数据缺失、模糊判断点

使用中文，思考过程应清晰、有条理、体现专业判断，不需要给出最终答案。`

// ThinkChunkMsg 是思考阶段推送给前端的事件结构，通过 plan_step SSE 事件发送。
type ThinkChunkMsg struct {
	Type    string `json:"type"`    // 固定为 "think"
	Content string `json:"content"` // 思考文本片段（增量）
}

// MarshalThinkChunk 序列化思考片段为 JSON 字符串，失败时直接返回内容原文。
func MarshalThinkChunk(content string) string {
	b, err := json.Marshal(ThinkChunkMsg{Type: "think", Content: content})
	if err != nil {
		return content
	}
	return string(b)
}

// StreamThinkChunks 调用 DeepSeek Think 模型对用户查询进行预思考，
// 实时流式推送推理内容，每个文本片段触发一次 onChunk 回调。
//
// 支持三种模式（自动检测）：
//   - reasoning 模式：模型通过 ReasoningContent 字段输出推理（如 DeepSeek-R1 API）
//   - think 模式：模型在文本中输出 <think>...</think> 标签
//   - plain 模式：普通文本输出（如 DeepSeek V3），直接实时流式推送
//
// 错误不中断主流程：调用方应忽略返回值，继续执行 Plan Agent。
func StreamThinkChunks(ctx context.Context, query string, onChunk func(string)) error {
	cm, err := models.OpenAIForDeepSeekV31Think(ctx)
	if err != nil {
		return fmt.Errorf("初始化 Think 模型: %w", err)
	}

	msgs := []*schema.Message{
		schema.SystemMessage(thinkingSystemPrompt),
		schema.UserMessage(query),
	}

	stream, err := cm.Stream(ctx, msgs)
	if err != nil {
		return fmt.Errorf("Think 模型调用失败: %w", err)
	}
	defer stream.Close()

	// mode 自动检测：
	//   ""          — 检测中（等待足够内容判断模型类型）
	//   "reasoning" — ReasoningContent 字段模式（DeepSeek-R1 API）
	//   "think"     — <think> 标签模式
	//   "plain"     — 普通文本，直接实时推送
	var (
		mode       string
		buf        strings.Builder
		thinkStart int
		lastPushed int
	)

	for {
		msg, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			g.Log().Warningf(ctx, "[StreamThink] 流读取中断: %v", recvErr)
			break
		}
		if msg == nil {
			continue
		}

		// ── ReasoningContent 模式：直接实时推送 ──────────────────────────
		if msg.ReasoningContent != "" {
			if mode == "" {
				mode = "reasoning"
			}
		}
		if mode == "reasoning" {
			if msg.ReasoningContent != "" {
				onChunk(msg.ReasoningContent)
			}
			continue
		}

		chunk := msg.Content
		if chunk == "" {
			continue
		}
		buf.WriteString(chunk)
		cur := buf.String()

		// ── 检测阶段：判断是 <think> 标签还是普通文本 ───────────────────
		if mode == "" {
			if i := strings.Index(cur, "<think>"); i >= 0 {
				// 检测到 <think> 标签，切换到 think 过滤模式
				mode = "think"
				thinkStart = i + len("<think>")
				lastPushed = 0
			} else if len([]rune(cur)) >= 15 {
				// 15 字后仍无 <think> 标签 → 普通文本模式，推送已缓冲内容后实时流式
				mode = "plain"
				onChunk(cur)
				lastPushed = buf.Len()
			}
			// 仍在检测中，继续缓冲
			continue
		}

		// ── 普通文本模式：每个 chunk 实时推送 ───────────────────────────
		if mode == "plain" {
			onChunk(chunk)
			lastPushed = buf.Len()
			continue
		}

		// ── <think> 标签模式：仅推送标签内内容 ─────────────────────────
		if mode == "think" {
			thinkBody := cur[thinkStart:]
			endIdx := strings.Index(thinkBody, "</think>")
			done := endIdx >= 0
			if done {
				thinkBody = thinkBody[:endIdx]
			}
			if len(thinkBody) > lastPushed {
				onChunk(thinkBody[lastPushed:])
				lastPushed = len(thinkBody)
			}
			if done {
				break
			}
		}
	}

	// 兜底：检测阶段结束时仍有未推送的缓冲内容
	if mode == "" && buf.Len() > 0 {
		onChunk(buf.String())
	}

	return nil
}
