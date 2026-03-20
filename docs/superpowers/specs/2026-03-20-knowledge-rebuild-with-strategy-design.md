# 知识库重建索引时切换分块策略 — 设计文档

**日期**：2026-03-20
**状态**：已批准

---

## 背景

RAG 检索测试（`/knowledge/v1/search`）可能暴露分块效果不理想的问题。现有重建索引接口（`DocRebuild` / `DocBatchRebuild`）已在 API 结构体中预留了 `chunk_strategy` 字段，但 Controller 未传入服务层，导致重建时无法切换策略。本设计补齐这一能力。

---

## 目标

- 单文档重建时，可在确认弹窗内选择新的分块策略（或保持原策略）
- 批量重建时，可在确认弹窗内统一指定分块策略（可选，空=各自保持原策略）
- 后端完整透传策略参数，服务层已有的 `newStrategy` 逻辑正式生效

---

## 不在范围内

- 分块参数细调（ChunkSize、OverlapSize 等），本次只选策略
- 上传流程改动（上传时策略自动选择逻辑保持不变）
- 新增分块策略类型

---

## 后端改动

### 1. Controller：`DocRebuild`（`internal/controller/knowledge/knowledge.go`）

**现状**：`DocRebuild` 调用 `knowledgesvc.RebuildDoc(ctx, req.ID)`，忽略了 `req.ChunkStrategy`。

**改动**：传入策略字段。

```go
func (c *controllerV1) DocRebuild(ctx context.Context, req *v1.DocRebuildReq) (res *v1.DocRebuildRes, err error) {
    return &v1.DocRebuildRes{}, knowledgesvc.RebuildDoc(ctx, req.ID, req.ChunkStrategy)
}
```

### 2. 服务层：`BatchRebuildDoc`（`internal/service/knowledge/knowledge.go`）

**现状**：签名为 `BatchRebuildDoc(ctx, docIDs []string)`，不接受策略参数。

**改动**：增加 `newStrategy string` 参数并透传。

```go
func BatchRebuildDoc(ctx context.Context, docIDs []string, newStrategy string) (submitted, failed int) {
    for _, id := range docIDs {
        if err := RebuildDoc(ctx, id, newStrategy); err != nil {
            g.Log().Warningf(ctx, "[knowledge] 批量重建文档 %s 失败: %v", id, err)
            failed++
        } else {
            submitted++
        }
    }
    return
}
```

> `newStrategy` 直接透传给 `RebuildDoc`，包括空串。`RebuildDoc` 内部已处理"空串=保持原策略"逻辑，`BatchRebuildDoc` 层无需额外判断。

### 3. Controller：`DocBatchRebuild`（`internal/controller/knowledge/knowledge.go`）

**改动**：透传 `req.ChunkStrategy`。

```go
func (c *controllerV1) DocBatchRebuild(ctx context.Context, req *v1.DocBatchRebuildReq) (res *v1.DocBatchRebuildRes, err error) {
    submitted, failed := knowledgesvc.BatchRebuildDoc(ctx, req.IDs, req.ChunkStrategy)
    return &v1.DocBatchRebuildRes{Submitted: submitted, Failed: failed}, nil
}
```

### API 层

`DocRebuildReq.ChunkStrategy` 和 `DocBatchRebuildReq.ChunkStrategy` 字段**无需改动**，已存在。

---

## 前端改动

### 1. 新增 `RebuildModal` 组件（内联于 `docs.tsx`）

**Props**：
```ts
interface RebuildModalProps {
  mode: 'single' | 'batch'
  doc?: DocItem          // mode=single 时传入，展示当前策略
  count?: number         // mode=batch 时传入，展示"N 个文档"；由调用处从 selected.size 实时计算后传入
  onConfirm: (strategy: string) => void
  onClose: () => void
}
```

**调用处状态结构体**：
```ts
const [rebuildModal, setRebuildModal] = useState<{
  open: boolean
  mode: 'single' | 'batch'
  doc?: DocItem
  count?: number         // mode=batch 时由 selected.size 赋值
} | null>(null)
```

**UI 内容**：
- 标题：重建索引
- 说明：mode=single 时展示当前策略（只读）；mode=batch 时展示"将对 N 个文档重建"
- 策略单选卡片（3 个）：
  - 固定分块（`fixed_size`）— 通用场景，均匀切分
  - 结构感知（`structure_aware`）— Markdown 文档，保留标题边界
  - 层级分块（`hierarchical`）— 父子双层，检索精度与上下文完整性兼顾
  - "保持原策略"选项（对应空串，**单批量模式均显示**；mode=single 时展示文档当前策略名作为副文字）
- 取消 / 确认重建 按钮

**默认选中**：mode=single 时默认选中文档现有策略；mode=batch 时默认选"保持原策略"（空串）。

### 2. 修改 `knowledgeService`（`web/src/services/knowledge.ts`）

```ts
async rebuildDoc(id: string, chunkStrategy?: string): Promise<void> {
  await api.post('/knowledge/v1/docs/rebuild', { id, chunk_strategy: chunkStrategy || '' })
}

async batchRebuildDocs(ids: string[], chunkStrategy?: string): Promise<{ submitted: number; failed: number }> {
  const res = await api.post<ApiWrap<...>>('/knowledge/v1/docs/batch_rebuild', {
    ids,
    chunk_strategy: chunkStrategy || '',
  })
  return res.data.data ?? { submitted: 0, failed: 0 }
}
```

### 3. 修改 `docs.tsx` 操作逻辑

新增状态：
```ts
const [rebuildModal, setRebuildModal] = useState<{
  open: boolean
  mode: 'single' | 'batch'
  doc?: DocItem
  count?: number         // mode=batch 时由 selected.size 赋值
} | null>(null)
```

`handleRebuildDoc(doc)` → 打开 `RebuildModal`（mode=single，传入 doc）
`handleBatchRebuild()` → 打开 `RebuildModal`（mode=batch，count=selected.size）

弹窗 `onConfirm(strategy)` 回调中执行原有提交逻辑，成功后关闭弹窗、更新本地状态为 `pending`。

---

## 数据流

```
用户点击"重建" / 批量重建
       ↓
RebuildModal 展示（策略单选卡片）
       ↓
用户选择策略 → 点击确认
       ↓
前端 rebuildDoc(id, strategy) / batchRebuildDocs(ids, strategy)
       ↓
POST /knowledge/v1/docs/rebuild  { id, chunk_strategy }
       ↓
Controller.DocRebuild → RebuildDoc(ctx, id, strategy)
       ↓
服务层：清理旧向量 → 清理旧 chunks → 更新 chunk_strategy/chunk_config → 重置 pending → 投入 Worker Pool
       ↓
Worker Pool 异步重新分块 + 向量化 + 写入 Milvus
```

---

## 不改动的部分

| 文件 | 原因 |
|------|------|
| `internal/ai/document/chunker.go` | 分块逻辑完整，无需修改 |
| `api/knowledge/v1/knowledge.go` | API 结构体字段已存在 |
| `chunks.tsx` / `ChunkDrawer.tsx` | 只展示分块结果，无交互改动 |
| 轮询逻辑 | 重建后状态变为 `pending`，现有3秒轮询自动生效 |
| `DocUploadModal` | 上传时策略自动选择逻辑保持不变 |

---

## 验收标准

1. 单文档点击"重建"弹出策略选择弹窗，选择不同策略后重建，`chunk_strategy` 字段在数据库中更新
2. 批量重建时可统一切换策略，或留空保持各文档原策略
3. 策略为空串时行为与修改前一致（保持原策略）
4. 重建完成后，通过 `GET /knowledge/v1/chunks?doc_id=xxx` 确认 chunk 记录的 `section_title` 和 char_count 分布符合所选策略特征（如 structure_aware 产生更大且带章节标题的块，hierarchical 产生父子两级）；通过 RAG 检索测试页（`/knowledge/v1/search`）验证召回结果文本边界符合新策略分块
