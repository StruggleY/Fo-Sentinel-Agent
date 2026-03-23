package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

type FileUploadReq struct {
	g.Meta      `path:"/upload" method:"post" mime:"multipart/form-data" summary:"文件上传"`
	Strategy    string `json:"strategy"     form:"strategy"`     // 分块策略：fixed_size | structure_aware
	ChunkSize   int    `json:"chunk_size"   form:"chunk_size"`   // fixed_size：目标块大小（rune），默认 512
	OverlapSize int    `json:"overlap_size" form:"overlap_size"` // fixed_size：重叠大小（rune），默认 128
	TargetChars int    `json:"target_chars" form:"target_chars"` // structure_aware：目标块大小，默认 1400
	MaxChars    int    `json:"max_chars"    form:"max_chars"`    // structure_aware：最大块大小，默认 1800
	MinChars    int    `json:"min_chars"    form:"min_chars"`    // structure_aware：最小块大小，默认 600
}

type FileUploadRes struct {
	FileName string `json:"fileName" dc:"保存的文件名"`
	FilePath string `json:"filePath" dc:"文件保存路径"`
	FileSize int64  `json:"fileSize" dc:"文件大小(字节)"`
}

// ChatReq 多 Agent 对话请求。
// deep_thinking=false（标准模式）：Router LLM 识别意图 → SubAgent 分发（chat/event/report/risk/solve）。
// deep_thinking=true（深度思考模式）：直接进入 Plan Agent（Supervisor-Worker），跳过意图识别路由。
type ChatReq struct {
	g.Meta       `path:"/chat/v1/chat" method:"post" summary:"多 Agent 对话"`
	Query        string `json:"query" v:"required"`
	SessionId    string `json:"session_id"`    // 会话唯一标识，前端生成并持久化，用于跨轮次上下文隔离
	MessageIndex int    `json:"message_index"` // 消息在会话中的序号，用于关联用户反馈
	DeepThinking bool   `json:"deep_thinking"` // 深度思考模式：true 时直接进入 Plan Agent，跳过意图识别路由
	WebSearch    bool   `json:"web_search"`    // 联网搜索开关：true 时各 Agent 可调用 web_search 工具
}

// ChatRes 对话响应（SSE 流式，Body 为空，内容通过 SSE 推送）
type ChatRes struct {
}
