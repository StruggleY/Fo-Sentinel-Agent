// Package chat 提供对话相关 HTTP 控制器。
// 职责仅限 HTTP 层：解析请求、管理 SSE 连接、调用 chatsvc、映射响应 DTO。
// 业务逻辑（Agent 调用、记忆管理、Prompt 拼装、意图识别）已下沉至 internal/service/chat。
package chat

import (
	"context"
	"os"
	"path/filepath"
	"time"

	apichat "Fo-Sentinel-Agent/api/chat"
	v1 "Fo-Sentinel-Agent/api/chat/v1"
	chatsvc "Fo-Sentinel-Agent/internal/service/chat"
	"Fo-Sentinel-Agent/utility/common"
	"Fo-Sentinel-Agent/utility/sse"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
)

type ControllerV1 struct{}

func NewV1() apichat.IChatV1 {
	return &ControllerV1{}
}

// Chat 同步阻塞式对话，等待完整回答后一次性返回。
func (c *ControllerV1) Chat(ctx context.Context, req *v1.ChatReq) (*v1.ChatRes, error) {
	answer, err := chatsvc.Chat(ctx, req.Id, req.Question)
	if err != nil {
		return nil, err
	}
	return &v1.ChatRes{Answer: answer}, nil
}

// ChatStream 流式对话，SSE 逐 token 推送。
// 心跳机制：每 15s 发送 SSE 注释行，防止代理或负载均衡因超时断开连接。
func (c *ControllerV1) ChatStream(ctx context.Context, req *v1.ChatStreamReq) (*v1.ChatStreamRes, error) {
	client := sse.NewClient(g.RequestFromCtx(ctx))

	// 启动心跳 goroutine，cancelHeartbeat 在函数退出时自动触发
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				client.SendHeartbeat()
			case <-heartbeatCtx.Done():
				return
			}
		}
	}()

	// 调用 service 流式对话，onChunk 回调将每个 chunk 推送到 SSE
	err := chatsvc.StreamChat(ctx, req.Id, req.Question, func(chunk string) {
		client.Send("message", chunk)
	})
	if err != nil {
		client.Send("error", err.Error())
	} else {
		client.Send("done", "Stream completed")
	}
	client.Done()
	return &v1.ChatStreamRes{}, nil
}

// FileUpload 上传知识文档并构建向量索引，单次上限 50 MB。
// 文件解析和保存依赖 GoFrame API，必须保留在 HTTP 层；向量索引构建委托 chatsvc.BuildFileIndex。
func (c *ControllerV1) FileUpload(ctx context.Context, req *v1.FileUploadReq) (*v1.FileUploadRes, error) {
	const maxUploadBytes int64 = 50 << 20 // 50 MB

	r := g.RequestFromCtx(ctx)
	uploadFile := r.GetUploadFile("file")
	if uploadFile == nil {
		return nil, gerror.New("请上传文件")
	}
	if uploadFile.Size > maxUploadBytes {
		return nil, gerror.Newf("文件过大（%.1f MB），单次上传上限为 50 MB", float64(uploadFile.Size)/(1<<20))
	}
	// 存储目录不存在时自动创建
	if !gfile.Exists(common.FileDir) {
		if err := gfile.Mkdir(common.FileDir); err != nil {
			return nil, gerror.Wrapf(err, "创建目录失败: %s", common.FileDir)
		}
	}
	savePath := filepath.Join(common.FileDir)
	// 保存文件到磁盘，false 表示不覆盖同名文件
	if _, err := uploadFile.Save(savePath, false); err != nil {
		return nil, gerror.Wrapf(err, "保存文件失败")
	}
	fileInfo, err := os.Stat(savePath)
	if err != nil {
		return nil, gerror.Wrapf(err, "获取文件信息失败")
	}
	// 构建向量索引（Milvus 去重 + 嵌入写入）
	if err = chatsvc.BuildFileIndex(ctx, common.FileDir+"/"+uploadFile.Filename); err != nil {
		return nil, gerror.Wrapf(err, "构建知识库失败")
	}
	return &v1.FileUploadRes{
		FileName: uploadFile.Filename,
		FilePath: savePath,
		FileSize: fileInfo.Size(),
	}, nil
}

// Intent 意图驱动多 Agent 对话入口，SSE 流式推送 type+content。
// 委托 chatsvc.ExecuteIntent 完成意图识别和 Agent 分发，onOutput 回调写入 SSE。
func (c *ControllerV1) Intent(ctx context.Context, req *v1.IntentChatReq) (*v1.IntentChatRes, error) {
	client := sse.NewClient(g.RequestFromCtx(ctx))

	// onOutput 回调：SSE event type = 意图类型，data = 文本片段
	err := chatsvc.ExecuteIntent(ctx, req.SessionId, req.Query, func(intentType, chunk string) {
		client.Send(intentType, chunk)
	})
	if err != nil {
		g.Log().Errorf(ctx, "[Intent] execute failed: %v", err)
		client.Send("error", err.Error())
	}

	client.Done()
	return nil, nil
}
