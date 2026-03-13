import { useState, useEffect } from 'react'
import { ExternalLink, Loader2 } from 'lucide-react'
import { formatDate } from '@/utils'
import { eventService } from '@/services/event'
import type { SecurityEvent } from '@/types'

const severityConfig: Record<string, { label: string; class: string }> = {
  critical: { label: '严重', class: 'severity-critical' },
  high: { label: '高危', class: 'severity-high' },
  medium: { label: '中危', class: 'severity-medium' },
  low: { label: '低危', class: 'severity-low' },
  info: { label: '信息', class: 'severity-info' },
}

export default function RecentEvents() {
  const [events, setEvents] = useState<SecurityEvent[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchEvents = async () => {
      try {
        const res = await eventService.list({ page: 1, size: 5 })
        setEvents(res.list || [])
      } catch (error) {
        console.error('获取最新事件失败:', error)
      } finally {
        setLoading(false)
      }
    }
    fetchEvents()
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <Loader2 className="w-6 h-6 animate-spin text-primary-500" />
      </div>
    )
  }

  if (events.length === 0) {
    return (
      <div className="text-center py-8 text-gray-500">
        暂无安全事件
      </div>
    )
  }

  return (
    <div className="table-container">
      <table className="table">
        <thead>
          <tr>
            <th>事件</th>
            <th>级别</th>
            <th>来源</th>
            <th>时间</th>
          </tr>
        </thead>
        <tbody>
          {events.map((event) => {
            const severity = severityConfig[event.severity] || severityConfig.info
            return (
              <tr key={event.id} className="cursor-pointer group">
                <td className="max-w-0 w-full">
                  <div className="flex items-center gap-2">
                    <span className="truncate block">{event.title}</span>
                    <ExternalLink className="w-3.5 h-3.5 text-gray-500 opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0" />
                  </div>
                </td>
                <td className="whitespace-nowrap">
                  <span className={severity.class}>{severity.label}</span>
                </td>
                <td className="whitespace-nowrap">
                  <span className="tag tag-default">{event.source || '-'}</span>
                </td>
                <td className="text-gray-500 whitespace-nowrap">
                  {formatDate(event.created_at, 'YYYY-MM-DD HH:mm')}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
