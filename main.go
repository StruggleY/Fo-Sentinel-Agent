// Fo-Sentinel-Agent 安全事件智能研判多智能体协同平台
// 数据流转：外部事件源 → Fetcher抓取 → Extractor提取 → Dedup去重 → MySQL存储 → Indexer向量化 → Milvus检索 → AI分析 → 用户界面
package main

import (
	"Fo-Sentinel-Agent/internal/ai/retrieval"
	"Fo-Sentinel-Agent/internal/ai/rule"
	aitrace "Fo-Sentinel-Agent/internal/ai/trace"
	"Fo-Sentinel-Agent/internal/controller/auth"
	"Fo-Sentinel-Agent/internal/controller/chat"
	"Fo-Sentinel-Agent/internal/controller/event"
	knowledgectrl "Fo-Sentinel-Agent/internal/controller/knowledge"
	ragevalctrl "Fo-Sentinel-Agent/internal/controller/rageval"
	"Fo-Sentinel-Agent/internal/controller/report"
	settingsctrl "Fo-Sentinel-Agent/internal/controller/settings"
	"Fo-Sentinel-Agent/internal/controller/subscription"
	termmapping "Fo-Sentinel-Agent/internal/controller/term_mapping"
	tracectrl "Fo-Sentinel-Agent/internal/controller/trace"
	dao "Fo-Sentinel-Agent/internal/dao/mysql"
	"Fo-Sentinel-Agent/internal/service/knowledge"
	"Fo-Sentinel-Agent/internal/service/scheduler"
	authPkg "Fo-Sentinel-Agent/utility/auth"
	"Fo-Sentinel-Agent/utility/middleware"

	einocallbacks "github.com/cloudwego/eino/callbacks"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"
)

func main() {
	ctx := gctx.New()

	// ========== 阶段1：认证与存储初始化 ==========
	// 初始化 JWT 认证（auth.jwt.enabled=true 时生效）
	if secret, err := g.Cfg().Get(ctx, "auth.jwt.secret"); err == nil {
		authPkg.Init(secret.String())
	}

	// 初始化 MySQL（事件/订阅/报告表），GORM 自动迁移，创建默认管理员
	if err := dao.Init(ctx); err != nil {
		g.Log().Warningf(ctx, "database init skipped or failed: %v", err)
	} else {
		dao.SeedAdmin(ctx)
		dao.SeedSettings(ctx)
		dao.SeedTermMappings(ctx)
		// 创建性能优化索引（幂等）
		dao.CreateIndexes(ctx)
		// 确保默认知识库存在
		knowledge.EnsureDefaultBase(ctx)
		// 注册 GORM plugin（慢查询追踪）
		if err := dao.RegisterPlugin(aitrace.NewGORMPlugin()); err != nil {
			g.Log().Warningf(ctx, "register gorm plugin failed: %v", err)
		}
	}

	// ========== 阶段2：AI组件初始化 ==========
	// 预热 Milvus Retriever 单例（向量检索 + 语义缓存）
	if err := retrieval.WarmUp(ctx); err != nil {
		g.Log().Warningf(ctx, "retrieval warmup failed: %v", err)
	}

	// 加载术语归一化规则到进程内存
	rule.InitTermMappings(ctx)

	// 注册全链路追踪 Callback（全局生效，所有 Eino DAG 节点自动触发）
	einocallbacks.AppendGlobalHandlers(aitrace.NewCallbackHandler())

	// ========== 阶段3：HTTP服务器配置 ==========
	s := g.Server()
	s.Group("/api", func(group *ghttp.RouterGroup) {
		// 中间件链：CORS → 响应格式化 → JWT认证
		group.Middleware(middleware.CORSMiddleware)
		group.Middleware(middleware.ResponseMiddleware)
		group.Middleware(middleware.JWTMiddleware()) // auth.jwt.enabled=true 时生效，登录接口自动放行
		// 绑定控制器：认证、聊天、事件、订阅、报告
		group.Bind(auth.NewV1())
		group.Bind(chat.NewV1())
		group.Bind(event.NewV1())
		group.Bind(subscription.NewV1())
		group.Bind(report.NewV1())
		group.Bind(settingsctrl.NewV1())
		group.Bind(termmapping.NewV1())
		group.Bind(tracectrl.NewV1())
		group.Bind(knowledgectrl.NewV1())
		group.Bind(ragevalctrl.NewV1())
	})

	// ========== 阶段4：后台任务启动 ==========
	// 启动知识库文档异步索引 Worker Pool
	knowledge.StartWorkerPool(ctx)

	// 启动定时调度器：每个订阅按自身 CronExpr 独立调度 + Indexer（间隔由配置 scheduler.index_interval_minutes 决定，默认 10 分钟）
	// 数据流：订阅源 → Fetcher → Extractor → Dedup → MySQL → Indexer → Milvus
	scheduler.Run(ctx)

	s.Run()
}
