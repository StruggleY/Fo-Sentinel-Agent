import { useState, useEffect } from 'react'
import {
  Search,
  ExternalLink,
  Shield,
  Trash2,
  RefreshCw,
  Loader2,
  ChevronUp,
  ChevronDown as ChevronDownIcon,
} from 'lucide-react'
import { cn, formatDate } from '@/utils'
import Pagination from '@/components/common/Pagination'
import { useContextStore } from '@/stores/contextStore'
import { useSettingsStore } from '@/stores/settingsStore'
import EventDetailModal from './components/EventDetailModal'
import type { SecurityEvent } from '@/types'
import { eventService } from '@/services/event'
import toast from 'react-hot-toast'

type SortField = 'severity' | 'source' | 'created_at'

const severityConfig: Record<string, { label: string; class: string }> = {
  critical: { label: '严重', class: 'severity-critical' },
  high: { label: '高危', class: 'severity-high' },
  medium: { label: '中危', class: 'severity-medium' },
  low: { label: '低危', class: 'severity-low' },
  info: { label: '信息', class: 'severity-info' },
}

export default function Events() {
  const autoMarkRead = useSettingsStore(s => s.autoMarkRead)
  const [searchQuery, setSearchQuery] = useState('')
  const [severityFilter, setSeverityFilter] = useState('all')
  const [sortField, setSortField] = useState<SortField>('created_at')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')
  const [selectedEvent, setSelectedEvent] = useState<SecurityEvent | null>(null)

  const [events, setEvents] = useState<SecurityEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const pageSize = 20
  const setContext = useContextStore((s) => s.setContext)

  const fetchEvents = async () => {
    try {
      setLoading(true)
      const filter: Record<string, unknown> = {
        page, size: pageSize,
        order_by: sortField,
        order_dir: sortDir,
      }
      if (searchQuery) filter.keyword = searchQuery
      if (severityFilter !== 'all') filter.severity = severityFilter
      const res = await eventService.list(filter)
      setEvents(res.list || [])
      setTotal(res.total || 0)
    } catch (error) {
      console.error('[Events] 获取事件列表失败:', error)
      setEvents([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchEvents() }, [page, severityFilter, sortField, sortDir])

  const handleSearch = () => { setPage(1); fetchEvents() }

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDir(d => d === 'desc' ? 'asc' : 'desc')
    } else {
      setSortField(field)
      setSortDir('desc')
    }
    setPage(1)
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定要删除该事件吗？')) return
    try {
      await eventService.delete(id)
      toast.success('已删除')
      fetchEvents()
    } catch {
      toast.error('删除失败')
    }
  }

  const totalPages = Math.ceil(total / pageSize)

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900 tracking-tight">安全事件</h1>
          <p className="text-sm text-gray-500 mt-1">共 {total} 个事件</p>
        </div>
      </div>

      {/* Filters */}
      <div className="card card-body">
        <div className="flex flex-col lg:flex-row gap-3">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <input
              type="text" placeholder="搜索事件标题、CVE ID..."
              value={searchQuery} onChange={e => setSearchQuery(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleSearch()}
              className="input pl-9"
            />
          </div>
          <select value={severityFilter} onChange={e => { setSeverityFilter(e.target.value); setPage(1) }} className="select w-28">
            <option value="all">全部级别</option>
            <option value="critical">严重</option>
            <option value="high">高危</option>
            <option value="medium">中危</option>
            <option value="low">低危</option>
          </select>
          <button onClick={fetchEvents} disabled={loading} className="btn-default">
            <RefreshCw className={cn('w-4 h-4', loading && 'animate-spin')} />刷新
          </button>
        </div>
      </div>

      {/* Event Table */}
      <div className="card">
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-8 h-8 animate-spin text-primary-500" />
          </div>
        ) : (
          <div className="table-container">
            <table className="table w-full whitespace-nowrap">
              <thead>
                <tr>
                  <th className="pl-6 w-full min-w-[300px]">事件</th>
                  {[
                    { id: 'severity', label: '级别', align: 'center' },
                    { id: 'source', label: '来源', align: 'left' },
                    { id: 'created_at', label: '时间', align: 'left' }
                  ].map(({ id, label, align }) => {
                    const field = id as SortField
                    const active = sortField === field
                    return (
                      <th key={field} className={cn(align === 'center' ? 'text-center' : 'text-left', 'px-4')}>
                        <button
                          onClick={() => handleSort(field)}
                          className={cn("inline-flex items-center gap-1 hover:text-gray-900 transition-colors", align === 'center' && 'mx-auto')}
                        >
                          {label}
                          <span className="flex flex-col">
                            <ChevronUp className={cn('w-2.5 h-2.5 -mb-0.5', active && sortDir === 'asc' ? 'text-primary-500' : 'text-gray-300')} />
                            <ChevronDownIcon className={cn('w-2.5 h-2.5', active && sortDir === 'desc' ? 'text-primary-500' : 'text-gray-300')} />
                          </span>
                        </button>
                      </th>
                    )
                  })}
                  <th className="text-center pr-6 w-20">操作</th>
                </tr>
              </thead>
              <tbody>
                {events.map(event => {
                  const severity = severityConfig[event.severity] || severityConfig.info
                  return (
                    <tr
                      key={event.id}
                      className="cursor-pointer"
                      onClick={() => {
                        setSelectedEvent(event)
                        setContext('events', event.id, event.title)
                        if (autoMarkRead && event.status === 'new') {
                          eventService.updateStatus(event.id, 'processing')
                            .then(() => setEvents(prev => prev.map(e => e.id === event.id ? { ...e, status: 'processing' as const } : e)))
                            .catch(() => {})
                        }
                      }}
                    >
                      <td className="pl-6 py-4 max-w-0 w-full">
                        <div className="flex items-center gap-1.5 min-w-0 pr-4">
                          <p className="text-sm font-medium text-gray-900 truncate" title={event.title}>{event.title}</p>
                          {event.source_url && (
                            <a href={event.source_url} target="_blank" rel="noopener noreferrer"
                              onClick={e => e.stopPropagation()}
                              className="text-gray-400 hover:text-primary-500 transition-colors shrink-0">
                              <ExternalLink className="w-3.5 h-3.5" />
                            </a>
                          )}
                        </div>
                        {event.cve_id && (
                          <div className="flex items-center gap-2 mt-1">
                            <p className="text-xs text-gray-400 font-mono truncate">
                              {event.cve_id}{event.cvss_score ? ` · CVSS ${event.cvss_score}` : ''}
                            </p>
                          </div>
                        )}
                      </td>

                      <td className="py-4 px-4 text-center">
                        <span className={severity.class}>{severity.label}</span>
                      </td>

                      <td className="py-4 px-4">
                        <span className="tag tag-default truncate max-w-[200px] inline-block" title={event.source}>{event.source}</span>
                      </td>

                      <td className="text-gray-500 text-sm py-4 px-4">
                        {formatDate(event.created_at, 'YYYY-MM-DD HH:mm')}
                      </td>

                      <td className="text-center pr-6 py-4" onClick={e => e.stopPropagation()}>
                        <button
                          onClick={() => handleDelete(event.id)}
                          className="p-1.5 rounded text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors inline-flex items-center justify-center"
                          title="删除"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}

        {!loading && events.length === 0 && (
          <div className="empty">
            <Shield className="empty-icon" />
            <p className="empty-text">没有找到匹配的安全事件</p>
          </div>
        )}

        {!loading && events.length > 0 && (
          <Pagination page={page} totalPages={totalPages} total={total} onChange={setPage} />
        )}
      </div>

      <EventDetailModal event={selectedEvent} onClose={() => setSelectedEvent(null)} onUpdate={(updated) => setSelectedEvent(updated)} />
    </div>
  )
}
