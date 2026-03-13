import { useState, useEffect } from 'react'
import { CheckCircle, AlertCircle, PauseCircle, Loader2 } from 'lucide-react'
import { cn, formatRelativeTime, getSourceTypeLabel } from '@/utils'
import { subscriptionService } from '@/services/subscription'
import type { Subscription } from '@/types'

const statusConfig = {
  active: {
    icon: CheckCircle,
    color: 'text-success-500',
    label: '运行中',
  },
  paused: {
    icon: PauseCircle,
    color: 'text-warning-500',
    label: '已暂停',
  },
  disabled: {
    icon: AlertCircle,
    color: 'text-danger-500',
    label: '已禁用',
  },
}

export default function SubscriptionStatus() {
  const [subscriptions, setSubscriptions] = useState<Subscription[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchSubscriptions = async () => {
      try {
        const res = await subscriptionService.list(1, 5)
        setSubscriptions(res.list || [])
      } catch (error) {
        console.error('获取订阅列表失败:', error)
      } finally {
        setLoading(false)
      }
    }
    fetchSubscriptions()
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <Loader2 className="w-6 h-6 animate-spin text-primary-500" />
      </div>
    )
  }

  if (subscriptions.length === 0) {
    return (
      <div className="text-center py-8 text-gray-500">
        暂无订阅
      </div>
    )
  }

  return (
    <div className="table-container">
      <table className="table">
        <thead>
          <tr>
            <th>名称</th>
            <th>类型</th>
            <th>状态</th>
            <th>上次抓取</th>
            <th>事件数</th>
          </tr>
        </thead>
        <tbody>
          {subscriptions.map((sub) => {
            const config = statusConfig[sub.status as keyof typeof statusConfig] || statusConfig.active
            const StatusIcon = config.icon
            return (
              <tr key={sub.id} className="cursor-pointer">
                <td className="font-medium">{sub.name}</td>
                <td>
                  <span className="tag tag-default">{getSourceTypeLabel(sub.source_type)}</span>
                </td>
                <td className="whitespace-nowrap">
                  <span className={cn('flex items-center gap-1.5', config.color)}>
                    <StatusIcon className="w-3.5 h-3.5 flex-shrink-0" />
                    {config.label}
                  </span>
                </td>
                <td className="text-gray-500">
                  {sub.last_fetch_at ? formatRelativeTime(sub.last_fetch_at) : '-'}
                </td>
                <td className="text-right">{sub.total_events || 0}</td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
