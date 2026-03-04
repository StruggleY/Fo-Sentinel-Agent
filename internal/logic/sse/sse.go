package sse

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/container/gmap"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
)

// Client 表示一个 SSE 客户端长连接。
//
//   - Id：客户端唯一标识，优先取请求中的 client_id，否则自动生成 UUID。
//   - Request：持有原始 HTTP 请求对象，通过 Request.Response 直接向该连接写入推送数据。
//   - messageChan：消息缓冲通道。
type Client struct {
	Id          string
	Request     *ghttp.Request
	messageChan chan string
}

// Service 管理所有活跃的 SSE 客户端连接。
// clients 使用 GoFrame 的并发安全 map（gmap.StrAnyMap），
// 构造时传入 true 开启内部读写锁，支持多 goroutine 并发注册/注销客户端。
type Service struct {
	clients *gmap.StrAnyMap
}

// New 创建 SSE Service 实例，通常在应用启动时初始化一次，全局复用。
func New() *Service {
	return &Service{
		clients: gmap.NewStrAnyMap(true),
	}
}

// Create 建立一条 SSE 长连接，设置必要的响应头并立即发送 connected 事件。
//
// SSE（Server-Sent Events）协议要点：
//   - 基于普通 HTTP，服务端通过持续写入响应体向客户端单向推送数据，连接在函数返回前保持不关闭。
//   - 每条事件由若干 "字段: 值\n" 行组成，事件之间以空行（\n\n）分隔。
//   - 客户端（浏览器 EventSource）会自动解析事件流，无需额外的 WebSocket 握手。
//
// 响应头说明：
//   - Content-Type: text/event-stream  ── SSE 协议的强制要求，客户端据此识别为事件流
//   - Cache-Control: no-cache          ── 禁止中间代理缓存，确保每条事件实时到达客户端
//   - Connection: keep-alive           ── 保持 TCP 连接不断开，支撑持续推送
//   - Access-Control-Allow-Origin: *   ── 允许跨域访问，前端页面可跨域订阅事件流
func (s *Service) Create(ctx context.Context, r *ghttp.Request) (*Client, error) {
	r.Response.Header().Set("Content-Type", "text/event-stream")
	r.Response.Header().Set("Cache-Control", "no-cache")
	r.Response.Header().Set("Connection", "keep-alive")
	r.Response.Header().Set("Access-Control-Allow-Origin", "*")

	// 优先使用请求中传入的 client_id（由上层 ctx 注入），
	// 若不存在则用 guid.S() 生成全局唯一 ID，保证每条连接可被独立寻址。
	clientId := r.Get("client_id", guid.S()).String()
	client := &Client{
		Id:          clientId,
		Request:     r,
		messageChan: make(chan string, 100),
	}

	// 注册客户端到全局 Map，支持服务端主动向指定会话推送消息（如工具执行进度通知）
	s.clients.Set(client.Id, client)

	// 监听 ctx.Done() 自动注销客户端，防止 goroutine 泄漏和僵尸连接
	go func() {
		<-ctx.Done()
		s.clients.Remove(client.Id)
	}()

	// 启动消息推送 goroutine，从 messageChan 消费并写入 HTTP Response
	// 解耦 LLM 生成速度与网络发送速度，防止背压阻塞 ReAct 主循环
	client.StartPusher(ctx)

	// 立即向客户端推送 connected 事件，告知连接已建立。
	// Flush() 强制将缓冲区数据立即写入 TCP，否则数据可能在缓冲区滞留，客户端无法及时收到。
	r.Response.Writefln("id: %s", clientId)
	r.Response.Writefln("event: connected")
	r.Response.Writefln("data: {\"status\": \"connected\", \"client_id\": \"%s\"}\n", clientId)
	r.Response.Flush()
	return client, nil
}

// StartPusher 启动后台 goroutine，从 messageChan 消费消息并写入 HTTP Response。
//
// 设计原理：
//   - LLM 流式输出速度 >> 网络发送速度时，若同步写 response 会产生背压阻塞 ReAct 循环
//   - 通过 channel 解耦：SendToClient 非阻塞写入 channel，pusher goroutine 独立消费
//   - channel buffer=100，若填满则 SendToClient 丢弃（背压保护，不影响 LLM 推理）
func (c *Client) StartPusher(ctx context.Context) {
	go func() {
		for {
			select {
			case msg := <-c.messageChan:
				c.Request.Response.Writefln(msg)
				c.Request.Response.Flush()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// SendToClient 向客户端推送一条 SSE 事件（非阻塞，写入 messageChan）。
//
// SSE 事件格式（每字段独占一行，事件以空行结尾）：
//
//	id: <纳秒时间戳>
//	event: <eventType>    // 事件类型，前端用 addEventListener(eventType, ...) 监听
//	data: <data>          // 事件数据，前端通过 event.data 读取
//	                      // ← 末尾空行标志本条事件结束
//
// eventType 约定：
//   - "message" ── LLM 生成的 Token 片段（chunk），前端拼接后展示
//   - "done"    ── 流式输出正常结束
//   - "error"   ── 推理过程发生异常
//
// id 使用纳秒时间戳，确保每条事件 ID 唯一，客户端断线重连时可凭 Last-Event-ID 续接。
//
// 返回值：true=成功写入 channel，false=channel 满（背压保护，丢弃该消息）
func (c *Client) SendToClient(eventType, data string) bool {
	msg := fmt.Sprintf(
		"id: %d\nevent: %s\ndata: %s\n\n",
		time.Now().UnixNano(), eventType, data,
	)
	// 非阻塞写入 channel，满时丢弃（不阻塞 LLM 推理主循环）
	select {
	case c.messageChan <- msg:
		return true
	default:
		return false // channel 满，背压保护
	}
}

// SendHeartbeat 发送 SSE 心跳注释行（浏览器忽略，但保持 TCP 连接活跃）。
//
// 用途：
//   - Nginx/代理服务器通常在 60s 无数据时断开长连接
//   - ReAct 工具调用（如日志查询）可能耗时较长，此时 SSE 连接无数据输出
//   - 定时发送心跳注释行（`: keepalive\n\n`），防止代理超时断连
//
// 调用频率：建议 15s 一次（低于代理超时时间即可）
func (c *Client) SendHeartbeat() {
	c.Request.Response.Writefln(": keepalive\n")
	c.Request.Response.Flush()
}

// PushToClient 向指定 client_id 的会话主动推送消息（供服务端内部调用）。
//
// 用途：
//   - 工具执行进度通知（如"正在查询日志..."）
//   - 后台任务完成通知
//
// 返回值：true=推送成功，false=该 client_id 不存在或已断线
func (s *Service) PushToClient(clientId, eventType, data string) bool {
	if v := s.clients.Get(clientId); v != nil {
		return v.(*Client).SendToClient(eventType, data)
	}
	return false
}
