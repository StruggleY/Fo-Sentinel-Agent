// Package sse 提供统一的 Server-Sent Events 工具。
//
// 所有 SSE 端点使用同一套 Client，推送标准 SSE 格式：
//
//	event: <type>\n
//	data: <content>\n
//	\n
//
// 流结束标志：data: [DONE]\n\n
package sse

import (
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
)

// Client SSE 写入器，直接向 HTTP Response 写入标准 SSE 事件。
type Client struct {
	Id      string
	Request *ghttp.Request
}

// NewClient 创建 Client 并设置 SSE 必要响应头。
func NewClient(r *ghttp.Request) *Client {
	r.Response.Header().Set("Content-Type", "text/event-stream")
	r.Response.Header().Set("Cache-Control", "no-cache")
	r.Response.Header().Set("Connection", "keep-alive")
	r.Response.Header().Set("Access-Control-Allow-Origin", "*")
	return &Client{
		Id:      guid.S(),
		Request: r,
	}
}

// Send 推送一条标准 SSE 事件并立即 Flush。
// eventType 为事件类型（如 "message"、"error"），data 为事件内容。
// 如果 data 包含换行符，会自动分成多行，每行都以 "data: " 开头（符合 SSE 协议）。
func (c *Client) Send(eventType, data string) {
	c.Request.Response.Writef("id: %d\nevent: %s\n", time.Now().UnixNano(), eventType)

	// 处理多行内容：将每行都以 "data: " 开头
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		c.Request.Response.Writef("data: %s\n", line)
	}

	c.Request.Response.Writef("\n")
	c.Request.Response.Flush()
}

// Done 推送流结束标志，通知前端关闭读流。
func (c *Client) Done() {
	c.Request.Response.Writef("data: [DONE]\n\n")
	c.Request.Response.Flush()
}

// SendHeartbeat 推送 SSE 心跳注释行，防止代理/浏览器因空闲超时断连。
// 建议每 15s 调用一次。
func (c *Client) SendHeartbeat() {
	c.Request.Response.Writefln(": keepalive\n")
	c.Request.Response.Flush()
}
