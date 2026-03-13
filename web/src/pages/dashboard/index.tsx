import { useState, useEffect } from 'react'
import {
  ShieldAlert,
  Rss,
  FileText,
  TrendingUp,
  RefreshCw,
  Loader2,
} from 'lucide-react'
import { cn } from '@/utils'
import ReactECharts from 'echarts-for-react'
import EventTrendChart from './components/EventTrendChart'
import SeverityDistribution from './components/SeverityDistribution'
import RecentEvents from './components/RecentEvents'
import SubscriptionStatus from './components/SubscriptionStatus'
import ActionQueue from './components/ActionQueue'
import SecurityFunnel from './components/SecurityFunnel'
import { eventService } from '@/services/event'
import { subscriptionService } from '@/services/subscription'
import { reportService } from '@/services/report'

// ECharts 迷你折线图（Sparkline）
function Sparkline({ data, color }: { data: number[]; color: string }) {
  const option = {
    backgroundColor: 'transparent',
    grid: { top: 4, bottom: 4, left: 4, right: 4 },
    xAxis: { type: 'category', show: false, boundaryGap: false },
    yAxis: { type: 'value', show: false },
    series: [{
      type: 'line',
      data,
      smooth: 0.5,
      symbol: 'none',
      lineStyle: { color, width: 2 },
      areaStyle: {
        color: {
          type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [{ offset: 0, color: color + '40' }, { offset: 1, color: color + '00' }],
        },
      },
    }],
  }
  return <ReactECharts option={option} style={{ width: 80, height: 40 }} opts={{ renderer: 'svg' }} />
}

interface DashboardStats {
  eventCount: number
  subscriptionCount: number
  reportCount: number
  todayCount: number
  criticalCount: number
}

export default function Dashboard() {
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [stats, setStats] = useState<DashboardStats>({
    eventCount: 0,
    subscriptionCount: 0,
    reportCount: 0,
    todayCount: 0,
    criticalCount: 0,
  })

  const fetchStats = async () => {
    try {
      const [subscriptionsRes, reportsRes, eventStats] = await Promise.all([
        subscriptionService.list(1, 1),
        reportService.list(1, 1),
        eventService.getStats(),
      ])
      setStats({
        eventCount: eventStats.total || 0,
        subscriptionCount: subscriptionsRes.total || 0,
        reportCount: reportsRes.total || 0,
        todayCount: eventStats.today_count || 0,
        criticalCount: eventStats.critical_count || 0,
      })
    } catch (error) {
      console.error('获取统计数据失败:', error)
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  useEffect(() => { fetchStats() }, [])

  const handleRefresh = () => { setRefreshing(true); fetchStats() }

  const statsConfig = [
    {
      label: '安全事件',
      value: stats.eventCount.toLocaleString(),
      sub: `高危 ${stats.criticalCount}`,
      icon: ShieldAlert,
      color: '#ef4444',
      bgColor: 'bg-red-50',
      textColor: 'text-red-600',
      data: [45, 52, 38, 65, 48, 72, stats.eventCount % 100 || 58],
    },
    {
      label: '活跃订阅',
      value: stats.subscriptionCount.toString(),
      sub: '数据源',
      icon: Rss,
      color: '#3b82f6',
      bgColor: 'bg-blue-50',
      textColor: 'text-blue-600',
      data: [18, 20, 19, 22, 21, 23, stats.subscriptionCount || 24],
    },
    {
      label: '分析报告',
      value: stats.reportCount.toString(),
      sub: '已生成',
      icon: FileText,
      color: '#10b981',
      bgColor: 'bg-emerald-50',
      textColor: 'text-emerald-600',
      data: [3, 5, 4, 7, 6, 8, stats.reportCount || 9],
    },
    {
      label: '今日新增',
      value: stats.todayCount.toString(),
      sub: '今天',
      icon: TrendingUp,
      color: '#f59e0b',
      bgColor: 'bg-amber-50',
      textColor: 'text-amber-600',
      data: [2, 5, 3, 8, 4, 6, stats.todayCount || 7],
    },
  ]

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="w-8 h-8 animate-spin text-primary-500" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900 tracking-tight">仪表盘</h1>
          <p className="text-sm text-gray-500 mt-1">安全态势总览与实时监控</p>
        </div>
        <button onClick={handleRefresh} className="btn-default" disabled={refreshing}>
          <RefreshCw className={cn('w-4 h-4', refreshing && 'animate-spin')} />
          刷新
        </button>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {statsConfig.map((stat) => (
          <div key={stat.label} className="card card-body flex flex-col gap-3">
            <div className="flex items-center justify-between">
              <div className={cn('w-9 h-9 rounded-xl flex items-center justify-center', stat.bgColor)}>
                <stat.icon className={cn('w-4.5 h-4.5', stat.textColor)} style={{ width: 18, height: 18 }} />
              </div>
              <Sparkline data={stat.data} color={stat.color} />
            </div>
            <div>
              <p className="text-3xl font-bold text-gray-900 tracking-tight leading-none">{stat.value}</p>
              <div className="flex items-center justify-between mt-1">
                <p className="text-sm text-gray-500">{stat.label}</p>
                <p className="text-xs text-gray-400">{stat.sub}</p>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Security Funnel */}
      <SecurityFunnel
        collected={stats.eventCount}
        todayCount={stats.todayCount}
        critical={stats.criticalCount}
      />

      {/* Action Queue - 全宽待办任务 */}
      <ActionQueue criticalCount={stats.criticalCount} pendingReports={stats.reportCount} />

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div className="lg:col-span-2 card">
          <div className="card-header flex items-center justify-between">
            <span>事件趋势</span>
            <span className="text-xs text-gray-400 font-normal">最近 30 天</span>
          </div>
          <div className="card-body pt-0">
            <EventTrendChart />
          </div>
        </div>
        <div className="card">
          <div className="card-header">严重级别分布</div>
          <div className="card-body pt-0">
            <SeverityDistribution />
          </div>
        </div>
      </div>

      {/* Bottom Row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <div className="card">
          <div className="card-header flex items-center justify-between">
            <span>最新事件</span>
            <a href="/events" className="text-sm text-gray-500 hover:text-gray-900 font-normal transition-colors">
              查看全部 →
            </a>
          </div>
          <div className="p-0">
            <RecentEvents />
          </div>
        </div>
        <div className="card">
          <div className="card-header flex items-center justify-between">
            <span>订阅状态</span>
            <a href="/subscriptions" className="text-sm text-gray-500 hover:text-gray-900 font-normal transition-colors">
              管理订阅 →
            </a>
          </div>
          <div className="p-0">
            <SubscriptionStatus />
          </div>
        </div>
      </div>
    </div>
  )
}

