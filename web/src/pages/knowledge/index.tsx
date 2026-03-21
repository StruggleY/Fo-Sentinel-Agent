import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Plus, Database, Trash2, Loader2, FileText, Layers,
  RefreshCw, ArrowRight, Search, Inbox,
} from 'lucide-react'
import { cn } from '@/utils'
import { knowledgeService, type KnowledgeBase } from '@/services/knowledge'
import StatCard from '@/components/common/StatCard'
import Pagination from '@/components/common/Pagination'
import BaseCreateModal from './components/BaseCreateModal'
import ConfirmDialog from '@/components/common/ConfirmDialog'
import toast from 'react-hot-toast'

export default function Knowledge() {
  const navigate = useNavigate()
  const [bases, setBases] = useState<KnowledgeBase[]>([])
  const [loading, setLoading] = useState(false)
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [showCreate, setShowCreate] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState<{ open: boolean; base?: KnowledgeBase; isBatch?: boolean }>({ open: false })
  const [queueLen, setQueueLen] = useState(0)

  const fetchBases = useCallback(async () => {
    try {
      setLoading(true)
      setBases(await knowledgeService.listBases())
    } catch {
      toast.error('获取知识库列表失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchBases() }, [fetchBases])

  useEffect(() => {
    const poll = async () => {
      try { setQueueLen(await knowledgeService.queueStatus()) } catch { /* ignore */ }
    }
    poll()
    const t = setInterval(poll, 5000)
    return () => clearInterval(t)
  }, [])

  const filtered = bases.filter(b =>
    b.name.toLowerCase().includes(search.toLowerCase()) ||
    b.description?.toLowerCase().includes(search.toLowerCase())
  )
  const paginated = filtered.slice((page - 1) * pageSize, page * pageSize)
  const totalPages = Math.ceil(filtered.length / pageSize)

  const totalDocs = bases.reduce((s, b) => s + b.doc_count, 0)
  const totalChunks = bases.reduce((s, b) => s + b.chunk_count, 0)

  const toggleSelect = (id: string) => {
    setSelected(prev => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })
  }

  const pageSelectableIds = paginated.filter(b => b.id !== 'default').map(b => b.id)
  const pageAllSelected = pageSelectableIds.length > 0 && pageSelectableIds.every(id => selected.has(id))
  const pagePartialSelected = !pageAllSelected && pageSelectableIds.some(id => selected.has(id))

  const toggleAll = () => {
    setSelected(prev => {
      const next = new Set(prev)
      if (pageAllSelected) {
        pageSelectableIds.forEach(id => next.delete(id))
      } else {
        pageSelectableIds.forEach(id => next.add(id))
      }
      return next
    })
  }

  const handleDeleteBase = async (base: KnowledgeBase) => {
    try {
      await knowledgeService.deleteBase(base.id)
      toast.success(`知识库「${base.name}」已删除`)
      setBases(prev => prev.filter(b => b.id !== base.id))
      setSelected(prev => { const next = new Set(prev); next.delete(base.id); return next })
    } catch {
      toast.error('删除失败')
    }
  }

  const handleBatchDelete = async () => {
    const ids = [...selected].filter(id => id !== 'default')
    let ok = 0
    for (const id of ids) {
      try { await knowledgeService.deleteBase(id); ok++ } catch { /* ignore */ }
    }
    toast.success(`已删除 ${ok} 个知识库`)
    setBases(prev => prev.filter(b => !ids.includes(b.id)))
    setSelected(new Set())
  }

  return (
    <div className="flex flex-col gap-5">

      {/* 标题栏 */}
      <div className="flex items-center justify-between flex-shrink-0">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900 tracking-tight">AI 知识库</h1>
          <p className="text-sm text-gray-500 mt-1">共 {bases.length} 个知识库</p>
        </div>
        <div className="flex items-center gap-2">
          {queueLen > 0 && (
            <span className="inline-flex items-center gap-1.5 text-xs text-amber-700 bg-amber-50 border border-amber-200 px-3 h-9 rounded-lg font-medium">
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
              队列 {queueLen} 个任务
            </span>
          )}
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <input
              type="text"
              placeholder="搜索知识库…"
              value={search}
              onChange={e => { setSearch(e.target.value); setPage(1) }}
              className="input pl-9 w-52"
            />
          </div>
          <button onClick={fetchBases} disabled={loading} className="btn-default">
            <RefreshCw className={cn('w-4 h-4', loading && 'animate-spin')} />
            刷新
          </button>
          <button onClick={() => setShowCreate(true)} className="btn btn-primary">
            <Plus className="w-4 h-4" />
            新建知识库
          </button>
        </div>
      </div>

      {/* 统计卡片 */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 flex-shrink-0">
        <StatCard label="知识库总数" value={bases.length} Icon={Database} tone="gray" />
        <StatCard label="文档总数" value={totalDocs} Icon={FileText} tone="blue" />
        <StatCard label="分块总数" value={totalChunks} Icon={Layers} tone="indigo" />
        <StatCard label="索引队列" value={queueLen} Icon={Loader2} tone={queueLen > 0 ? 'amber' : 'gray'} />
      </div>

      {/* 批量操作栏 */}
      {selected.size > 0 && (
        <div className="flex items-center gap-3 border border-slate-200 bg-slate-50 px-4 py-2 rounded-lg text-sm">
          <span className="text-slate-700 font-medium">已选 {selected.size} 个</span>
          <button
            onClick={() => setConfirmDelete({ open: true, isBatch: true })}
            className="px-3 py-1.5 rounded-lg bg-red-100 text-red-700 hover:bg-red-200 text-xs font-medium transition-colors ml-2"
          >
            批量删除
          </button>
          <button
            onClick={() => setSelected(new Set())}
            className="ml-auto text-xs text-gray-500 hover:text-gray-700 px-2 py-1 rounded hover:bg-gray-100 transition-colors"
          >
            取消选择
          </button>
        </div>
      )}

      {/* 表格卡片 */}
      <div className="card flex flex-col">
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-8 h-8 animate-spin text-gray-400" />
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="table w-full" style={{ tableLayout: 'fixed', minWidth: '720px' }}>
              <colgroup>
                {/* 复选框列 */}
                <col style={{ width: '44px' }} />
                {/* 名称列：弹性主列 */}
                <col style={{ width: '22%' }} />
                {/* 描述列 */}
                <col style={{ width: '22%' }} />
                {/* 文档数 */}
                <col style={{ width: '80px' }} />
                {/* 分块数 */}
                <col style={{ width: '80px' }} />
                {/* 创建时间 */}
                <col style={{ width: '140px' }} />
                {/* 修改时间 */}
                <col style={{ width: '140px' }} />
                {/* 操作：固定宽度，能容纳 2 个带文字按钮 */}
                <col style={{ width: '160px' }} />
              </colgroup>
              <thead>
                <tr>
                  <th className="pl-4 w-11">
                    <input
                      type="checkbox"
                      className="rounded border-gray-300"
                      checked={pageAllSelected}
                      ref={el => { if (el) el.indeterminate = pagePartialSelected }}
                      onChange={toggleAll}
                    />
                  </th>
                  <th className="text-left px-3">名称</th>
                  <th className="text-left px-3">描述</th>
                  <th className="text-left px-2">文档数</th>
                  <th className="text-left px-2">分块数</th>
                  <th className="text-left px-3">创建时间</th>
                  <th className="text-left px-3">修改时间</th>
                  <th className="text-left pl-2 pr-4">操作</th>
                </tr>
              </thead>
              <tbody>
                {paginated.length === 0 ? (
                  <tr>
                    <td colSpan={8}>
                      <div className="py-20 flex flex-col items-center text-gray-400">
                        <Inbox className="w-16 h-16 mb-4 text-gray-200" />
                        <p className="text-base font-medium text-gray-500 mb-1">暂无知识库</p>
                        <p className="text-sm text-gray-400">点击「新建知识库」创建第一个</p>
                      </div>
                    </td>
                  </tr>
                ) : (
                  <>
                    {paginated.map(base => (
                  <tr
                    key={base.id}
                    className={cn('cursor-pointer group', selected.has(base.id) && 'bg-blue-50/60')}
                    onClick={() => navigate(`/knowledge/${base.id}/docs`)}
                  >
                    {/* 复选框：默认库不可选 */}
                    <td className="pl-4 py-3.5" onClick={e => e.stopPropagation()}>
                      {base.id !== 'default' ? (
                        <input
                          type="checkbox"
                          className="rounded border-gray-300"
                          checked={selected.has(base.id)}
                          onChange={() => toggleSelect(base.id)}
                        />
                      ) : (
                        <span className="inline-block w-4 h-4" />
                      )}
                    </td>

                    {/* 名称 */}
                    <td className="py-3.5 px-3 min-w-0">
                      <div className="flex items-center gap-2 min-w-0">
                        <div className="w-7 h-7 rounded-lg bg-indigo-50 flex items-center justify-center flex-shrink-0">
                          <Database className="w-3.5 h-3.5 text-indigo-500" />
                        </div>
                        <span className="text-sm font-medium text-gray-900 truncate">{base.name}</span>
                        {base.id === 'default' && (
                          <span className="tag tag-default flex-shrink-0 whitespace-nowrap">默认</span>
                        )}
                      </div>
                    </td>

                    {/* 描述 */}
                    <td className="py-3.5 px-3 text-gray-500 min-w-0">
                      <span className="truncate block text-sm">{base.description || ''}</span>
                    </td>

                    {/* 文档数 */}
                    <td className="py-3.5 px-2">
                      <span className="tag bg-blue-50 text-blue-700">
                        <FileText className="w-3 h-3 mr-1" />{base.doc_count}
                      </span>
                    </td>

                    {/* 分块数 */}
                    <td className="py-3.5 px-2">
                      <span className="tag bg-indigo-50 text-indigo-700">
                        <Layers className="w-3 h-3 mr-1" />{base.chunk_count}
                      </span>
                    </td>

                    {/* 创建时间 */}
                    <td className="py-3.5 px-3 text-gray-500 text-xs whitespace-nowrap">{base.created_at}</td>

                    {/* 修改时间 */}
                    <td className="py-3.5 px-3 text-gray-500 text-xs whitespace-nowrap">{base.updated_at}</td>

                    {/* 操作 */}
                    <td className="py-3.5 pl-2 pr-4" onClick={e => e.stopPropagation()}>
                      <div className="flex items-center gap-1.5">
                        <button
                          onClick={() => navigate(`/knowledge/${base.id}/docs`)}
                          className="inline-flex items-center gap-1 px-2.5 h-7 rounded-md border border-indigo-200 bg-indigo-50 text-indigo-700 hover:bg-indigo-100 text-xs font-medium whitespace-nowrap transition-all"
                        >
                          <ArrowRight className="w-3 h-3" />
                          管理文档
                        </button>
                        {base.id !== 'default' && (
                          <button
                            onClick={() => setConfirmDelete({ open: true, base })}
                            className="inline-flex items-center gap-1 px-2.5 h-7 rounded-md border border-red-200 bg-red-50 text-red-600 hover:bg-red-100 text-xs font-medium whitespace-nowrap transition-all"
                          >
                            <Trash2 className="w-3 h-3" />
                            删除
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                    ))}
                  </>
                )}
              </tbody>
            </table>
          </div>
        )}

        {!loading && filtered.length > 0 && (
          <Pagination
            page={page}
            totalPages={totalPages}
            total={filtered.length}
            pageSize={pageSize}
            onChange={setPage}
            onPageSizeChange={size => { setPageSize(size); setPage(1) }}
          />
        )}
      </div>

      {showCreate && (
        <BaseCreateModal
          onClose={() => setShowCreate(false)}
          onSuccess={base => { setBases(prev => [base, ...prev]); setShowCreate(false) }}
        />
      )}

      <ConfirmDialog
        open={confirmDelete.open}
        title={confirmDelete.isBatch ? `删除 ${[...selected].filter(id => id !== 'default').length} 个知识库` : '删除知识库'}
        description={
          confirmDelete.isBatch
            ? `确定要删除选中的 ${[...selected].filter(id => id !== 'default').length} 个知识库及其所有文档和向量吗？此操作无法撤销。`
            : `确定要删除知识库「${confirmDelete.base?.name}」吗？其下所有文档和向量将一并删除，此操作无法撤销。`
        }
        onConfirm={() => {
          if (confirmDelete.isBatch) handleBatchDelete()
          else if (confirmDelete.base) handleDeleteBase(confirmDelete.base)
        }}
        onClose={() => setConfirmDelete({ open: false })}
      />
    </div>
  )
}
