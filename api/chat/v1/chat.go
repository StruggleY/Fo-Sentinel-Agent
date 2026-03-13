package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

type ChatReq struct {
	g.Meta   `path:"/chat" method:"post" summary:"对话"`
	Id       string
	Question string
}

type ChatRes struct {
	Answer string `json:"answer"`
}

type ChatStreamReq struct {
	g.Meta   `path:"/chat_stream" method:"post" summary:"流式对话"`
	Id       string
	Question string
}

type ChatStreamRes struct {
}

type FileUploadReq struct {
	g.Meta `path:"/upload" method:"post" mime:"multipart/form-data" summary:"文件上传"`
}

type FileUploadRes struct {
	FileName string `json:"fileName" dc:"保存的文件名"`
	FilePath string `json:"filePath" dc:"文件保存路径"`
	FileSize int64  `json:"fileSize" dc:"文件大小(字节)"`
}

// IntentChatReq Intent 意图驱动多 Agent 对话请求
type IntentChatReq struct {
	g.Meta    `path:"/chat/v1/intent_recognition" method:"post" summary:"Intent 意图驱动多 Agent 对话"`
	Query     string `json:"query" v:"required"`
	SessionId string `json:"session_id"` // 会话唯一标识，前端生成并持久化，用于跨轮次上下文隔离
}

// IntentChatRes Intent 响应（SSE 流式）
type IntentChatRes struct {
}
