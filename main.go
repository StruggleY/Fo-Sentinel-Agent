// Fo-Sentinel-Agent 安全哨兵系统主程序
// 功能：安全事件智能研判多智能体协同平台
// 数据流转：外部事件源 → Fetcher抓取 → Extractor提取 → Dedup去重 → MySQL存储 → Indexer向量化 → Milvus检索 → AI分析 → 用户界面
package main

import (
	"Fo-Sentinel-Agent/internal/ai/retriever"
	"Fo-Sentinel-Agent/internal/ai/skills"
	authPkg "Fo-Sentinel-Agent/internal/auth"
	"Fo-Sentinel-Agent/internal/controller/auth"
	"Fo-Sentinel-Agent/internal/controller/chat"
	"Fo-Sentinel-Agent/internal/controller/event"
	"Fo-Sentinel-Agent/internal/controller/report"
	settingsctrl "Fo-Sentinel-Agent/internal/controller/settings"
	skillctrl "Fo-Sentinel-Agent/internal/controller/skills"
	"Fo-Sentinel-Agent/internal/controller/subscription"
	"Fo-Sentinel-Agent/internal/dao"
	"Fo-Sentinel-Agent/internal/service/scheduler"
	"Fo-Sentinel-Agent/utility/common"
	"Fo-Sentinel-Agent/utility/middleware"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gctx"
)

func main() {
	ctx := gctx.New()

	// ========== 阶段1：配置初始化 ==========
	fileDir, err := g.Cfg().Get(ctx, "file_dir")
	if err != nil {
		panic(err)
	}
	common.FileDir = fileDir.String()

	// ========== 阶段2：认证与存储初始化 ==========
	// 初始化 JWT 认证（auth.jwt.enabled=true 时生效）
	if secret, err := g.Cfg().Get(ctx, "auth.jwt.secret"); err == nil {
		authPkg.Init(secret.String())
	}

	// 初始化 MySQL（事件/订阅/报告表），GORM 自动迁移，创建默认管理员
	if err = dao.Init(ctx); err != nil {
		g.Log().Warningf(ctx, "database init skipped or failed: %v", err)
	} else {
		dao.SeedAdmin(ctx)
		dao.SeedSettings(ctx)
	}

	// ========== 阶段3：AI组件初始化 ==========
	// 预热 Milvus Retriever 单例（向量检索 + 语义缓存）
	if err = retriever.WarmUp(ctx); err != nil {
		g.Log().Warningf(ctx, "retriever warmup failed: %v", err)
	}

	// 加载 skills/ 目录下的 SKILL.md 技能定义（事件分析、威胁狩猎等）
	if err = skills.LoadAllSkills("skills"); err != nil {
		g.Log().Warningf(ctx, "failed to load skills: %v", err)
	}

	// ========== 阶段4：HTTP服务器配置 ==========
	s := g.Server()
	s.Group("/api", func(group *ghttp.RouterGroup) {
		// 中间件链：CORS → 响应格式化 → JWT认证
		group.Middleware(middleware.CORSMiddleware)
		group.Middleware(middleware.ResponseMiddleware)
		group.Middleware(middleware.JWTMiddleware()) // auth.jwt.enabled=true 时生效，登录接口自动放行
		// 绑定控制器：认证、聊天、技能、事件、订阅、报告
		group.Bind(auth.NewV1())
		group.Bind(chat.NewV1())
		group.Bind(skillctrl.NewV1())
		group.Bind(event.NewV1())
		group.Bind(subscription.NewV1())
		group.Bind(report.NewV1())
		group.Bind(settingsctrl.NewV1())
	})
	s.SetPort(6872)

	// ========== 阶段5：后台任务启动 ==========
	// 启动定时调度器：每个订阅按自身 CronExpr 独立调度 + Indexer（间隔由配置 scheduler.index_interval_minutes 决定，默认 10 分钟）
	// 数据流：订阅源 → Fetcher → Extractor → Dedup → MySQL → Indexer → Milvus
	scheduler.Run(ctx)

	s.Run()
}
