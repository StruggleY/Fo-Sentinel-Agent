import { useState, useEffect } from 'react'
import {
  Plus,
  Search,
  Play,
  Pause,
  Trash2,
  RefreshCw,
  Github,
  Rss,
  CheckCircle,
  PauseCircle,
  Ban,
  Loader2,
} from 'lucide-react'
import { cn, formatRelativeTime, getSourceTypeLabel, formatCronInterval } from '@/utils'
import Pagination from '@/components/common/Pagination'
import AddSubscriptionModal from './components/AddSubscriptionModal'
import { subscriptionService } from '@/services/subscription'
import type { Subscription } from '@/types'
import toast from 'react-hot-toast'

const sourceTypeIcons: Record<string, typeof Github> = {
  github_repo: Github,
  rss: Rss,
}

const statusConfig = {
  active: {
    icon: CheckCircle,
    class: 'status-active',
    label: '运行中',
  },
  paused: {
    icon: PauseCircle,
    class: 'status-paused',
    label: '已暂停',
  },
  disabled: {
    icon: Ban,
    class: 'status-disabled',
    label: '已禁用',
  },
}

export default function Subscriptions() {
  const [showAddModal, setShowAddModal] = useState(false)
  const [editingSubscription, setEditingSubscription] = useState<Subscription | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [filterType, setFilterType] = useState<string>('all')
  const [filterStatus, setFilterStatus] = useState<string>('all')

  const [subscriptions, setSubscriptions] = useState<Subscription[]>([])
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [pageSize] = useState(10)

  const fetchSubscriptions = async () => {
    try {
      setLoading(true)
      const res = await subscriptionService.list(page, pageSize)
      setSubscriptions(res.list || [])
    } catch (error) {
      console.error('[Subscriptions] 获取订阅列表失败:', error)
      setSubscriptions([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchSubscriptions()
  }, [page])

  const handlePause = async (id: string) => {
    try {
      await subscriptionService.pause(id)
      toast.success('已暂停订阅')
      fetchSubscriptions()
    } catch (error) {
      toast.error('操作失败')
    }
  }

  const handleResume = async (id: string) => {
    try {
      await subscriptionService.resume(id)
      toast.success('已恢复订阅')
      fetchSubscriptions()
    } catch (error) {
      toast.error('操作失败')
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定要删除这个订阅吗？')) return
    try {
      await subscriptionService.delete(id)
      toast.success('已删除订阅')
      fetchSubscriptions()
    } catch (error) {
      toast.error('删除失败')
    }
  }

  const handleEdit = (subscription: Subscription) => {
    setEditingSubscription(subscription)
    setShowAddModal(true)
  }

  const filteredSubscriptions = subscriptions.filter((sub) => {
    const matchesSearch = sub.name.toLowerCase().includes(searchQuery.toLowerCase())
    const matchesType = filterType === 'all' || sub.source_type === filterType
    const matchesStatus = filterStatus === 'all' || sub.status === filterStatus
    return matchesSearch && matchesType && matchesStatus
  })

  const pagedSubscriptions = filteredSubscriptions.slice((page - 1) * pageSize, page * pageSize)
  const totalPages = Math.ceil(filteredSubscriptions.length / pageSize)

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900 tracking-tight">订阅管理</h1>
          <p className="text-sm text-gray-500 mt-1">管理安全事件数据源订阅</p>
        </div>
        <button onClick={() => setShowAddModal(true)} className="btn-primary">
          <Plus className="w-4 h-4" />
          添加订阅
        </button>
      </div>

      {/* Filters */}
      <div className="card card-body">
        <div className="flex flex-col sm:flex-row gap-4">
          {/* Search */}
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
            <input
              type="text"
              placeholder="搜索订阅名称..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="input pl-9"
            />
          </div>

          {/* Type Filter */}
          <select
            value={filterType}
            onChange={(e) => setFilterType(e.target.value)}
            className="select w-40 focus:border-gray-400 focus:ring-gray-400"
          >
            <option value="all">全部类型</option>
            <option value="github_repo">GitHub</option>
            <option value="rss">RSS</option>
          </select>

          {/* Status Filter */}
          <select
            value={filterStatus}
            onChange={(e) => setFilterStatus(e.target.value)}
            className="select w-32 focus:border-gray-400 focus:ring-gray-400"
          >
            <option value="all">全部状态</option>
            <option value="active">运行中</option>
            <option value="paused">已暂停</option>
            <option value="disabled">已禁用</option>
          </select>

          {/* Refresh Button */}
          <button
            onClick={fetchSubscriptions}
            disabled={loading}
            className="btn-default"
          >
            <RefreshCw className={cn('w-4 h-4', loading && 'animate-spin')} />
            刷新
          </button>
        </div>
      </div>

      {/* Subscription Table */}
      <div className="card flex flex-col" style={{ height: 'calc(100vh - 320px)' }}>
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-8 h-8 animate-spin text-primary-500" />
          </div>
        ) : (
          <>
            <div className="flex-1 overflow-auto">
              <table className="table w-full">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>类型</th>
                  <th>状态</th>
                  <th>拉取间隔</th>
                  <th>上次抓取</th>
                  <th className="text-right">事件数</th>
                  <th className="text-center w-44">操作</th>
                </tr>
              </thead>
              <tbody>
                {pagedSubscriptions.map((sub) => {
                  const SourceIcon = sourceTypeIcons[sub.source_type] || Rss
                  const config = statusConfig[sub.status as keyof typeof statusConfig]
                  const StatusIcon = config?.icon || CheckCircle

                  return (
                    <tr key={sub.id}>
                      <td className="min-w-[280px]">
                        <div className="flex items-center gap-3">
                          <div className="w-8 h-8 rounded bg-gray-100 flex items-center justify-center flex-shrink-0">
                            <SourceIcon className="w-4 h-4 text-gray-500" />
                          </div>
                          <div className="min-w-0 flex-1">
                            <p className="font-medium text-gray-900 mb-0.5">{sub.name}</p>
                            <a
                              href={sub.source_url}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-xs text-gray-500 hover:text-blue-600 truncate block"
                              title={sub.source_url}
                            >
                              {sub.source_url}
                            </a>
                          </div>
                        </div>
                      </td>
                      <td className="whitespace-nowrap">
                        <span className="tag tag-default">
                          {getSourceTypeLabel(sub.source_type)}
                        </span>
                      </td>
                      <td className="whitespace-nowrap">
                        <span className={cn('flex items-center gap-1.5', config?.class)}>
                          <StatusIcon className="w-3.5 h-3.5" />
                          {config?.label || sub.status}
                        </span>
                      </td>
                      <td className="text-gray-700 font-mono text-sm whitespace-nowrap">
                        {formatCronInterval(sub.cron_expr)}
                      </td>
                      <td className="text-gray-600 whitespace-nowrap">
                        {sub.last_fetch_at ? formatRelativeTime(sub.last_fetch_at) : '-'}
                      </td>
                      <td className="text-right text-gray-900 font-medium whitespace-nowrap">
                        {(sub.total_events || 0).toLocaleString()}
                      </td>
                      <td className="text-center">
                        <div className="flex items-center justify-center gap-2">
                          {/* 启动/暂停按钮 */}
                          {sub.status === 'active' ? (
                            <button
                              onClick={() => handlePause(sub.id)}
                              className="inline-flex items-center justify-center w-8 h-8 rounded-md border border-amber-200 bg-amber-50 text-amber-600 hover:bg-amber-100 transition-all"
                              title="暂停"
                            >
                              <Pause className="w-4 h-4" />
                            </button>
                          ) : (
                            <button
                              onClick={() => handleResume(sub.id)}
                              className="inline-flex items-center justify-center w-8 h-8 rounded-md border border-emerald-200 bg-emerald-50 text-emerald-600 hover:bg-emerald-100 transition-all"
                              title="启动"
                            >
                              <Play className="w-4 h-4" />
                            </button>
                          )}

                          {/* 编辑按钮 */}
                          <button
                            onClick={() => handleEdit(sub)}
                            className="inline-flex items-center justify-center w-8 h-8 rounded-md border border-gray-200 bg-gray-50 text-gray-600 hover:bg-gray-100 transition-all"
                            title="编辑"
                          >
                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                            </svg>
                          </button>

                          {/* 删除按钮 */}
                          <button
                            onClick={() => handleDelete(sub.id)}
                            className="inline-flex items-center justify-center w-8 h-8 rounded-md border border-red-200 bg-red-50 text-red-600 hover:bg-red-100 transition-all"
                            title="删除"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>

          {/* Pagination - 固定在底部 */}
          {filteredSubscriptions.length > 0 && (
            <Pagination
              page={page}
              totalPages={totalPages}
              total={filteredSubscriptions.length}
              onChange={setPage}
            />
          )}
        </>
        )}

        {!loading && pagedSubscriptions.length === 0 && (
          <div className="flex-1 flex flex-col items-center justify-center py-16 text-gray-500">
            <Rss className="w-16 h-16 mb-4 text-gray-400" />
            <p className="text-sm font-medium mb-4">没有找到匹配的订阅</p>
            <button
              onClick={() => setShowAddModal(true)}
              className="btn-primary"
            >
              <Plus className="w-4 h-4" />
              添加订阅
            </button>
          </div>
        )}
      </div>

      {/* Add Modal */}
      <AddSubscriptionModal
        key={editingSubscription ? `edit-${editingSubscription.id}` : 'new'}
        isOpen={showAddModal}
        onClose={() => {
          setShowAddModal(false)
          setEditingSubscription(null)
        }}
        onSuccess={fetchSubscriptions}
        editMode={!!editingSubscription}
        initialData={editingSubscription || undefined}
      />
    </div>
  )
}
