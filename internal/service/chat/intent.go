// intent.go 意图识别业务逻辑：LLM 意图分发 + 多 Agent 执行。
package chatsvc

import (
	"context"

	"Fo-Sentinel-Agent/internal/ai/orchestration/intent_recognition"
)

// ExecuteIntent 执行意图识别和 Agent 分发，通过 onOutput 回调逐步推送结果。
// onOutput 参数：intentType 为当前意图类型（"chat"/"event"/"report" 等），chunk 为文本片段。
// 调用方负责将 onOutput 中的内容写入 SSE 或其他输出流。
func ExecuteIntent(ctx context.Context, sessionId, query string, onOutput func(intentType, chunk string)) error {
	ig := intent_recognition.NewIntent(ctx, sessionId)
	_, err := ig.Execute(query, func(intent intent_recognition.IntentType, chunk string) {
		onOutput(string(intent), chunk)
	})
	return err
}
