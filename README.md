# Fo-Sentinel-Agent

**安全事件智能研判多智能体协同平台** — 通过多 Agent AI 架构自动化安全事件监控、分析与响应

![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)
![GoFrame](https://img.shields.io/badge/GoFrame-v2.7.1-blue?style=flat)
![Eino](https://img.shields.io/badge/Eino-v0.6.0-purple?style=flat)
![React](https://img.shields.io/badge/React-18-61DAFB?style=flat&logo=react)
![License](https://img.shields.io/badge/License-MIT-green?style=flat)

---

## 项目简介

Fo-Sentinel-Agent 是一套面向安全运营团队的智能哨兵系统。系统从多种来源（漏洞情报、威胁情报源、厂商公告、GitHub 安全公告）自动收集安全事件，通过 **6 个专业 AI Agent** 协同完成事件去重、关联分析、风险评估与报告生成，最终输出可操作的安全处置建议。

---

## 系统架构

```
┌───────────────────────────────────────────────────────────────────────────────────┐
│                              用户 / 外部系统                                        │
│                    Web UI (React 18)      │      REST API                          │
└──────────────────────────────────┬────────────────────────────────────────────────┘
                                   │
                                   ▼
┌───────────────────────────────────────────────────────────────────────────────────┐
│                         API 路由层  (GoFrame v2 · :6872)                           │
│   /api/chat/v1/*   /api/event/v1/*   /api/report/v1/*   /api/skill/v1/*           │
│   /api/subscription/v1/*   /api/settings/v1/*   /api/auth/v1/*                    │
│                                                                                   │
│   Controller 层：参数校验 · JWT 认证 · SessionId 注入 · SSE 响应头设置              │
└──────────────────────────────────┬────────────────────────────────────────────────┘
                                   │
                                   ▼
┌───────────────────────────────────────────────────────────────────────────────────┐
│                    意图识别与路由层  (Intent Recognition)                            │
│                                                                                   │
│   输入  ──►  Router (DeepSeek V3 Quick)  ──►  Executor DAG                        │
│                          │                                                        │
│              ┌───────────┼──────────────────────┐                                │
│              │           │           │           │           │           │        │
│            chat        event      report        risk        plan       solve      │
│         (通用对话)    (事件分析)  (报告生成)   (风险评估)  (多步规划)  (应急响应)   │
│                                                                                   │
│   容错降级：Router 失败 / 未知意图 / SubAgent 错误  ──►  Chat Agent                │
└──────┬───────────┬───────────┬───────────┬───────────┬───────────┬───────────────┘
       │           │           │           │           │           │
       ▼           ▼           ▼           ▼           ▼           ▼
┌────────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
│ Chat Agent │ │  Event   │ │  Report  │ │  Risk    │ │  Plan    │ │  Solve   │
│            │ │  Agent   │ │  Agent   │ │  Agent   │ │  Agent   │ │  Agent   │
│ ReAct+RAG  │ │ ReAct    │ │ ReAct    │ │ ReAct    │ │ Plan-    │ │ ReAct    │
│ 分层记忆   │ │ +RAG     │ │ +RAG     │ │ +RAG     │ │ Execute  │ │ +RAG     │
│ max 25步   │ │ max 15步 │ │ max 15步 │ │ max 15步 │ │ max 20轮 │ │ max 15步 │
│            │ │          │ │          │ │          │ │ Replan   │ │          │
│ 11个工具   │ │ 3个工具  │ │ 3个工具  │ │ 2个工具  │ │ 全部工具 │ │ 2个工具  │
└─────┬──────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘
      │              │            │             │            │            │
      │         ┌────┴────────────┴─────────────┴────────────┘            │
      │         │         RAG 检索管道（Eino Graph）                        │
      │         │   InputToRag ──► Embedder ──► Retriever ──► Template     │
      │         │                   (DashScope)   (Milvus)                 │
      └─────────┴──────────────────────┬──────────────────────────────────┘
                                       │
                                       ▼
┌───────────────────────────────────────────────────────────────────────────────────┐
│                            工具调用层  (11 个工具)                                  │
│                                                                                   │
│  ┌─── event/ ────────────────────┐  ┌─── observe/ ──────────────────────────┐   │
│  │  query_events                 │  │  query_log          (MCP CLS)          │   │
│  │  search_similar_events        │  │  query_metrics_alerts (Prometheus)     │   │
│  │  query_subscriptions          │  └───────────────────────────────────────┘   │
│  └───────────────────────────────┘                                               │
│  ┌─── report/ ───────────────────┐  ┌─── system/ ───────────────────────────┐   │
│  │  query_reports                │  │  get_current_time                      │   │
│  │  query_report_templates       │  │  query_database                        │   │
│  │  create_report                │  │  query_internal_docs                   │   │
│  └───────────────────────────────┘  └───────────────────────────────────────┘   │
└────────────────────┬──────────────────────────┬──────────────────────────────────┘
                     │                          │ 后台调度器（异步）
         ┌───────────┼───────────┐              │
         ▼           ▼           ▼              ▼
    ┌─────────┐ ┌─────────┐ ┌─────────┐  ┌────────────────────────────────────┐
    │  MySQL  │ │  Redis  │ │ Milvus  │  │  Scheduler (goroutine)             │
    │─────────│ │─────────│ │─────────│  │  Fetcher: RSS/GitHub 抓取         │
    │ events  │ │语义缓存  │ │事件向量  │  │  每 15 分钟 → 写入 MySQL           │
    │ reports │ │对话历史  │ │文档向量  │  │  Indexer: 向量嵌入 → 写入 Milvus   │
    │ subscr. │ │会话摘要  │ │相似检索  │  │  每 20 分钟                        │
    │ users   │ │         │ │         │  └────────────────────────────────────┘
    └─────────┘ └─────────┘ └─────────┘
```

---

## 核心功能

- **多源事件订阅**：支持 RSS、GitHub 等多种安全情报来源，自动定时抓取
- **智能去重聚合**：基于 `dedup_key` 对重复事件去重，向量相似度聚合关联事件
- **多 Agent 协同分析**：6 个专业 Agent + 意图路由，自动分配分析任务
- **RAG 增强检索**：Milvus 向量数据库 + 语义缓存，快速检索相关历史事件与知识库
- **分层记忆管理**：短期消息历史 + 长期摘要压缩，支持跨会话上下文持久化
- **结构化报告生成**：自动生成周报/月报/自定义安全分析报告，存储于 MySQL
- **SSE 流式输出**：所有 AI 分析过程实时推流，用户可见 AI 推理步骤与工具调用
- **可插拔技能系统**：通过 SKILL.md 声明式定义技能，支持参数化 Prompt 模板
- **风险评估与应急响应**：CVE/CVSS 风险评分 + 攻击路径分析 + 处置优先级建议
- **JWT 认证（可选）**：支持多用户角色与访问控制

---

## 技术栈

| 层次 | 技术 | 说明 |
|------|------|------|
| 后端框架 | GoFrame v2.7.1 | HTTP 服务器、配置管理、日志 |
| AI 编排 | Cloudwego Eino v0.6.0 | 多 Agent 管道编排、ReAct、Graph |
| 对话模型 | DeepSeek V3 | 主要推理与分析（OpenAI 兼容接口）|
| 嵌入模型 | DashScope text-embedding-v4 | 向量嵌入 |
| 关系数据库 | MySQL 8.0+ | 事件、订阅、报告、用户持久化 |
| 向量数据库 | Milvus 2.x | 事件向量检索（RAG） |
| 缓存 | Redis 7.x | 语义缓存 + 对话历史 |
| 前端 | React 18 + TypeScript + Vite | 现代化 Web 界面 |
| 状态管理 | Zustand | 前端全局状态 |
| 样式 | TailwindCSS | 响应式 UI |

---

## 快速开始

### 前置条件

- Go 1.24+
- Node.js 18+ / npm
- MySQL 8.0+
- Redis 7.x
- Milvus 2.x（可选，关闭 RAG 可不启动）

### 1. 克隆仓库

```bash
git clone https://github.com/your-org/fo-sentinel-agent.git
cd fo-sentinel-agent
```

### 2. 启动依赖服务（Docker Compose）

```bash
docker compose -f manifest/docker/docker-compose.yml up -d
```

这将启动 MySQL（3307 端口）、Redis（6379）、Milvus（19530）。

### 3. 配置

复制并编辑配置文件：

```bash
cp manifest/config/config.yaml manifest/config/config.local.yaml
```

**必填配置项：**

```yaml
# LLM 模型配置
ai:
  ds_think_chat_model:
    api_key: "your-deepseek-api-key"     # DeepSeek V3 API Key
  doubao_embedding_model:
    api_key: "your-dashscope-api-key"    # DashScope API Key

# 数据库
database:
  mysql:
    dsn: "root:password@tcp(127.0.0.1:3307)/fo_sentinel"

# Redis
redis:
  addr: "127.0.0.1:6379"
```

### 4. 启动后端

```bash
go mod tidy
go run main.go
# 服务运行在 http://localhost:6872
```

### 5. 启动前端

```bash
cd web
npm install
npm run dev
# 前端运行在 http://localhost:3001
```

---

## 配置参考

所有配置位于 `manifest/config/config.yaml`：

### AI 模型

| 配置项 | 类型 | 说明 | 默认值 |
|--------|------|------|--------|
| `ai.ds_think_chat_model.api_key` | string | DeepSeek V3 API Key（主推理） | - |
| `ai.ds_think_chat_model.model` | string | 模型名称 | `deepseek-chat` |
| `ai.ds_quick_chat_model.api_key` | string | DeepSeek V3 Quick API Key（意图识别） | - |
| `ai.doubao_embedding_model.api_key` | string | DashScope 向量嵌入 API Key | - |

### 存储

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `database.mysql.dsn` | MySQL 连接字符串 | `root:sentinel123@tcp(127.0.0.1:3307)/fo_sentinel` |
| `redis.addr` | Redis 地址 | `localhost:6379` |
| `redis.chat_cache.ttl` | 对话历史 TTL | `720h`（30 天） |
| `redis.semantic_cache.threshold` | 语义缓存相似度阈值 | `0.85` |
| `redis.semantic_cache.ttl` | 语义缓存 TTL | `24h` |

### 认证

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `auth.jwt.enabled` | 是否启用 JWT 认证 | `false` |
| `auth.jwt.expire_hours` | Token 有效期（小时） | `24` |
| `auth.jwt.secret` | JWT 签名密钥 | - |
| `auth.seed.admin_password` | 初始管理员密码 | - |

### 调度器

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `scheduler.fetch_interval_minutes` | 事件抓取间隔（分钟） | `15` |
| `scheduler.index_interval_minutes` | 向量索引间隔（分钟） | `20` |

### 记忆管理

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `memory.summaryTrigger` | 触发摘要压缩的消息数阈值 | `30` |
| `memory.summaryBatchSize` | 每次压缩的消息条数 | `10` |

---

## API 文档

> 所有接口前缀 `/api`，启用 JWT 时需在请求头添加 `Authorization: Bearer <token>`

### 认证

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | `/api/auth/v1/login` | 用户登录，返回 JWT Token | 无 |

### 聊天

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | `/api/chat/v1/chat` | 普通对话（非流式） | 可选 |
| POST | `/api/chat/v1/chat_stream` | 对话流式输出（SSE） | 可选 |
| POST | `/api/chat/v1/intent_recognition` | 意图驱动多 Agent 对话（SSE） | 可选 |
| POST | `/api/chat/v1/upload` | 上传知识文档（PDF/TXT） | 可选 |

### 事件

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/event/v1/list` | 事件列表（分页、过滤） | 可选 |
| POST | `/api/event/v1/create` | 手动创建事件 | 可选 |
| GET | `/api/event/v1/stats` | 事件统计数据 | 可选 |
| GET | `/api/event/v1/trend` | 事件趋势图数据 | 可选 |
| POST | `/api/event/v1/update_status` | 更新事件状态 | 可选 |
| POST | `/api/event/v1/delete` | 删除事件 | 可选 |
| POST | `/api/event/v1/analyze/stream` | 单条事件 AI 分析（SSE） | 可选 |
| POST | `/api/event/v1/pipeline/stream` | 事件分析管道（SSE） | 可选 |

### 技能

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/skill/v1/list` | 获取可用技能列表 | 可选 |
| POST | `/api/skill/v1/execute` | 按名称执行技能（SSE） | 可选 |

### 报告

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/report/v1/list` | 报告列表 | 可选 |
| POST | `/api/report/v1/create` | 生成安全报告 | 可选 |
| GET | `/api/report/v1/get` | 获取报告详情 | 可选 |
| POST | `/api/report/v1/delete` | 删除报告 | 可选 |
| GET | `/api/report/v1/template/list` | 获取报告模板列表 | 可选 |

### 订阅管理

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/subscription/v1/list` | 订阅列表 | 可选 |
| POST | `/api/subscription/v1/create` | 添加订阅源 | 可选 |
| POST | `/api/subscription/v1/update` | 更新订阅 | 可选 |
| POST | `/api/subscription/v1/delete` | 删除订阅 | 可选 |
| POST | `/api/subscription/v1/pause` | 暂停订阅 | 可选 |
| POST | `/api/subscription/v1/resume` | 恢复订阅 | 可选 |
| GET | `/api/subscription/v1/logs` | 抓取日志 | 可选 |
| POST | `/api/subscription/v1/fetch` | 手动触发抓取 | 可选 |

### 系统设置

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/settings/v1/general` | 获取全局配置 | 可选 |
| POST | `/api/settings/v1/general` | 更新全局配置 | 可选 |

### SSE 流式协议

```
data: {"type":"content","content":"文本块"}
data: {"type":"tool_call","tool":"query_events","args":{...}}
data: {"type":"tool_result","tool":"query_events","result":{...}}
data: [DONE]
```

---

## 多智能体系统

### Agent 一览

| Agent | 职责 | 架构 | 最大步数 | 工具 |
|-------|------|------|---------|------|
| **Chat Agent** | 通用对话，知识库问答 | ReAct + RAG + 分层记忆 | 25 | 11 个 |
| **Event Analysis Agent** | 安全事件关联分析、威胁研判 | 5 节点 DAG + ReAct + RAG | 15 | query_events, search_similar, query_subscriptions |
| **Report Agent** | 生成结构化安全报告（周报/月报） | 5 节点 DAG + ReAct + RAG | 15 | create_report, query_reports, query_templates |
| **Risk Agent** | CVE/CVSS 风险评分，攻击路径分析 | 5 节点 DAG + ReAct + RAG | 15 | query_events, search_similar |
| **Plan Agent** | 多步复杂任务规划与执行 | Plan-Execute-Replan 循环 | 20 轮 | 所有工具 |
| **Solve Agent** | 单条事件应急响应方案生成 | ReAct + RAG | 15 | query_events, query_subscriptions |
| **Summary Agent** | 对话历史压缩（长期记忆） | 4 节点线性流水线 | - | 无（触发阈值：30 条消息） |

### 意图识别路由

```
用户输入
   │
   ▼
Router (DeepSeek V3 Quick)
   │
   ├── chat   → Chat Agent
   ├── event  → Event Analysis Agent
   ├── report → Report Agent
   ├── risk   → Risk Agent
   ├── plan   → Plan Agent
   └── solve  → Solve Agent
         │
         ▼
   容错降级：任意失败 → Chat Agent
```

**三层架构：**
- `core/`：公共类型（IntentType、SubAgent 接口、Registry）
- `subagents/`：各 Agent 适配器层
- `intent_recognition/`：Router + Executor 编排层

### 工具系统

| 工具名称 | 分类 | 功能说明 | 使用 Agent |
|----------|------|----------|-----------|
| `query_events` | event/ | 查询安全事件（过滤、分页） | Event、Plan、Chat |
| `search_similar_events` | event/ | 向量相似度搜索事件 | Event、Risk |
| `query_subscriptions` | event/ | 查询订阅源配置 | Event、Chat |
| `query_log` | observe/ | 查询系统日志（MCP CLS） | Chat、Plan |
| `query_metrics_alerts` | observe/ | 查询监控指标和告警（Prometheus） | Event、Risk |
| `query_reports` | report/ | 查询已生成的报告 | Report |
| `query_report_templates` | report/ | 获取报告模板 | Report |
| `create_report` | report/ | 创建并保存安全报告 | Report |
| `get_current_time` | system/ | 获取当前时间（用于时间范围查询） | 所有 ReAct Agent |
| `query_database` | system/ | 通用数据库查询 | Plan |
| `query_internal_docs` | system/ | 查询内部文档知识库 | Chat |

### Skills 技能系统

技能通过 `skills/<skill-name>/SKILL.md` 声明式定义，支持参数化 Prompt 模板：

| 技能名称 | 功能 | 使用工具 |
|----------|------|----------|
| `event-analysis` | 深度分析指定安全事件 | query_events, search_similar_events |
| `threat-hunting` | 威胁狩猎分析 | query_events, query_metrics_alerts |
| `log-diagnosis` | 日志异常诊断 | query_log, get_current_time |

执行技能示例：
```bash
POST /api/skill/v1/execute
Content-Type: application/json

{
  "name": "event-analysis",
  "params": {
    "event_id": "123",
    "depth": "full"
  },
  "session_id": "user-session-001"
}
```

---

## 项目结构

```
fo-sentinel-agent/
├── api/                        # API 接口定义（GoFrame 路由绑定）
│   ├── auth/v1/                # 认证接口
│   ├── chat/v1/                # 聊天接口
│   ├── event/v1/               # 事件接口
│   ├── report/v1/              # 报告接口
│   ├── skill/v1/               # 技能接口
│   ├── subscription/v1/        # 订阅接口
│   └── settings/v1/            # 设置接口
├── internal/
│   ├── ai/
│   │   ├── agent/              # 专业 Agent 管道实现
│   │   │   ├── chat_pipeline/              # Chat Agent
│   │   │   ├── event_analysis_pipeline/    # Event Analysis Agent
│   │   │   ├── report_pipeline/            # Report Agent
│   │   │   ├── risk_pipeline/              # Risk Agent
│   │   │   ├── plan_pipeline/              # Plan Agent
│   │   │   ├── solve_pipeline/             # Solve Agent
│   │   │   └── summary_pipeline/           # Summary Agent
│   │   ├── orchestration/
│   │   │   └── intent_recognition/         # 意图路由系统
│   │   │       ├── core/                   # 公共类型与注册表
│   │   │       └── subagents/              # SubAgent 适配器
│   │   ├── cache/              # Redis 缓存（对话历史、语义缓存）
│   │   ├── skills/             # 技能执行引擎
│   │   └── tools/              # 工具实现
│   │       ├── event/          # query_events, search_similar, query_subscriptions
│   │       ├── observe/        # query_log, query_metrics_alerts
│   │       ├── report/         # query_reports, create_report, query_templates
│   │       └── system/         # get_current_time, query_database, query_internal_docs
│   ├── auth/                   # JWT 中间件
│   ├── controller/             # HTTP 控制器
│   ├── database/               # GORM 模型（Event, Subscription, Report, User）
│   └── service/
│       └── scheduler/          # 后台任务（Fetcher + Indexer）
├── manifest/
│   ├── config/config.yaml      # 主配置文件
│   └── docker/docker-compose.yml
├── skills/                     # 技能定义目录（SKILL.md 文件）
│   ├── event-analysis/SKILL.md
│   ├── threat-hunting/SKILL.md
│   └── log-diagnosis/SKILL.md
├── utility/                    # 公共工具（客户端、中间件）
│   ├── client/                 # Milvus、Redis 客户端
│   ├── middleware/             # CORS、JWT、响应包装
│   └── sse/                    # SSE 工具库
├── web/                        # React 前端
│   └── src/
│       ├── components/         # ChatView, EventAnalysis, Dashboard 等
│       ├── contexts/           # AppContext（Zustand 全局状态）
│       ├── hooks/              # React Hooks
│       └── lib/                # API 客户端（axios）
└── main.go                     # 程序入口（端口 6872）
```

---

## 开发指南

### 添加新 Agent

1. 在 `internal/ai/agent/<name>_pipeline/` 创建目录，实现以下文件：
   - `flow.go`：Agent 管道图（Lambda/Graph）
   - `prompt.go`：系统提示词
   - `model.go`：LLM 客户端配置

2. 在 `internal/ai/orchestration/intent_recognition/subagents/` 创建适配器，实现 `SubAgent` 接口：
   ```go
   type SubAgent interface {
       Execute(ctx context.Context, task *core.Task) (*core.Result, error)
   }
   ```

3. 在 `internal/ai/orchestration/intent_recognition/core/registry.go` 注册：
   ```go
   registry.Register(core.IntentYourType, subagents.NewYourAgent(cfg))
   ```

4. 更新 `intent_recognition/router.go` 的意图识别 Prompt，添加新意图的识别示例。

### 添加新工具

1. 在 `internal/ai/tools/<category>/<tool_name>.go` 创建工具文件，实现 `tool.BaseTool` 接口：
   ```go
   func (t *YourTool) Name() string { return "your_tool" }
   func (t *YourTool) Description() string { return "工具描述" }
   func (t *YourTool) ParametersJSONSchema() map[string]any { ... }
   func (t *YourTool) Invoke(ctx context.Context, args map[string]any) (string, error) { ... }
   ```

2. 在 `internal/ai/skills/executor.go` 的 `globalToolMap` 中注册工具实例。

3. 在目标 Agent 的 `flow.go` 中将工具加入 ReAct 工具列表。

### 添加新技能

在 `skills/<skill-name>/SKILL.md` 创建文件，格式如下：

```markdown
# Skill Name
描述技能功能

## Tools
- query_events
- search_similar_events

## Prompt
你是一个安全分析专家，请针对事件 {event_id} 进行深度分析...
```

技能将在服务启动时自动加载，无需修改代码。

### 修改数据库模型

更新 `internal/database/model.go`，GORM 在启动时自动执行 `AutoMigrate`。

---

## 部署

### 开发环境（Docker Compose）

```bash
# 启动所有依赖服务
docker compose -f manifest/docker/docker-compose.yml up -d

# 查看服务状态
docker compose -f manifest/docker/docker-compose.yml ps
```

服务端口：
- MySQL：`3307`
- Redis：`6379`
- Milvus：`19530`（gRPC）、`9091`（HTTP）

### 生产部署检查清单

- [ ] 修改 `auth.seed.admin_password`（初始管理员密码）
- [ ] 启用 JWT：`auth.jwt.enabled: true`
- [ ] 设置强随机 `auth.jwt.secret`
- [ ] 配置 HTTPS（反向代理 Nginx/Caddy）
- [ ] 配置 Redis 持久化（RDB 或 AOF）
- [ ] Milvus 生产集群替换单机版
- [ ] 监控 LLM API 配额（DeepSeek、DashScope）
- [ ] 配置日志收集与告警

### 构建生产二进制

```bash
# 后端
go build -o fo-sentinel-agent main.go

# 前端
cd web && npm run build
# 构建产物在 web/dist/，由后端 static 服务托管
```

---

## 数据模型

| 表名 | 主要字段 | 说明 |
|------|----------|------|
| `events` | id, title, event_type, dedup_key, severity, source, status, cve_id, risk_score, metadata, indexed_at | 安全事件主表 |
| `subscriptions` | id, name, url, type(rss/github), cron_expr, enabled, last_fetch_at | 订阅源配置 |
| `reports` | id, title, content, type(weekly/monthly/custom) | 生成的安全报告 |
| `users` | id, username, password(bcrypt), role(admin/user) | 用户账户 |
| `fetch_logs` | id, subscription_id, status, fetched_count, new_count, duration_ms | 抓取任务日志 |
| `settings` | key, value | 全局系统配置 |

---

## 前端路由

| 路径 | 组件 | 说明 |
|------|------|------|
| `/` | 重定向 | 默认跳转 `/chat` |
| `/chat` | ChatView | 主对话界面（意图路由入口） |
| `/events/analysis` | EventAnalysis | 事件分析独立页面 |
| `/events/list` | EventList | 事件管理与状态更新 |
| `/dashboard` | Dashboard | 工作台（事件统计卡片） |
| `/reports` | ReportList | 安全报告列表与详情 |
| `/subscriptions` | SubscriptionList | 订阅源管理 |
| `/skills` | SkillsPanel | 技能面板（执行技能） |

---

## 许可证

MIT License — 详见 [LICENSE](LICENSE) 文件

---

## 贡献

欢迎提交 Issue 和 Pull Request。提交代码前请确保：

```bash
go fmt ./...      # 格式化代码
go vet ./...      # 静态检查
go test ./...     # 运行测试
```
