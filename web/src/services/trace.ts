import api from './api'

// TraceRun 链路运行记录
export interface TraceRun {
  traceId: string
  traceName: string
  entryPoint: string
  status: 'running' | 'success' | 'error'
  durationMs: number
  startTime: string
  sessionId: string
  queryText: string
  totalInputTokens: number
  totalOutputTokens: number
  totalCachedTokens: number
  errorCode?: string
  errorMessage?: string
  tags: string
}

// TraceNode 链路节点记录
export interface TraceNode {
  nodeId: string
  parentNodeId: string
  nodeType: 'LLM' | 'TOOL' | 'RETRIEVER' | 'LAMBDA' | 'AGENT' | 'CACHE' | 'EMBEDDING'
  nodeName: string
  depth: number
  status: 'running' | 'success' | 'error'
  durationMs: number
  startTime: string
  errorMessage?: string
  errorCode?: string
  errorType?: string
  // LLM 专属
  modelName?: string
  inputTokens?: number
  outputTokens?: number
  cachedTokens?: number
  completionText?: string
  // RETRIEVER 专属
  queryText?: string
  retrievedDocs?: string
  finalTopK?: number
  cacheHit?: boolean
  // 通用
  metadata?: string
}

// TraceStats 统计数据
export interface TraceStats {
  totalRuns: number
  successRuns: number
  errorRuns: number
  avgDurationMs: number
  p95DurationMs: number
  totalInputTokens: number
  totalOutputTokens: number
  errorRate: number
}

export interface ListParams {
  page?: number
  pageSize?: number
  status?: string
  traceId?: string
  sessionId?: string
}

export const traceService = {
  // 查询链路运行列表
  async list(params: ListParams = {}): Promise<{ total: number; list: TraceRun[] }> {
    const res = await api.get('/trace/v1/list', { params })
    return res.data?.data || { total: 0, list: [] }
  },

  // 查询链路详情（含节点树）
  async detail(traceId: string): Promise<{ traceId: string } & TraceRun & { nodes: TraceNode[] }> {
    const res = await api.get('/trace/v1/detail', { params: { traceId } })
    return res.data?.data
  },

  // 查询统计数据
  async stats(days = 7): Promise<TraceStats> {
    const res = await api.get('/trace/v1/stats', { params: { days } })
    return res.data?.data || {}
  },

  // 批量删除链路记录
  async batchDelete(traceIds: string[]): Promise<{ deleted: number }> {
    const res = await api.delete('/trace/v1/batch_delete', { data: { traceIds } })
    return res.data?.data || { deleted: 0 }
  },
}
