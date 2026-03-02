package main

import (
	"Fo-Sentinel-Agent/internal/ai/retriever"
	"Fo-Sentinel-Agent/internal/controller/chat"
	"Fo-Sentinel-Agent/utility/common"
	"Fo-Sentinel-Agent/utility/middleware"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"
)

func main() {
	ctx := gctx.New()
	fileDir, err := g.Cfg().Get(ctx, "file_dir")
	if err != nil {
		panic(err)
	}
	common.FileDir = fileDir.String()

	// 预热 Milvus Retriever 单例。
	// 触发一次 TCP 握手 + DB/Collection 元数据检查 + Embedding 客户端构建
	if err = retriever.WarmUp(ctx); err != nil {
		g.Log().Warningf(ctx, "retriever warmup failed: %v", err)
	}

	s := g.Server()
	s.Group("/api", func(group *ghttp.RouterGroup) {
		group.Middleware(middleware.CORSMiddleware)
		group.Middleware(middleware.ResponseMiddleware)
		group.Bind(chat.NewV1())
	})
	s.SetPort(6872)
	s.Run()
}
