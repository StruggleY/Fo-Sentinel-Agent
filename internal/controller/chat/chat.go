// Package chat 提供对话相关 HTTP 控制器。
// 职责仅限 HTTP 层：解析请求、管理 SSE 连接、调用 chatsvc、映射响应 DTO。
// 业务逻辑（Agent 调用、记忆管理、Prompt 拼装、意图识别）已下沉至 internal/service/chat。
package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	apichat "Fo-Sentinel-Agent/api/chat"
	v1 "Fo-Sentinel-Agent/api/chat/v1"
	aidoc "Fo-Sentinel-Agent/internal/ai/document"
	toolsintelligence "Fo-Sentinel-Agent/internal/ai/tools/intelligence"
	"Fo-Sentinel-Agent/internal/ai/trace"
	chatsvc "Fo-Sentinel-Agent/internal/service/chat"
	"Fo-Sentinel-Agent/utility/sse"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
)

type ControllerV1 struct{}

func NewV1() apichat.IChatV1 {
	return &ControllerV1{}
}

// FileUpload 上传知识文档并构建向量索引，单次上限 50 MB。
// 支持格式：.txt .md .markdown .pdf .docx .pptx
// 文件解析和保存依赖 GoFrame API，必须保留在 HTTP 层；向量索引构建委托 chatsvc.BuildFileIndex。
func (c *ControllerV1) FileUpload(ctx context.Context, req *v1.FileUploadReq) (*v1.FileUploadRes, error) {
	const maxUploadBytes int64 = 50 << 20 // 50 MB

	fileDir, err := g.Cfg().Get(ctx, "file_dir")
	if err != nil {
		return nil, gerror.Wrap(err, "读取文件目录配置失败")
	}
	fileDirPath := fileDir.String()

	r := g.RequestFromCtx(ctx)
	uploadFile := r.GetUploadFile("file")
	if uploadFile == nil {
		return nil, gerror.New("请上传文件")
	}
	if uploadFile.Size > maxUploadBytes {
		return nil, gerror.Newf("文件过大（%.1f MB），单次上传上限为 50 MB", float64(uploadFile.Size)/(1<<20))
	}

	// 扩展名白名单校验
	allowedExt := map[string]bool{
		".txt": true, ".md": true, ".markdown": true,
		".pdf": true, ".docx": true, ".pptx": true,
	}
	ext := filepath.Ext(uploadFile.Filename)
	if !allowedExt[ext] {
		return nil, gerror.Newf("不支持的文件格式 %s，支持：.txt .md .markdown .pdf .docx .pptx", ext)
	}

	// 启动链路追踪（文件验证通过后再启动，避免记录无效请求）
	ctx = trace.StartRun(ctx, "chat.file_upload", "/api/chat/v1/file_upload", "",
		uploadFile.Filename, map[string]any{"file_size": uploadFile.Size})
	var uploadErr error
	defer func() { trace.FinishRun(ctx, uploadErr) }()

	// 存储目录不存在时自动创建
	if !gfile.Exists(fileDirPath) {
		if err := gfile.Mkdir(fileDirPath); err != nil {
			uploadErr = err
			return nil, gerror.Wrapf(err, "创建目录失败: %s", fileDirPath)
		}
	}
	savePath := filepath.Join(fileDirPath)
	// 保存文件到磁盘，false 表示不覆盖同名文件
	if _, err := uploadFile.Save(savePath, false); err != nil {
		uploadErr = err
		return nil, gerror.Wrapf(err, "保存文件失败")
	}
	fileInfo, err := os.Stat(savePath)
	if err != nil {
		uploadErr = err
		return nil, gerror.Wrapf(err, "获取文件信息失败")
	}

	// 从请求参数构造分块配置（零值字段使用默认值）
	cfg := aidoc.DefaultChunkConfig()
	if req.Strategy != "" {
		cfg.Strategy = aidoc.ChunkStrategy(req.Strategy)
	}
	if req.ChunkSize > 0 {
		cfg.ChunkSize = req.ChunkSize
	}
	if req.OverlapSize > 0 {
		cfg.OverlapSize = req.OverlapSize
	}
	if req.TargetChars > 0 {
		cfg.TargetChars = req.TargetChars
	}
	if req.MaxChars > 0 {
		cfg.MaxChars = req.MaxChars
	}
	if req.MinChars > 0 {
		cfg.MinChars = req.MinChars
	}

	// 构建向量索引（Milvus 去重 + 嵌入写入）
	if err = chatsvc.BuildFileIndex(ctx, fileDirPath+"/"+uploadFile.Filename, cfg); err != nil {
		uploadErr = err
		return nil, gerror.Wrapf(err, "构建知识库失败")
	}
	return &v1.FileUploadRes{
		FileName: uploadFile.Filename,
		FilePath: savePath,
		FileSize: fileInfo.Size(),
	}, nil
}

// Chat 多 Agent 对话入口，SSE 流式推送 type+content。
//
// ┌─ deep_thinking=false（标准模式）─────────────────────────────────────────────────┐
// │  Router LLM 识别意图（chat/event/report/risk/solve）→ Executor 分发 SubAgent    │
// │  SSE 事件：status + chat/event/report/risk/solve                               │
// └─────────────────────────────────────────────────────────────────────────────────┘
// ┌─ deep_thinking=true（深度思考模式）──────────────────────────────────────────────┐
// │  跳过路由，直接进入 Plan Agent（Supervisor-Worker）                              │
// │  Planner 规划步骤 → Executor 按步调用 Worker（event/report/risk/solve Agent）    │
// │  SSE 事件：status + plan_step（中间步骤）+ plan（最终答案）                      │
// └─────────────────────────────────────────────────────────────────────────────────┘
func (c *ControllerV1) Chat(ctx context.Context, req *v1.ChatReq) (*v1.ChatRes, error) {
	// 启动链路追踪（新增）
	tags := map[string]any{"deep_thinking": req.DeepThinking}
	ctx = trace.StartRun(ctx, "chat.intent", "/api/chat/v1/intent", req.SessionId, req.Query, tags)
	var execErr error
	defer func() { trace.FinishRun(ctx, execErr) }()

	client := sse.NewClient(g.RequestFromCtx(ctx))

	// 推送 meta 事件：携带会话元数据，供前端显示 Agent 状态和调试追踪
	metaData := fmt.Sprintf(`{"sessionId":%q,"timestamp":%q,"deepThinking":%v}`,
		req.SessionId, time.Now().Format(time.RFC3339), req.DeepThinking)
	client.Send("meta", metaData)

	// 心跳 goroutine：每 15 秒推送一次 keepalive 注释行，防止 Plan Agent 长时间执行期间
	// 代理或浏览器因空闲超时关闭 SSE 连接。
	heartbeatDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				client.SendHeartbeat()
			case <-heartbeatDone:
				return
			}
		}
	}()

	var err error
	// WithoutCancel 隔离 Agent 执行上下文，使整个 Agent 链路不受 HTTP 客户端断连影响。
	// 客户端导航/切换对话时浏览器关闭 TCP → GoFrame 取消 request ctx（标准 net/http 行为）→
	// 若不隔离，Router LLM 节点会立即收到 context.Canceled，整个 intent DAG 提前终止。
	// agentCtx 保留所有 ctx value（trace、session 等），仅移除取消传播，确保 Agent 完整执行。
	// SSE write 对已关闭连接会静默失败（GoFrame Flush 不 panic），Backend 资源正常回收。
	agentCtx := context.WithoutCancel(ctx)
	if req.WebSearch {
		agentCtx = context.WithValue(agentCtx, toolsintelligence.WebSearchEnabledKey{}, true)
	}
	if req.DeepThinking {
		err = chatsvc.ExecuteDeepThink(agentCtx, req.SessionId, req.Query, func(intentType, chunk string) {
			client.Send(intentType, chunk)
		})
	} else {
		// 标准意图路由：onOutput 回调写入 SSE
		err = chatsvc.ExecuteIntent(agentCtx, req.SessionId, req.Query, func(intentType, chunk string) {
			client.Send(intentType, chunk)
		})
	}
	execErr = err

	// 意图执行结束（正常或异常），停止心跳
	close(heartbeatDone)

	if err != nil {
		g.Log().Errorf(ctx, "[Intent] 执行失败 | deep_thinking=%v | err=%v", req.DeepThinking, err)
		client.Send("error", err.Error())
	}

	client.Done()
	return nil, nil
}
