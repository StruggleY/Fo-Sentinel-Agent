import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Activity,
  Search,
  RefreshCw,
  Loader2,
  CheckCircle2,
  XCircle,
  Clock,
  Zap,
  Trash2,
  Copy,
  Check,
} from 'lucide-react'
import { cn } from '@/utils'
import StatCard from '@/components/common/StatCard'
import Pagination from '@/components/common/Pagination'
import ConfirmDialog from '@/components/common/ConfirmDialog'
import { traceService, type TraceRun, type TraceStats } from '@/services/trace'
import toast from 'react-hot-toast'

function formatDuration(ms: number): string {
  if (ms <= 0) return '-'
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

function formatTokens(n: number): string {
  if (!n) return '0'
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`
  return String(n)
}

function truncateQuery(text: string, maxLen = 60): string {
  if (!text) return '-'
  if (text.length <= maxLen) return text
  return text.slice(0, maxLen) + '…'
}

function formatTime(iso: string): string {
  if (!iso) return '-'
  try {
    return new Date(iso).toLocaleString('zh-CN', {
      month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
    })
  } catch { return iso }
}

const statusConfig: Record<string, { label: string; class: string }> = {
  success: { label: '成功', class: 'bg-emerald-50 text-emerald-700 border border-emerald-200' },
  error:   { label: '失败', class: 'bg-red-50 text-red-700 border border-red-200' },
  running: { label: '运行中', class: 'bg-amber-50 text-amber-700 border border-amber-200' },
}

// CopyButton 纯复制图标按钮，带复制成功视觉反馈
function CopyButton({ text, className }: { text: string; className?: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <button
      onClick={e => {
        e.stopPropagation()
        navigator.clipboard.writeText(text).then(() => {
          setCopied(true)
          setTimeout(() => setCopied(false), 1500)
        })
      }}
      title={`复制: ${text}`}
      className={cn(
        'p-0.5 rounded transition-all flex-shrink-0',
        copied ? 'text-emerald-500 opacity-100' : 'text-gray-300 hover:text-gray-500',
        className,
      )}
    >
      {copied ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
    </button>
  )
}

export default function Traces() {
  const navigate = useNavigate()
  const [runs, setRuns] = useState<TraceRun[]>([])
  const [total, setTotal] = useState(0)
  const [stats, setStats] = useState<TraceStats | null>(null)
  const [loading, setLoading] = useState(true)
  const [statsLoading, setStatsLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [statusFilter, setStatusFilter] = useState('')
  const [traceIdFilter, setTraceIdFilter] = useState('')
  const [searchInput, setSearchInput] = useState('')
  const [sessionFilter, setSessionFilter] = useState('')
  const [sessionInput, setSessionInput] = useState('')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [confirmDelete, setConfirmDelete] = useState<{ open: boolean; id?: string; isBatch?: boolean }>({ open: false })

  const loadStats = useCallback(async () => {
    setStatsLoading(true)
    try {
      setStats(await traceService.stats(7))
    } catch (e) {
      console.error('加载统计数据失败', e)
    } finally {
      setStatsLoading(false)
    }
  }, [])

  const loadRuns = useCallback(async () => {
    setLoading(true)
    try {
      const res = await traceService.list({
        page, pageSize,
        status: statusFilter || undefined,
        traceId: traceIdFilter || undefined,
        sessionId: sessionFilter || undefined,
      })
      setRuns(res.list || [])
      setTotal(res.total || 0)
    } catch (e) {
      console.error('加载链路数据失败', e)
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, statusFilter, traceIdFilter, sessionFilter])

  useEffect(() => { loadStats() }, [loadStats])
  useEffect(() => { loadRuns() }, [loadRuns])

  // 自动轮询：有 running 状态的链路时，每 5s 刷新一次
  useEffect(() => {
    const hasRunning = runs.some(r => r.status === 'running')
    if (!hasRunning) return
    const timer = setInterval(() => { loadRuns() }, 5000)
    return () => clearInterval(timer)
  }, [runs, loadRuns])

  const handleSearch = () => { setTraceIdFilter(searchInput.trim()); setPage(1) }

  // 应用 SessionId 筛选
  const handleSessionSearch = () => {
    setSessionFilter(sessionInput.trim())
    setPage(1)
  }

  // 勾选逻辑
  const toggleSelect = (id: string) => {
    setSelected(prev => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })
  }
  const toggleSelectAll = () => {
    const pageIds = runs.map(r => r.traceId)
    const allSel = pageIds.length > 0 && pageIds.every(id => selected.has(id))
    setSelected(prev => {
      const next = new Set(prev)
      if (allSel) pageIds.forEach(id => next.delete(id))
      else pageIds.forEach(id => next.add(id))
      return next
    })
  }
  const pageAllSelected = runs.length > 0 && runs.every(r => selected.has(r.traceId))
  const pagePartialSelected = !pageAllSelected && runs.some(r => selected.has(r.traceId))

  // 批量删除
  const handleBatchDelete = async () => {
    const ids = [...selected]
    if (ids.length === 0) return
    try {
      const res = await traceService.batchDelete(ids)
      toast.success(`已删除 ${res.deleted} 条链路记录`)
      setSelected(new Set())
      loadRuns()
      loadStats()
    } catch {
      toast.error('删除失败')
    }
  }

  // 单条删除
  const handleDelete = async (traceId: string) => {
    try {
      const res = await traceService.batchDelete([traceId])
      toast.success(`已删除 ${res.deleted} 条链路记录`)
      loadRuns()
      loadStats()
    } catch {
      toast.error('删除失败')
    }
  }

  const totalPages = Math.ceil(total / pageSize)

  return (
    <div className="flex flex-col gap-5 h-full">

      {/* 页面标题栏 */}
      <div className="flex items-center justify-between flex-shrink-0">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900 tracking-tight">Agent Trace</h1>
          <p className="text-sm text-gray-500 mt-1">AI Agent 全链路追踪与 Token 成本分析</p>
        </div>
        <button
          onClick={() => { loadStats(); loadRuns() }}
          disabled={loading}
          className="btn-default"
        >
          <RefreshCw className={cn('w-4 h-4', loading && 'animate-spin')} />
          刷新
        </button>
      </div>

      {/* 统计卡片 */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 flex-shrink-0">
        {statsLoading ? (
          <div className="col-span-4 flex justify-center py-4">
            <Loader2 className="w-5 h-5 animate-spin text-gray-400" />
          </div>
        ) : (
          <>
            <StatCard label="总请求数（7天）" value={stats?.totalRuns ?? 0} Icon={Activity} tone="blue" />
            <StatCard
              label="成功率"
              value={stats ? `${((1 - stats.errorRate) * 100).toFixed(1)}%` : '-'}
              Icon={CheckCircle2}
              tone="emerald"
              sub={`${stats?.successRuns ?? 0} 成功 / ${stats?.errorRuns ?? 0} 失败`}
            />
            <StatCard
              label="P95 耗时"
              value={formatDuration(stats?.p95DurationMs ?? 0)}
              Icon={Clock}
              tone="amber"
              sub={`均值 ${formatDuration(Math.round(stats?.avgDurationMs ?? 0))}`}
            />
            <StatCard
              label="Token 消耗"
              value={formatTokens((stats?.totalInputTokens ?? 0) + (stats?.totalOutputTokens ?? 0))}
              Icon={Zap}
              tone="blue"
              sub={`↑${formatTokens(stats?.totalInputTokens ?? 0)} ↓${formatTokens(stats?.totalOutputTokens ?? 0)}`}
            />
          </>
        )}
      </div>

      {/* 过滤栏 */}
      <div className="flex flex-wrap items-center gap-3 flex-shrink-0">
        {[
          { value: '', label: '全部' },
          { value: 'success', label: '成功' },
          { value: 'error', label: '失败' },
          { value: 'running', label: '运行中' },
        ].map(opt => (
          <button
            key={opt.value}
            onClick={() => { setStatusFilter(opt.value); setPage(1) }}
            className={cn(
              'px-3 h-8 rounded-lg text-sm font-medium transition-all border',
              statusFilter === opt.value
                ? 'bg-gray-900 text-white border-gray-900'
                : 'bg-white text-gray-600 border-gray-200 hover:border-gray-400 hover:text-gray-900',
            )}
          >
            {opt.label}
          </button>
        ))}
        <div className="flex gap-2 ml-auto">
        {/* SessionId 筛选 */}
          <div className="relative">
            <input
              value={sessionInput}
              onChange={e => setSessionInput(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleSessionSearch()}
              placeholder="筛选会话 ID..."
              className="input w-44 text-xs"
            />
          </div>
          <button onClick={handleSessionSearch} className="btn-default text-xs px-2">筛选</button>
          {sessionFilter && (
            <button
              onClick={() => { setSessionFilter(''); setSessionInput(''); setPage(1) }}
              className="text-xs text-gray-400 hover:text-gray-600 underline"
            >
              清除会话筛选
            </button>
          )}
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <input
              value={searchInput}
              onChange={e => setSearchInput(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleSearch()}
              placeholder="搜索 TraceId..."
              className="input pl-9 w-52"
            />
          </div>
          <button onClick={handleSearch} className="btn-default">搜索</button>
        </div>
      </div>

      {/* 批量操作栏（有勾选时显示） */}
      {selected.size > 0 && (
        <div className="flex items-center gap-3 border border-slate-200 bg-slate-50 px-4 py-2 rounded-lg text-sm flex-shrink-0">
          <span className="text-slate-700 font-medium">已选 {selected.size} 条</span>
          <div className="flex gap-2 ml-2">
            <button
              onClick={() => setConfirmDelete({ open: true, isBatch: true })}
              className="px-3 py-1.5 rounded-lg bg-red-100 text-red-700 hover:bg-red-200 text-xs font-medium transition-colors"
            >
              批量删除
            </button>
          </div>
          <button
            onClick={() => setSelected(new Set())}
            className="ml-auto text-xs text-gray-500 hover:text-gray-700 px-2 py-1 rounded hover:bg-gray-100 transition-colors"
          >
            取消选择
          </button>
        </div>
      )}

      {/* 数据表格 */}
      <div className="card flex-1 min-h-0 flex flex-col overflow-hidden">
        <div className="overflow-auto flex-1">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-100">
                {/* 全选复选框 */}
                <th className="px-4 py-3 w-10">
                  <input
                    type="checkbox"
                    checked={pageAllSelected}
                    ref={el => { if (el) el.indeterminate = pagePartialSelected }}
                    onChange={toggleSelectAll}
                    className="w-4 h-4 rounded border-gray-300 text-gray-900 cursor-pointer"
                  />
                </th>
                {['TraceID', '聊天会话 ID', '查询内容', '状态', '耗时', 'Token (↑输入 / ↓输出)', '时间'].map(h => (
                  <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-gray-800 whitespace-nowrap">
                    {h}
                  </th>
                ))}
                <th className="px-4 py-3 text-center text-xs font-semibold text-gray-800 w-16">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {loading ? (
                <tr>
                  <td colSpan={9} className="px-4 py-12 text-center">
                    <Loader2 className="w-6 h-6 animate-spin mx-auto text-gray-400" />
                  </td>
                </tr>
              ) : runs.length === 0 ? (
                <tr>
                  <td colSpan={9} className="px-4 py-12 text-center text-sm text-gray-400">
                    暂无链路数据
                  </td>
                </tr>
              ) : (
                runs.map(run => {
                  const sc = statusConfig[run.status] || statusConfig.running
                  const isSelected = selected.has(run.traceId)
                  return (
                    <tr
                      key={run.traceId}
                      onClick={() => navigate(`/traces/${run.traceId}`)}
                      className={cn(
                        'hover:bg-gray-50 cursor-pointer transition-colors',
                        isSelected && 'bg-indigo-50/50',
                      )}
                    >
                      {/* 复选框（不冒泡到行点击） */}
                      <td className="px-4 py-3 w-10" onClick={e => e.stopPropagation()}>
                        <input
                          type="checkbox"
                          checked={isSelected}
                          onChange={() => toggleSelect(run.traceId)}
                          className="w-4 h-4 rounded border-gray-300 text-gray-900 cursor-pointer"
                        />
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-1.5 group/tid">
                          <code
                            className="text-xs font-mono bg-indigo-50 text-indigo-600 px-1.5 py-0.5 rounded"
                            title={run.traceId}
                          >
                            {run.traceId.slice(0, 8)}…
                          </code>
                          <CopyButton text={run.traceId} className="opacity-0 group-hover/tid:opacity-100" />
                        </div>
                      </td>
                      {/* 会话列 */}
                      <td className="px-4 py-3 min-w-[160px]" onClick={e => e.stopPropagation()}>
                        {run.sessionId ? (
                          <div className="flex items-center gap-1.5 group/sid">
                            <button
                              onClick={() => {
                                setSessionInput(run.sessionId)
                                setSessionFilter(run.sessionId)
                                setPage(1)
                              }}
                              className="text-xs font-mono text-indigo-500 hover:text-indigo-700 bg-indigo-50 px-1.5 py-0.5 rounded"
                              title={run.sessionId}
                            >
                              {run.sessionId.split('-').pop() ?? run.sessionId.slice(-10)}
                            </button>
                            <CopyButton text={run.sessionId} className="opacity-0 group-hover/sid:opacity-100" />
                          </div>
                        ) : <span className="text-gray-300 text-xs">-</span>}
                      </td>
                      <td className="px-4 py-3 max-w-xs">
                        <p className="text-gray-800 truncate">{truncateQuery(run.queryText)}</p>
                        {run.traceName && (
                          <p className="text-xs text-gray-400 mt-0.5">{run.traceName}</p>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        <span className={cn('inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium', sc.class)}>
                          {run.status === 'success' && <CheckCircle2 className="w-3 h-3" />}
                          {run.status === 'error' && <XCircle className="w-3 h-3" />}
                          {run.status === 'running' && <Clock className="w-3 h-3" />}
                          {sc.label}
                          {run.errorCode && <span className="opacity-60 ml-1">·{run.errorCode}</span>}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-gray-600 tabular-nums">
                        {formatDuration(run.durationMs)}
                      </td>
                      <td className="px-4 py-3 text-xs tabular-nums">
                        <span className="text-blue-600">↑{formatTokens(run.totalInputTokens)}</span>
                        <span className="text-gray-300 mx-1">/</span>
                        <span className="text-violet-600">↓{formatTokens(run.totalOutputTokens)}</span>
                      </td>
                      <td className="px-4 py-3 text-xs text-gray-400 whitespace-nowrap">
                        {formatTime(run.startTime)}
                      </td>
                      {/* 操作列（不冒泡到行点击） */}
                      <td className="px-4 py-3 text-center" onClick={e => e.stopPropagation()}>
                        <button
                          onClick={(e) => { e.stopPropagation(); setConfirmDelete({ open: true, id: run.traceId }) }}
                          className="inline-flex items-center justify-center w-7 h-7 rounded-md border border-red-200 bg-red-50 text-red-600 hover:bg-red-100 transition-all"
                          title="删除"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>

        {/* 分页 */}
        {!loading && total > 0 && (
          <Pagination
            page={page}
            totalPages={totalPages}
            total={total}
            pageSize={pageSize}
            onChange={setPage}
            onPageSizeChange={size => { setPageSize(size); setPage(1) }}
          />
        )}
      </div>

      <ConfirmDialog
        open={confirmDelete.open}
        title={confirmDelete.isBatch ? `删除 ${selected.size} 条链路记录` : '删除链路记录'}
        description={
          confirmDelete.isBatch
            ? `确定要删除选中的 ${selected.size} 条链路记录吗？此操作无法撤销。`
            : '确定要删除该链路记录吗？此操作无法撤销。'
        }
        onConfirm={() => {
          if (confirmDelete.isBatch) handleBatchDelete()
          else if (confirmDelete.id) handleDelete(confirmDelete.id)
        }}
        onClose={() => setConfirmDelete({ open: false })}
      />
    </div>
  )
}
