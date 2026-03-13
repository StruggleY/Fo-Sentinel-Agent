package notify

import "context"

// Message 通知消息
type Message struct {
	Title   string
	Content string
	Level   string // info, warning, error
}

// Send 发送通知（当前无可用通知渠道，保留接口供后续扩展）
func Send(_ context.Context, _ Message) error {
	return nil
}
