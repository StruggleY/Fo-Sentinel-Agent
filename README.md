# Fo-Sentinel-Agent

**安全事件智能研判多智能体协同平台** — 通过多 Agent AI 架构自动化安全事件监控、分析与响应

![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)
![GoFrame](https://img.shields.io/badge/GoFrame-v2.7.1-blue?style=flat)
![Eino](https://img.shields.io/badge/Eino-v0.3.0+-purple?style=flat)
![React](https://img.shields.io/badge/React-18-61DAFB?style=flat&logo=react)
![License](https://img.shields.io/badge/License-MIT-green?style=flat)

---

## 目录

- [项目简介](#项目简介)
- [系统架构](#系统架构)
- [核心功能模块](#核心功能模块)
    - [1. 多源情报订阅与自动抓取](#1-多源情报订阅与自动抓取)
    - [2. 多 Agent 协同分析引擎](#2-多-agent-协同分析引擎)
    - [3. 深度思考模式（Plan Agent Supervisor-Worker）](#3-深度思考模式plan-agent-supervisor-worker)
    - [4. 联网威胁情报（Intelligence Agent）](#4-联网威胁情报intelligence-agent)
    - [5. RAG 增强检索管道](#5-rag-增强检索管道)
    - [6. 知识库管理](#6-知识库管理)
    - [7. 全链路可观测性（Trace）](#7-全链路可观测性trace)
    - [8. RAG 质量评估](#8-rag-质量评估)
    - [9. 分层记忆管理](#9-分层记忆管理)
    - [10. 结构化报告生成](#10-结构化报告生成)
    - [11. SSE 流式输出](#11-sse-流式输出)
- [技术栈](#技术栈)
- [快速开始](#快速开始)
- [配置参考](#配置参考)
- [API 文档](#api-文档)
- [多智能体系统详解](#多智能体系统详解)
- [项目结构](#项目结构)
- [开发指南](#开发指南)
- [部署](#部署)
- [数据模型](#数据模型)
- [许可证](#许可证)

---

## 项目简介

Fo-Sentinel-Agent 是一套面向安全运营团队的智能哨兵系统。系统从多种来源（漏洞情报、威胁情报源、厂商公告、GitHub 安全公告）自动收集安全事件，通过 **8 个专业 AI Agent** 协同完成事件去重、关联分析、风险评估与报告生成，最终输出可操作的安全处置建议。

它不是一个调通 API 就收工的 Demo，而是覆盖了安全运营平台从情报入库、事件研判、风险评估、报告生成到全链路追踪的完整工程实现。你在企业里做安全 AI 会遇到的问题——多轮对话记忆、RAG 检索质量、多 Agent 编排、深度思考推理、可观测性——这里都有对应的解决方案。

---

## 系统架构

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                              用户 / 外部系统                                       │
│                    Web UI (React 18)      │      REST API                         │
└─────────────────────────────────┬────────────────────────────────────────────────┘
                                  │
                                  ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│                        API 路由层  (GoFrame v2 · :8000)                           │
│  /api/chat/v1/*   /api/event/v1/*   /api/report/v1/*                             │
│  /api/subscription/v1/*   /api/settings/v1/*   /api/auth/v1/*                   │
│  /api/knowledge/v1/*   /api/trace/v1/*   /api/rageval/v1/*                      │
│                                                                                  │
│  Controller 层：参数校验 · JWT 认证 · SessionId 注入 · SSE 响应头设置              │
└─────────────────────────────────┬────────────────────────────────────────────────┘
                                  │
                         ┌────────┴────────┐
                         │  deep_thinking? │
                         └────────┬────────┘
              ┌───── false ───────┤───── true ──────┐
              ▼                                      ▼
┌─────────────────────────────┐   ┌─────────────────────────────────────────────┐
│     标准意图路由层            │   │         深度思考 — Plan Agent                │
│ Router (DeepSeek V3 Quick)  │   │       (Supervisor-Worker 架构)               │
│          │                  │   │                                              │
│ 6 类意图：                   │   │ Planner (Think 模型) → 步骤规划              │
│ chat / event / report /     │   │ Executor (Quick 模型) → Worker 调用          │
│ risk / solve / intel        │   │   ├─ event_analysis_agent (Worker)           │
│          │                  │   │   ├─ report_agent        (Worker)            │
│ 容错降级 → Chat Agent        │   │   ├─ risk_assessment_agent(Worker)           │
│                             │   │   ├─ solve_agent         (Worker)            │
│                             │   │   └─ intel_agent         (Worker)            │
│                             │   │ Replanner → 继续/终止决策                     │
└─────────────────────────────┘   └─────────────────────────────────────────────┘
              │                                      │
              └────────────────┬─────────────────────┘
                               ▼
┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐
│  Chat    │ │  Event   │ │  Report  │ │  Risk    │ │  Plan    │ │  Solve   │ │  Intel   │
│  Agent   │ │  Agent   │ │  Agent   │ │  Agent   │ │  Agent   │ │  Agent   │ │  Agent   │
│ ReAct    │ │ ReAct    │ │ ReAct    │ │ ReAct    │ │Supervisor│ │ ReAct    │ │ ReAct    │
│ +RAG     │ │ +RAG     │ │ +RAG     │ │ +RAG     │ │ Worker   │ │ +RAG     │ │ +Search  │
│ max 25步 │ │ max 15步 │ │ max 15步 │ │ max 15步 │ │ max 20轮 │ │ max 10步 │ │ max 12步 │
└────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘ └──────────┘ └────┬─────┘ └────┬─────┘
     │             │            │             │                          │             │
     └─────────────┴────────────┴─────────────┴──────────────────────────┴─────────────┘
                                              │
                             RAG 检索管道（Eino Graph）
               InputToRag → Rewrite → Split → Embedder → Retriever → Rerank → Template
                                            (DashScope)  (Milvus)  (Qwen3)
                                              │
                                              ▼
┌──────────────────────────────────────────────────────────────────────────────────┐
│                           工具调用层  (13 个工具)                                   │
│                                                                                  │
│  ┌─── event/ ─────────────────────┐  ┌─── report/ ───────────────────────────┐  │
│  │  query_events                  │  │  query_reports                        │  │
│  │  search_similar_events         │  │  query_report_templates               │  │
│  │  query_subscriptions           │  │  create_report                        │  │
│  └────────────────────────────────┘  └───────────────────────────────────────┘  │
│  ┌─── intelligence/ ──────────────┐  ┌─── system/ ───────────────────────────┐  │
│  │  web_search   (Tavily API)     │  │  get_current_time                     │  │
│  │  save_intelligence             │  │  query_database                       │  │
│  └────────────────────────────────┘  │  query_internal_docs                  │  │
│                                      └───────────────────────────────────────┘  │
└───────────────────────┬──────────────────────────┬───────────────────────────────┘
                        │                          │ 后台调度器（异步）
            ┌───────────┼───────────┐              │
            ▼           ▼           ▼              ▼
       ┌─────────┐ ┌─────────┐ ┌─────────┐  ┌────────────────────────────────────┐
       │  MySQL  │ │  Redis  │ │ Milvus  │  │  Scheduler (goroutine)             │
       │─────────│ │─────────│ │─────────│  │  Fetcher: RSS/GitHub 抓取          │
       │ events  │ │语义缓存  │ │事件向量  │  │  每 15 分钟 → 写入 MySQL            │
       │ reports │ │对话历史  │ │文档向量  │  │  Indexer: 向量嵌入 → 写入 Milvus   │
       │ subscr. │ │会话摘要  │ │知识分块  │  │  每 20 分钟                        │
       │ trace   │ │         │ │相似检索  │  └────────────────────────────────────┘
       │ kb/docs │ │         │ │         │
       └─────────┘ └─────────┘ └─────────┘
```

---

## 核心功能模块

### 1. 多源情报订阅与自动抓取

安全运营的第一个问题是"情报从哪来"。手动收集漏洞公告、威胁情报、GitHub 安全公告既耗时又容易遗漏。Fo-Sentinel-Agent 的订阅系统解决的就是这个问题：

- **多协议支持**：RSS 订阅（NVD、CNVD、各厂商安全公告）和 GitHub Security Advisories 两种抓取方式，覆盖主流安全情报来源
- **自动定时抓取**：后台 goroutine 调度器每 15 分钟自动执行抓取，结果写入 MySQL，每 20 分钟触发向量索引，全程无需人工干预
- **智能去重**：基于 `SHA256(title|source|content[:500])` 生成 `dedup_key`，同一事件重复抓取时自动跳过，保证数据干净
- **异步向量化**：新事件入库后由 Indexer 协程批量向量化，写入 Milvus，供 RAG 检索使用，不阻塞主流程
- **抓取日志可查**：每次抓取结果（成功/失败、新增数量、耗时）记录在 `fetch_logs` 表，Web 界面可实时查看，方便排查订阅源异常
- **手动触发**：Web 界面支持单独触发某个订阅源立即抓取，无需等待定时调度

---

### 2. 多 Agent 协同分析引擎

安全事件研判不是一个问题能说清楚的：它需要关联历史事件、评估风险等级、生成处置方案、输出分析报告——这些是不同职责的专业能力。Fo-Sentinel-Agent 用 8 个专业 Agent 分工协作来解决这个问题。

**6 类意图 + 自动路由**

用户发送一条消息，系统首先由 Router（DeepSeek V3 Quick）快速识别意图，再将任务分发给对应 Agent：

| 意图 | Agent | 职责说明 |
|------|-------|----------|
| `chat` | Chat Agent | 通用安全咨询、知识问答、订阅管理查询 |
| `event` | Event Analysis Agent | 事件关联分析、威胁研判、攻击溯源 |
| `report` | Report Agent | 生成周报/月报/自定义安全分析报告 |
| `risk` | Risk Agent | CVE/CVSS 风险评分、攻击路径分析 |
| `solve` | Solve Agent | 单条事件应急响应方案、修复步骤建议 |
| `intel` | Intelligence Agent | 联网检索最新威胁情报、CVE 详情 |

**容错降级设计**

Router 识别失败、置信度不足（低于 0.70）或 SubAgent 执行出错时，系统自动降级到 Chat Agent 兜底，保证用户始终能收到响应，不会出现空白回复。

**Agent 工厂模式（单例）**

所有 Agent 通过 `agent.NewSingletonAgent(AgentConfig{...})` 统一注册为单例，使用 `sync.Once` 保证线程安全的懒初始化。工具实例同样全局单例，通过 `tools.GetMany(names)` 按名取用，无状态设计并发安全。

---

### 3. 深度思考模式（Plan Agent Supervisor-Worker）

标准路由模式适合单一意图的问题，但安全场景经常遇到复杂的多步骤任务："分析最近一周的高危事件、评估风险、生成处置报告"——这需要多个 Agent 协作、有序执行。深度思考模式就是为此设计的。

**架构：Plan-Execute-Replan 三段式循环**

```
用户请求（deep_thinking=true）
        ↓
   Plan Agent（Supervisor）
        ↓
Planner（DeepSeek V3 Think 深度推理）→ 生成步骤清单
        ↓
Executor 循环（每步调用对应 Worker 工具）
   ├─ intel_agent      → Intelligence Agent（联网情报采集）
   ├─ event_analysis   → Event Analysis Agent（含完整 RAG）
   ├─ risk_assessment  → Risk Agent（含完整 RAG）
   ├─ report_agent     → Report Agent（含完整 RAG）
   └─ solve_agent      → Solve Agent（含完整 RAG）
        ↓
Replanner → 评估当前结果，决策：继续下一步 or 终止输出最终答案
```

**关键工程细节**

- **Worker 工具隔离**：每个 Worker 通过 `isolateCtx` 派生全新 context，彻底隔离外层 Executor 的 Eino compose state，避免内部 Agent Graph 的 state key 冲突导致 panic
- **上下文传递**：通过进程内 `SessionMemory` 单例取最近 3 条历史消息注入 `enrichedQuery`，零 Redis 开销，Worker 能感知当前对话背景
- **输出截断防溢出**：Worker 返回值超过 2000 rune 时自动截断，防止 Executor 上下文爆炸影响后续步骤推理质量
- **最大 20 轮**：Replanner 最多允许循环 20 轮，防止无限规划

---

### 4. 联网威胁情报（Intelligence Agent）

传统 RAG 只能检索已有知识，面对新出现的 CVE、零日漏洞、最新攻击组织动态时无能为力。Intelligence Agent 通过集成联网搜索工具解决了这个问题。

- **Tavily API 集成**：`web_search` 工具接入 Tavily Search（专为 AI Agent 设计的搜索 API），支持 `advanced` 深度搜索模式，结果质量高于通用搜索
- **情报沉淀**：`save_intelligence` 工具将 Agent 分析总结的情报自动写入 MySQL `events` 表，同时触发异步 Milvus 向量化，下次检索可直接命中
- **联网开关**：前端聊天界面提供联网搜索开关，通过 context 中的 `WebSearchEnabledKey` 控制，关闭时工具返回提示信息而不执行实际搜索，避免不必要的 API 消耗
- **职责单一**：Intelligence Agent 专注"联网采集 → 分析 → 沉淀"三段式情报流程，不与本地事件工具混用，避免角色混淆

---

### 5. RAG 增强检索管道

"调个 Embedding API，往向量库塞点数据"只是跑通了 Demo。Fo-Sentinel-Agent 的 RAG 管道覆盖了影响检索质量的每个关键环节：

**① 查询重写（Query Rewrite）**

多轮对话中用户说"分析这个事件"，"这个"指什么？单靠原始问题检索，Milvus 根本不知道你在问什么。查询重写模块（`internal/ai/rewrite/`）在检索前调用 LLM 消除代词歧义、补全上下文：

- Event / Report / Risk pipeline 默认启用，补全多轮对话中的省略信息
- Solve pipeline 不启用（单事件明确查询，无需重写，节省约 200ms 延迟）
- 可通过 `retriever.rewrite_enabled` 配置全局开关

**② 子问题拆分（Sub-Question Splitting）**

复杂查询"分析最近一周的 Log4j 相关事件并评估风险"可以拆成多个独立子问题并行检索，提升多维度召回率。拆分模块（`internal/ai/split/`）将复杂查询分解为子查询，并发执行后合并去重结果，增加约 300ms 延迟但显著提升召回完整性。

**③ 语义缓存（Semantic Cache）**

相同语义的问题不应每次都打 Milvus。语义缓存（`internal/ai/cache/semantic.go`）对检索结果做缓存：

- 余弦相似度阈值 0.85 命中时直接返回，跳过 Embedding + Milvus 调用
- 缓存 TTL 24 小时，缓存键基于查询向量
- 命中状态记录在 Trace Node 的 `cache_hit` 字段，可通过追踪界面验证缓存效果

**④ Rerank 精排（Qwen3-Rerank）**

Milvus 向量检索召回 5 个候选文档，Rerank 模型（`qwen3-rerank`）对候选文档重新打分排序，取 Top-3 送入 LLM，在召回率和精准度之间取得更好平衡。Rerank 可通过 `retriever.rerank.enabled` 开关控制。

**⑤ 父子分块（Hierarchical Chunking）**

检索时用小块（子块 256 rune）保证语义精准，生成时附上父块（1024 rune）提供足够上下文，兼顾检索精度与 LLM 回答质量。分块策略支持三种：

- `sliding_window`：滑动窗口分块（512 字符 + 128 重叠），通用场景，计算开销最低
- `hierarchical`：层级分块（父块 1024 + 子块 256），文档文件默认策略，检索子块但送入 LLM 时附上父块和章节标题
- `code`：代码语法感知分块，按函数/类边界切分，保留函数名作为章节标题，适合 Go/Python/Java/JS/TS 代码文件

---

### 6. 知识库管理

安全团队积累的内部知识（安全规范、应急预案、历史分析报告、代码库）需要系统化管理，才能在 RAG 检索时精准命中。

- **多知识库**：支持创建多个知识库（MySQL `knowledge_bases` 表），通过 Milvus 的 `base_id` 元数据字段区分，逻辑隔离
- **文档全生命周期管理**：上传 → 解析 → 分块 → 向量化 → 写入 Milvus，状态流转 `pending → indexing → completed / failed`，支持重建索引
- **智能策略选择**：基于文件扩展名自动选择分块策略
  - 代码文件（`.go/.py/.java`）→ `code` 策略（语法感知，按函数/类边界切分）
  - 文档文件（`.pdf/.md/.docx`）→ `hierarchical` 策略（结构化父子分块，提取章节标题）
- **多格式解析器**：
  - PDF：三阶段 fallback（直接提取 → pdfcpu 修复 → 解密重试），处理损坏/加密 PDF
  - DOCX：识别 Heading 样式提取章节结构，保留原始文件名作为标题
  - Markdown：按 `#` 标题层级切分，构建 "H1 > H2 > H3" 层级路径
  - Go/Python/Java代码文件：直接读取 UTF-8 文本，由 `chunkers.Code()` 按语法边界分块
- **分块可视化**：Web 界面可查看每个文档的分块列表，包含章节标题（文档为章节路径，代码为函数名）、字符数、相似度匹配分数

---

### 7. 全链路可观测性（Trace）

AI 系统出了问题，最难的是定位：是 Embedding 慢了？Milvus 召回结果差？还是 LLM 幻觉？全链路追踪系统让每个环节的耗时、成本、检索质量都有记录可查。

**两级数据结构**

- `agent_trace_runs`：每次请求对应一条记录，记录总耗时、Token 消耗、估算成本、会话 ID
- `agent_trace_nodes`：每个处理节点（LLM 调用、向量检索、缓存读写、数据库查询）对应一条记录，含父子节点关系、深度、错误信息

**节点类型埋点**

| 节点类型 | 埋点方式 | 记录内容 |
|----------|----------|----------|
| **LLM** | Eino callbacks 自动 | 模型名、输入/输出 Token、成本估算（CNY） |
| **TOOL** | Eino callbacks 自动 | 工具名、输入参数 JSON、输出结果 |
| **RETRIEVER** | Eino callbacks 自动 | 查询文本、召回文档数、平均/最高相似度分、Rerank 状态 |
| **EMBEDDING** | 手动 span 包装 | 模型名、估算 Token（1.5字符/token）、成本估算（CNY） |
| **RERANK** | 手动 span 包装 | 模型名、输入 Token、成本估算（CNY） |
| **AGENT** | 手动 span 包装 | Agent 名称、执行耗时（Worker 工具内包裹 SubAgent） |
| **CACHE** | 手动 span 包装 | 缓存命中状态、Redis key（语义缓存、会话记忆） |
| **DB** | GORM Plugin 自动 | SQL 操作类型、表名、耗时、影响行数（慢查询 >100ms） |

**检索质量指标**

RETRIEVER 节点记录 `avg_vector_score`、`max_vector_score`、`doc_count`、`rerank_used`、`avg_rerank_score`，通过追踪界面可以量化每次检索的质量，为调参（TopK、相似度阈值）提供数据支撑。

**成本估算**

- **LLM 成本**：根据 Token 消耗和配置的模型单价（`trace.model_pricing`）计算
- **Embedding 成本**：查询时按字符数估算（1.5字符/token），索引时累计文档字符数计算
- **Rerank 成本**：根据 API 返回的实际 Token 消耗计算

**慢查询追踪**

通过 GORM Plugin 自动拦截数据库操作，只记录超过阈值（默认 100ms）的慢查询。防递归设计跳过 `agent_trace_*` 表自身的查询，避免追踪系统写库被追踪。

---

### 8. RAG 质量评估

追踪系统告诉你每次请求发生了什么，RAG 质量评估模块告诉你系统整体表现如何。

**KPI 仪表盘**

从 `agent_trace_runs/nodes` 聚合关键指标：

| 指标 | 说明 | 阈值判断 |
|------|------|----------|
| 成功率 | status='success' 的请求占比 | >= 0.99 为 good，>= 0.95 为 warning |
| 平均延迟 | 平均响应耗时（毫秒） | 反映用户体验 |
| P95 延迟 | 95 分位延迟（毫秒） | <= 5s 为 good，<= 15s 为 warning |
| 平均召回文档数 | RETRIEVER 节点平均召回文档数 | 反映检索覆盖度 |
| 平均最高相似度分 | RETRIEVER 节点平均最高相似度分 | 反映检索精准度 |
| 缓存命中率 | CACHE 节点命中次数 / 总次数 | 反映缓存效率 |
| 平均 Rerank 分数 | Rerank 节点平均相关性分数 | 反映精排质量 |

支持 3 种时间窗口：24h（按小时分组）、7d（按天分组）、30d（按天分组）

**模型成本分布**

按模型名称聚合成本占比，支持：
- **LLM 模型**：DeepSeek V3（主推理）、DeepSeek V3 Quick（路由/快速）
- **Embedding 模型**：text-embedding-v4（查询 + 索引）
- **Rerank 模型**：qwen3-rerank（精排）

每个模型显示：输入/输出 Token、总成本（CNY）、成本占比、请求次数

**用户反馈**

聊天界面每条 AI 回复底部支持点赞/点踩，反馈写入 `message_feedbacks` 表，可统计满意度趋势。反馈按 `(session_id, message_index)` 唯一标识一条回复，重复提交时更新而非插入新记录。

**链路关联**

质量评估页面可按 session_id 筛选具体链路，点击查看 Trace 详情，定位低质量回答的根因。链路列表支持按状态过滤（success/error/running），显示用户反馈状态（点赞/踩图标）。

**趋势图**

基于 ECharts 的响应耗时和检索得分趋势图，帮助识别性能退化和质量波动。

**趋势图**

基于 ECharts 的响应耗时和检索得分趋势图，帮助识别性能退化。

---

### 9. 分层记忆管理

把所有对话历史全塞给 LLM 是行不通的——20 轮对话就能把 Token 用完，成本飙升，模型也容易"迷失"在历史信息里。Fo-Sentinel-Agent 的记忆管理解决的是"如何在控制 Token 成本的前提下保留关键上下文"。

**两层记忆结构**

- **短期记忆（Short-term）**：保留最近 N 条消息原文，用于当前对话推理
- **长期记忆（Long-term）**：超过阈值的历史消息由 Summary Agent 压缩为摘要，持久化到 Redis

**触发机制（双重触发器）**

- 消息数触发：超过 30 条消息触发摘要
- Token 触发：累计 Token 超过 3000 触发摘要
- 摘要后保留最近 4 条消息保证对话连贯性

**Summary Agent 流水线**

Summary Agent 是 4 节点线性流水线（无 RAG，专注压缩），使用独立的 LLM 调用将历史消息摘要为简洁的上下文，摘要结果注入后续对话的系统 Prompt。

**跨会话持久化**

对话历史以 `session:${sessionId}` 为 key 存储在 Redis，TTL 30 天，服务重启后用户可继续历史对话，不丢上下文。

---

### 10. 结构化报告生成

安全团队每周/每月需要输出安全报告，手工整理数据、写分析、排版耗时巨大。Report Agent 自动完成这个过程。

- **多类型报告**：周报（weekly）/ 月报（monthly）/ 自定义（custom）三种模板，对应不同的时间范围和分析维度
- **Report Agent 工具链**：`query_events`（获取事件数据）→ `query_report_templates`（获取模板）→ ReAct 推理生成内容 → `create_report`（持久化到 MySQL）
- **历史报告检索**：`query_reports` 工具支持按时间范围、类型过滤查询历史报告，Report Agent 可参考历史报告的写法和结构
- **RAG 增强**：报告生成时同样通过 Milvus 检索相关历史事件和内部文档，保证报告内容有据可查

---

### 11. SSE 流式输出

AI 分析可能需要数十秒才能完成，如果等分析完再一次性返回，用户体验极差。系统所有 AI 分析过程都通过 Server-Sent Events 实时推流，用户可以看到推理步骤和工具调用过程。

**SSE 事件类型协议**

```
# 请求开始推送会话元数据
data: {"type":"meta","content":"{\"sessionId\":\"...\",\"deepThinking\":false}"}

# Agent 路由/状态通知
data: {"type":"status","content":"[Event Agent 分析中...]"}

# 标准模式内容流（按意图分类）
data: {"type":"chat","content":"文本块"}
data: {"type":"event","content":"分析结果块"}
data: {"type":"report","content":"报告内容块"}
data: {"type":"risk","content":"风险评估块"}
data: {"type":"solve","content":"处置方案块"}
data: {"type":"intel","content":"情报分析块"}

# 深度思考模式（Plan Agent）
data: {"type":"plan_step","content":"规划执行中间步骤"}
data: {"type":"plan","content":"最终答案块"}

# 异常 / 结束
data: {"type":"error","content":"错误信息"}
data: [DONE]
```

---

---

## 技术栈

| 层次 | 技术 | 说明 |
|------|------|------|
| 后端框架 | GoFrame v2.7.1 | HTTP 服务器、配置管理、日志 |
| AI 编排 | Cloudwego Eino | 多 Agent 管道编排、ReAct、Graph、Plan-Execute-Replan |
| 对话模型 | DeepSeek V3 | 主要推理与分析（OpenAI 兼容接口） |
| 嵌入模型 | DashScope text-embedding-v4 | 向量嵌入 |
| Rerank 模型 | DashScope qwen3-rerank | 检索结果重排序精排 |
| 联网搜索 | Tavily Search API | 专为 AI Agent 设计的搜索接口 |
| 关系数据库 | MySQL 8.0+ | 事件、订阅、报告、用户、知识库、追踪持久化 |
| 向量数据库 | Milvus 2.x | 事件/文档向量检索（RAG） |
| 缓存 | Redis 7.x | 语义缓存 + 对话历史 + 会话摘要 |
| 前端 | React 18 + TypeScript + Vite | 现代化 Web 界面 |
| 状态管理 | Zustand | 前端全局状态 |
| 样式 | TailwindCSS | 响应式 UI |
| 图表 | ECharts | 趋势图、分布图 |

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
ds_think_chat_model:
  api_key: "your-deepseek-api-key"      # DeepSeek V3 API Key（主推理/深度思考）
ds_quick_chat_model:
  api_key: "your-deepseek-api-key"      # DeepSeek V3 API Key（意图识别/快速）

doubao_embedding_model:
  api_key: "your-dashscope-api-key"     # DashScope API Key（嵌入 + Rerank）

# 数据库
database:
  mysql:
    dsn: "root:password@tcp(127.0.0.1:3307)/fo_sentinel?charset=utf8mb4&parseTime=True"

# Redis
redis:
  addr: "127.0.0.1:6379"

# 联网搜索（可选，Intelligence Agent 需要）
tools:
  web_search:
    tavily_api_key: "tvly-xxxx"
```

### 4. 启动后端

```bash
go mod tidy
go run main.go
# 服务运行在 http://localhost:8000
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

| 配置项 | 类型 | 说明 |
|--------|------|------|
| `ds_think_chat_model.api_key` | string | DeepSeek V3 API Key（主推理、深度思考） |
| `ds_think_chat_model.model` | string | 模型名称，默认 `deepseek-v3-1-terminus` |
| `ds_quick_chat_model.api_key` | string | DeepSeek V3 API Key（意图识别，低延迟） |
| `doubao_embedding_model.api_key` | string | DashScope Embedding + Rerank API Key |

### 存储

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `database.mysql.dsn` | MySQL 连接字符串 | `root:sentinel123@tcp(127.0.0.1:3307)/fo_sentinel` |
| `redis.addr` | Redis 地址 | `localhost:6379` |
| `redis.semantic_cache.ttl` | 语义缓存 TTL（小时） | `24` |
| `redis.semantic_cache.threshold` | 语义缓存相似度阈值 | `0.85` |
| `redis.semantic_cache.topk` | Milvus 初始召回数 | `5` |
| `redis.semantic_cache.final_topk` | 送入 LLM 的最终文档数 | `3` |

### 检索增强

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `retriever.rewrite_enabled` | 启用查询重写 | `true` |
| `retriever.split_enabled` | 启用子问题拆分 | `true` |
| `retriever.confidence_threshold` | 意图识别置信度阈值 | `0.70` |
| `retriever.rerank.enabled` | 启用 Rerank 精排 | `true` |
| `retriever.rerank.model` | Rerank 模型名 | `qwen3-rerank` |
| `retriever.chat_cache.ttl` | 对话历史缓存 TTL（小时） | `720`（30天） |

### 认证

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `auth.jwt.enabled` | 是否启用 JWT 认证 | `false` |
| `auth.jwt.expire_hours` | Token 有效期（小时） | `24` |
| `auth.jwt.secret` | JWT 签名密钥 | - |
| `auth.seed.admin_password` | 初始管理员密码 | `admin123` |

### 调度器

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `scheduler.fetch_interval_minutes` | 事件抓取间隔（分钟） | `15` |
| `scheduler.index_interval_minutes` | 向量索引间隔（分钟） | `20` |
| `scheduler.index_batch_size` | 向量索引单批文档数 | `10` |

### 记忆管理

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `memory.summaryTrigger` | 消息数触发摘要阈值 | `30` |
| `memory.summaryBatchSize` | 每次摘要的消息条数 | `10` |
| `memory.tokenTrigger` | Token 触发阈值（主触发器） | `3000` |
| `memory.minRecentMessages` | 摘要后最少保留消息数 | `4` |

### 知识库

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `knowledge.upload_dir` | 上传文件保存根目录 | `manifest/upload/knowledge` |
| `knowledge.index_workers` | Worker Pool 并发数 | `3` |
| `knowledge.parent_chunk_size` | 父块大小（rune） | `1024` |
| `knowledge.child_chunk_size` | 子块大小（rune） | `256` |
| `knowledge.child_overlap_size` | 子块重叠（rune） | `40` |

### 追踪

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `trace.enabled` | 启用全链路追踪 | `true` |
| `trace.record_prompt` | 记录 Prompt/Completion 原文 | `false` |
| `trace.record_sql` | 记录 SQL 语句 | `false` |

---

## API 文档

> 所有接口前缀 `/api`，启用 JWT 时需在请求头添加 `Authorization: Bearer <token>`

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/v1/login` | 用户登录，返回 JWT Token |

### 聊天

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/chat/v1/chat` | 普通对话（非流式） |
| POST | `/api/chat/v1/chat_stream` | 对话流式输出（SSE） |
| POST | `/api/chat/v1/intent_recognition` | 意图驱动多 Agent 对话（SSE） |
| POST | `/api/chat/v1/upload` | 上传知识文档（PDF/TXT/MD） |

### 事件

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/event/v1/list` | 事件列表（分页、过滤） |
| POST | `/api/event/v1/create` | 手动创建事件 |
| GET | `/api/event/v1/stats` | 事件统计数据 |
| GET | `/api/event/v1/trend` | 事件趋势图数据 |
| POST | `/api/event/v1/update_status` | 更新事件状态 |
| POST | `/api/event/v1/delete` | 删除事件 |
| POST | `/api/event/v1/analyze/stream` | 单条事件 AI 分析（SSE） |
| POST | `/api/event/v1/pipeline/stream` | 事件分析管道（SSE） |

### 知识库

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/knowledge/v1/list` | 知识库列表 |
| POST | `/api/knowledge/v1/create` | 创建知识库 |
| POST | `/api/knowledge/v1/delete` | 删除知识库 |
| GET | `/api/knowledge/v1/docs` | 文档列表 |
| POST | `/api/knowledge/v1/doc/upload` | 上传文档 |
| POST | `/api/knowledge/v1/doc/delete` | 删除文档 |
| POST | `/api/knowledge/v1/doc/rebuild` | 重建文档索引 |
| GET | `/api/knowledge/v1/chunks` | 文档分块列表 |
| POST | `/api/knowledge/v1/search` | 语义搜索（返回相似度分数） |

### 报告

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/report/v1/list` | 报告列表 |
| POST | `/api/report/v1/create` | 生成安全报告 |
| GET | `/api/report/v1/get` | 获取报告详情 |
| POST | `/api/report/v1/delete` | 删除报告 |
| GET | `/api/report/v1/template/list` | 获取报告模板列表 |

### 订阅管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/subscription/v1/list` | 订阅列表 |
| POST | `/api/subscription/v1/create` | 添加订阅源 |
| POST | `/api/subscription/v1/update` | 更新订阅 |
| POST | `/api/subscription/v1/delete` | 删除订阅 |
| POST | `/api/subscription/v1/pause` | 暂停订阅 |
| POST | `/api/subscription/v1/resume` | 恢复订阅 |
| GET | `/api/subscription/v1/logs` | 抓取日志 |
| POST | `/api/subscription/v1/fetch` | 手动触发抓取 |

### 追踪

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/trace/v1/list` | 追踪记录列表 |
| GET | `/api/trace/v1/detail` | 追踪详情（含节点树） |
| GET | `/api/trace/v1/stats` | 追踪统计（KPI） |

### RAG 质量评估

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/rageval/v1/dashboard` | KPI 仪表盘数据 |
| GET | `/api/rageval/v1/traces` | 链路列表（含检索质量指标） |
| POST | `/api/rageval/v1/feedback` | 提交用户反馈（点赞/踩） |
| GET | `/api/rageval/v1/feedback_stats` | 满意度统计 |

### 系统设置

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/settings/v1/general` | 获取全局配置 |
| POST | `/api/settings/v1/general` | 更新全局配置 |

---

## 多智能体系统详解

### Agent 一览

| Agent | 职责 | 架构 | 最大步数 | 核心工具 |
|-------|------|------|---------|------|
| **Chat Agent** | 通用对话、安全咨询、知识库问答 | 自定义 DAG + RAG + 分层记忆 | 25 | query_subscriptions, query_internal_docs, get_current_time |
| **Event Analysis Agent** | 安全事件关联分析、威胁研判、攻击溯源 | ReAct + RAG（重写+拆分+Rerank） | 15 | query_events, search_similar_events, query_reports, query_internal_docs |
| **Report Agent** | 生成结构化安全报告（周报/月报/自定义） | ReAct + RAG（重写+拆分+Rerank） | 15 | query_events, query_reports, query_report_templates, create_report |
| **Risk Agent** | CVE/CVSS 风险评分、攻击路径分析、漏洞评估 | ReAct + RAG（重写+拆分+Rerank） | 15 | query_events, search_similar_events, query_internal_docs, query_subscriptions |
| **Plan Agent** | 深度思考模式 Supervisor：规划多步任务并委托 Worker | Plan-Execute-Replan 循环 | 20 轮 | intel_agent, event_analysis_agent, risk_assessment_agent, report_agent, solve_agent (均为 Worker) |
| **Solve Agent** | 单条安全事件应急响应方案、修复步骤 | ReAct + RAG | 10 | search_similar_events, query_internal_docs |
| **Intelligence Agent** | 联网威胁情报检索：CVE 详情、漏洞公告、威胁组织、恶意 IP | ReAct + 联网搜索 + RAG | 12 | web_search, save_intelligence, query_internal_docs, get_current_time |
| **Summary Agent** | 对话历史摘要压缩（长期记忆维护） | 4 节点线性流水线 | — | 无（触发阈值：30 条消息 或 3000 Token） |

### 意图路由流程

```
标准模式（deep_thinking=false）
   │
   ▼
Router (DeepSeek V3 Quick) 识别意图（6 类）
   │
   ├── chat   → Chat Agent（通用对话）
   ├── event  → Event Analysis Agent（事件分析）
   ├── report → Report Agent（报告生成）
   ├── risk   → Risk Agent（风险评估）
   ├── solve  → Solve Agent（应急响应）
   └── intel  → Intelligence Agent（联网情报）
   │
   容错降级：Router 失败 / 置信度 < 0.70 / SubAgent 错误 → Chat Agent

深度思考模式（deep_thinking=true）
   │
   ▼
直接进入 Plan Agent（跳过意图识别路由，节省一次 LLM 调用）
   │
   ▼
Planner（DeepSeek V3 Think 深度推理）→ 生成步骤清单
   │
   ▼
Executor 循环（每步调用对应 Worker 工具）
   ├── intel_agent          → Intelligence Agent（联网情报采集）
   ├── event_analysis_agent → Event Analysis Agent（含完整 RAG pipeline）
   ├── report_agent         → Report Agent（含完整 RAG pipeline）
   ├── risk_assessment_agent→ Risk Agent（含完整 RAG pipeline）
   └── solve_agent          → Solve Agent（含完整 RAG pipeline）
   │
   ▼
Replanner → 评估结果，决策继续或终止，输出最终答案
```

### 工具系统

| 工具名称 | 分类 | 功能说明 | 使用 Agent |
|----------|------|----------|-----------|
| `query_events` | event/ | 按条件过滤查询 MySQL 安全事件表 | Event、Risk、Report、Chat |
| `search_similar_events` | event/ | 基于 Milvus 向量相似度搜索事件 | Event、Risk、Report、Solve |
| `query_subscriptions` | event/ | 查询订阅源配置 | Event、Risk、Chat |
| `query_reports` | report/ | 查询已生成的历史报告 | Report、Risk |
| `query_report_templates` | report/ | 获取报告模板 | Report |
| `create_report` | report/ | 生成并持久化安全报告 | Report |
| `web_search` | intelligence/ | Tavily API 联网搜索最新威胁情报 | Intelligence |
| `save_intelligence` | intelligence/ | 将分析情报写入 MySQL + 触发异步向量化 | Intelligence |
| `get_current_time` | system/ | 获取当前时间（用于时间范围推算） | 所有 Agent |
| `query_database` | system/ | 执行 SELECT 查询（Plan Agent 通用数据查询） | Plan |
| `query_internal_docs` | system/ | 查询 Milvus 内部文档知识库 | Chat、Risk、Solve、Intel、Plan |

---

## 项目结构

```
fo-sentinel-agent/
├── api/                              # API 接口定义（GoFrame 路由绑定）
│   ├── auth/v1/                      # 认证接口
│   ├── chat/v1/                      # 聊天接口
│   ├── event/v1/                     # 事件接口
│   ├── knowledge/v1/                 # 知识库接口
│   ├── rageval/v1/                   # RAG 质量评估接口
│   ├── report/v1/                    # 报告接口
│   ├── subscription/v1/              # 订阅接口
│   ├── term_mapping/v1/              # 术语映射接口
│   ├── trace/v1/                     # 追踪接口
│   └── settings/v1/                  # 设置接口
├── internal/
│   ├── ai/
│   │   ├── agent/                    # 专业 Agent 管道实现
│   │   │   ├── base/                 # 公共 RAG+ReAct DAG 构建器（BuildReactAgentGraph）
│   │   │   ├── chat_pipeline/        # Chat Agent（自定义 DAG）
│   │   │   ├── event_analysis_pipeline/  # Event Analysis Agent
│   │   │   ├── report_pipeline/      # Report Agent
│   │   │   ├── risk_pipeline/        # Risk Agent
│   │   │   ├── solve_pipeline/       # Solve Agent
│   │   │   ├── intelligence_pipeline/# Intelligence Agent（联网情报）
│   │   │   ├── plan_pipeline/        # Plan Agent（Supervisor-Worker）
│   │   │   │   └── agent_worker.go   # 5 个 Worker 工具包装器
│   │   │   ├── knowledge_index_pipeline/ # 知识文档索引管道
│   │   │   ├── summary_pipeline/     # Summary Agent（对话历史压缩）
│   │   │   └── factory.go            # NewSingletonAgent 工厂函数
│   │   ├── intent/                   # 意图路由系统
│   │   │   ├── core/                 # 公共类型、注册表（IntentType、SubAgent 接口）
│   │   │   └── subagents/            # SubAgent 适配器（chat/event/report/risk/solve/intel）
│   │   ├── cache/                    # Redis 缓存（对话历史、语义缓存、会话记忆）
│   │   ├── document/                 # 文档解析与分块（fixed_size/structure_aware/hierarchical）
│   │   ├── embedder/                 # DashScope 向量嵌入（含稀疏嵌入 sparse.go）
│   │   ├── indexer/                  # Milvus 索引器
│   │   ├── models/                   # LLM 模型工厂（OpenAI 兼容接口）
│   │   ├── prompt/                   # 集中管理所有提示词
│   │   │   └── agents/               # 各 Agent 系统提示词（agents.go/plan.go/routing.go/rag.go）
│   │   ├── rerank/                   # Rerank 精排（qwen3-rerank）
│   │   ├── retriever/                # Milvus 检索器（含语义缓存）
│   │   ├── rewrite/                  # 查询重写
│   │   ├── rule/                     # 业务规则（如严重级别映射）
│   │   ├── split/                    # 子问题拆分
│   │   ├── tools/                    # 工具实现（全局注册表 registry.go + init.go）
│   │   │   ├── event/                # query_events, search_similar_events, query_subscriptions
│   │   │   ├── intelligence/         # web_search, save_intelligence
│   │   │   ├── report/               # query_reports, create_report, query_report_templates
│   │   │   └── system/               # get_current_time, query_database, query_internal_docs
│   │   └── trace/                    # 全链路追踪（context/span/callback/store）
│   ├── controller/                   # HTTP 控制器
│   ├── dao/
│   │   ├── milvus/                   # Milvus DAO（向量检索与索引）
│   │   └── mysql/                    # MySQL DAO（model/database/事件/报告/知识库/追踪等）
│   └── service/
│       ├── auth/                     # 认证服务
│       ├── chat/                     # 对话业务逻辑（intent.go 路由、file_index.go 文件索引）
│       ├── event/                    # 事件业务逻辑
│       ├── knowledge/                # 知识库管理（Worker Pool 异步索引）
│       ├── pipeline/                 # 事件分析管道服务
│       ├── rageval/                  # RAG 质量评估聚合
│       ├── report/                   # 报告业务逻辑
│       ├── scheduler/                # 后台调度器（Fetcher + Indexer）
│       ├── settings/                 # 系统设置
│       └── subscription/             # 订阅管理
├── manifest/
│   ├── config/config.yaml            # 主配置文件
│   ├── docker/docker-compose.yml     # 本地开发依赖服务
│   └── upload/knowledge/             # 知识库文档上传目录
├── utility/                          # 公共工具
│   ├── auth/                         # JWT 工具
│   ├── middleware/                   # CORS、JWT、响应包装
│   └── sse/                          # SSE 工具库
├── web/                              # React 18 前端
│   └── src/
│       ├── components/               # 公共组件（Layout、Sidebar、ConfirmDialog 等）
│       ├── pages/
│       │   ├── chat/                 # 聊天界面（WelcomeScreen、ChatInput、SessionList）
│       │   ├── dashboard/            # 工作台
│       │   ├── event-analysis/       # 事件分析（多组件）
│       │   ├── events/               # 事件列表
│       │   ├── knowledge/            # 知识库管理（列表/文档/分块）
│       │   ├── rag-eval/             # RAG 质量评估
│       │   ├── reports/              # 报告管理
│       │   ├── subscriptions/        # 订阅管理
│       │   ├── term-mapping/         # 术语映射
│       │   ├── traces/               # 全链路追踪（列表/详情）
│       │   └── settings/             # 系统设置
│       ├── services/                 # API 客户端
│       └── utils/                    # SSE 解析、工具函数
└── main.go                           # 程序入口（端口 8000）
```

---

## 开发指南

### 添加新 Agent

1. 在 `internal/ai/agent/<name>_pipeline/` 创建目录，实现 `orchestration.go`：

```go
var GetYourAgent = agent.NewSingletonAgent(agent.AgentConfig{
    GraphName:      "YourAgent",
    SystemPrompt:   agents.YourPrompt,    // 在 internal/ai/prompt/agents/ 定义
    ModelFactory:   models.OpenAIForDeepSeekV3Quick,
    MaxStep:        15,
    RewriteEnabled: true,
    SplitEnabled:   true,
    ToolNames:      []string{"query_events", "get_current_time"},
})
```

2. 在 `internal/ai/intent/subagents/` 创建 SubAgent 适配器：

```go
type YourSubAgent struct{}

func (a *YourSubAgent) Name() core.IntentType { return core.IntentType("your_intent") }

func (a *YourSubAgent) Execute(ctx context.Context, task *core.Task, callback core.StreamCallback) (*core.Result, error) {
    // 调用 GetYourAgent()，流式输出通过 callback 推送
}

func init() { core.RegisterSubAgent(&YourSubAgent{}) }
```

3. 在 `internal/ai/intent/core/types.go` 添加意图常量，更新 `internal/ai/prompt/agents/routing.go` 的路由 Prompt。

### 添加新工具

1. 在 `internal/ai/tools/<category>/<tool_name>.go` 创建工具文件：

```go
func NewYourTool() tool.InvokableTool {
    t, err := utils.InferOptionableTool(
        "your_tool",
        "Tool description in English for LLM to understand",
        func(ctx context.Context, input *YourInput, _ ...tool.Option) (string, error) {
            // 实现逻辑
        },
    )
    if err != nil { panic(err) }
    return t
}
```

2. 在 `internal/ai/tools/init.go` 的 `init()` 中注册：

```go
Register("your_tool", NewYourTool())
```

3. 在目标 Agent 的 `orchestration.go` 中将工具名加入 `ToolNames` 列表。

### 添加新的 Plan Worker

在 `internal/ai/agent/plan_pipeline/agent_worker.go` 中参照现有 Worker 实现：

```go
func NewYourAgentWorker() tool.InvokableTool {
    t, err := utils.InferOptionableTool(
        "your_agent",
        "Worker description",
        func(ctx context.Context, input *WorkerInput, _ ...tool.Option) (string, error) {
            isolated := isolateCtx(ctx)
            workerCtx := context.WithValue(isolated, SessionIdCtxKey{}, ctx.Value(SessionIdCtxKey{}))
            enrichedQuery := buildWorkerContext(ctx) + input.Query
            agent := your_pipeline.GetYourAgent()
            stream, err := agent.Stream(workerCtx, []*schema.Message{
                schema.UserMessage(enrichedQuery),
            })
            if err != nil { return "", err }
            return workerStream(stream)
        },
    )
    if err != nil { panic(err) }
    return t
}
```

然后在 `executor.go` 中注册到 Executor 工具列表。

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
- [ ] 配置 `trace.record_prompt: false`（减少 90% 追踪写入量）
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
| `knowledge_bases` | id, name, description, doc_count, chunk_count | 知识库（文档逻辑分组） |
| `knowledge_documents` | id, base_id, name, file_type, chunk_strategy, index_status, indexed_at | 知识文档 |
| `knowledge_chunks` | id, doc_id, chunk_index, content_preview, section_title, char_count | 文档分块元数据 |
| `query_term_mappings` | id, source_term, target_term, priority, enabled | RAG 查询术语归一化 |
| `agent_trace_runs` | trace_id, session_id, query_text, status, duration_ms, total_tokens, estimated_cost_usd | 链路运行记录（每请求一条） |
| `agent_trace_nodes` | node_id, trace_id, parent_node_id, node_type, node_name, duration_ms, model_name, avg_vector_score | 链路节点记录（每节点一条） |
| `message_feedbacks` | session_id, message_index, vote, reason | 用户对 AI 回复的点赞/踩反馈 |

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
