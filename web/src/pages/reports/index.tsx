import { useState, useEffect } from 'react'
import {
  Search,
  FileText,
  Download,
  Trash2,
  Eye,
  RefreshCw,
  Loader2,
} from 'lucide-react'
import { cn, formatDate } from '@/utils'
import Pagination from '@/components/common/Pagination'
import ReportDetailModal from './components/ReportDetailModal'
import { reportService } from '@/services/report'
import type { Report } from '@/types'
import toast from 'react-hot-toast'

const reportTypeConfig: Record<string, { label: string; class: string }> = {
  vuln_alert: { label: '漏洞告警', class: 'tag-danger' },
  weekly: { label: '周报', class: 'tag-success' },
  custom: { label: '分析报告', class: 'tag-default' },
}

const statusConfig: Record<string, { label: string; class: string }> = {
  pending: { label: '待生成', class: 'tag-default' },
  generating: { label: '生成中', class: 'tag-warning' },
  completed: { label: '已完成', class: 'tag-success' },
  failed: { label: '失败', class: 'tag-danger' },
}

export default function Reports() {
  const [searchQuery, setSearchQuery] = useState('')
  const [typeFilter, setTypeFilter] = useState('all')
  const [selectedReport, setSelectedReport] = useState<Report | null>(null)

  const [reports, setReports] = useState<Report[]>([])
  const [loading, setLoading] = useState(true)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize] = useState(20)

  const fetchReports = async () => {
    try {
      setLoading(true)
      const res = await reportService.list(page, pageSize, typeFilter)
      setReports(res.list || [])
      setTotal(res.total || 0)
    } catch (error) {
      console.error('[Reports] 获取报告列表失败:', error)
      setReports([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchReports()
  }, [page, typeFilter])

  // 当列表中存在 generating 状态的报告时，每 5s 自动刷新
  useEffect(() => {
    const hasGenerating = reports.some(r => r.status === 'generating')
    if (!hasGenerating) return

    const interval = setInterval(() => {
      fetchReports()
    }, 5000)

    return () => clearInterval(interval)
  }, [reports])

  const handleDelete = async (id: string) => {
    if (!confirm('确定要删除这个报告吗？')) return
    try {
      await reportService.delete(id)
      toast.success('已删除报告')
      fetchReports()
    } catch {
      toast.error('删除失败')
    }
  }

  const handleDownload = (report: Report) => {
    const content = report.content || `# ${report.title}\n\n（报告内容为空）`
    const blob = new Blob([content], { type: 'text/markdown' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${report.title}.md`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  const handleViewReport = (report: Report) => {
    setSelectedReport(report)
  }

  const filteredReports = reports.filter((report) => {
    return report.title.toLowerCase().includes(searchQuery.toLowerCase())
  })

  const totalPages = Math.ceil(total / pageSize)

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900 tracking-tight">分析报告</h1>
          <p className="text-sm text-gray-500 mt-1">AI Agent 生成的安全分析报告存档</p>
        </div>
      </div>

      {/* Filters */}
      <div className="card card-body">
        <div className="flex flex-col sm:flex-row gap-4">
          {/* Search */}
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
            <input
              type="text"
              placeholder="搜索报告标题..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="input pl-9"
            />
          </div>

          {/* Type Filter */}
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            className="select w-36"
          >
            <option value="all">全部类型</option>
            <option value="vuln_alert">漏洞告警</option>
            <option value="weekly">周报</option>
            <option value="custom">分析报告</option>
          </select>

          {/* Refresh Button */}
          <button
            onClick={fetchReports}
            disabled={loading}
            className="btn-default"
          >
            <RefreshCw className={cn('w-4 h-4', loading && 'animate-spin')} />
            刷新
          </button>
        </div>
      </div>

      {/* Report Table */}
      <div className="card">
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-8 h-8 animate-spin text-primary-500" />
          </div>
        ) : (
          <div className="table-container">
            <table className="table w-full [&_th]:text-sm [&_th]:normal-case [&_th]:tracking-normal [&_th]:font-semibold [&_td]:text-sm" style={{ tableLayout: 'fixed' }}>
              <colgroup>
                <col />
                <col style={{ width: '96px' }} />
                <col style={{ width: '80px' }} />
                <col style={{ width: '140px' }} />
                <col style={{ width: '108px' }} />
              </colgroup>
              <thead>
                <tr>
                  <th>报告标题</th>
                  <th className="w-24">类型</th>
                  <th className="w-20">状态</th>
                  <th className="w-32">创建时间</th>
                  <th className="w-24">操作</th>
                </tr>
              </thead>
              <tbody>
                {filteredReports.map((report) => {
                  const typeConfig = reportTypeConfig[report.type] || reportTypeConfig.custom
                  const stConfig = statusConfig[report.status] || statusConfig.pending
                  return (
                    <tr
                      key={report.id}
                      className="cursor-pointer"
                      onClick={() => handleViewReport(report)}
                    >
                      <td className="max-w-0">
                        <div className="min-w-0">
                          <p className="font-medium text-gray-900 truncate">{report.title}</p>
                          {report.summary && (
                            <p className="text-xs text-gray-500 truncate mt-0.5">{report.summary}</p>
                          )}
                        </div>
                      </td>
                      <td className="whitespace-nowrap">
                        <span className={cn('tag', typeConfig.class)}>{typeConfig.label}</span>
                      </td>
                      <td className="whitespace-nowrap">
                        <span className={cn('tag', stConfig.class)}>
                          {report.status === 'generating' && <Loader2 className="w-3 h-3 animate-spin mr-1" />}
                          {stConfig.label}
                        </span>
                      </td>
                      <td className="text-gray-600 whitespace-nowrap">
                        {formatDate(report.created_at, 'YYYY-MM-DD HH:mm')}
                      </td>
                      <td onClick={(e) => e.stopPropagation()}>
                        <div className="flex items-center gap-1">
                          <button
                            onClick={() => handleViewReport(report)}
                            className="p-1.5 rounded text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors"
                            title="查看"
                          >
                            <Eye className="w-3.5 h-3.5" />
                          </button>
                          <button
                            onClick={() => handleDownload(report)}
                            className="p-1.5 rounded text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors"
                            title="下载 Markdown"
                          >
                            <Download className="w-3.5 h-3.5" />
                          </button>
                          <button
                            onClick={() => handleDelete(report.id)}
                            className="p-1.5 rounded text-gray-300 hover:text-red-500 hover:bg-red-50 transition-colors"
                            title="删除"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}

        {!loading && filteredReports.length === 0 && (
          <div className="empty">
            <FileText className="empty-icon" />
            <p className="empty-text">暂无报告</p>
            <p className="text-xs text-gray-400 mt-1">前往 <strong>Agent 分析</strong> 页面执行分析后，点击"保存到报告库"即可</p>
          </div>
        )}

        {/* Pagination */}
        {!loading && filteredReports.length > 0 && (
          <Pagination
            page={page}
            totalPages={totalPages}
            total={total}
            onChange={setPage}
          />
        )}
      </div>

      {/* Report Detail Modal */}
      <ReportDetailModal
        report={selectedReport}
        onClose={() => setSelectedReport(null)}
      />
    </div>
  )
}
