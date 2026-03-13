import { X, Download, ExternalLink, Copy } from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import { cn, normalizeMarkdown } from '@/utils'

interface EventDetail {
  id: number
  event_id: string
  title: string
  desc: string
  cve_id: string
  cvss: number
  severity: string
  vendor: string
  product: string
  source: string
  source_url: string
  recommendation?: string
  recommendationComplete?: boolean
}

interface Props {
  visible: boolean
  onClose: () => void
  data: { count: number; maxCVSS: number; avgRisk: number; critical?: number; highRisk?: number; events?: EventDetail[] } | null
  logs: { agent: string; message: string; status?: string; timestamp?: string }[]
  analysisText: string
}

const severityLabel: Record<string, string> = {
  critical: '严重', high: '高危', medium: '中危', low: '低危', info: '信息',
}

const severityClass: Record<string, string> = {
  critical: 'severity-critical',
  high: 'severity-high',
  medium: 'severity-medium',
  low: 'severity-low',
}

// 按严重程度排序的分组顺序
const SEVERITY_ORDER = ['critical', 'high', 'medium', 'low', 'info']

function groupBySeverity(events: EventDetail[]): Record<string, EventDetail[]> {
  const groups: Record<string, EventDetail[]> = {}
  for (const ev of events) {
    const key = ev.severity || 'info'
    if (!groups[key]) groups[key] = []
    groups[key].push(ev)
  }
  return groups
}

// 构建报告 Markdown 正文（遵循 NIST SP 800-61r3 格式）
export function buildMarkdown(
  data: NonNullable<Props['data']>,
  logs: Props['logs'],
  analysisText: string,
) {
  const now = new Date()
  const nowStr = now.toLocaleString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  })
  const reportId = `REPORT-${now.getFullYear()}${String(now.getMonth() + 1).padStart(2, '0')}${String(now.getDate()).padStart(2, '0')}-${String(now.getHours()).padStart(2, '0')}${String(now.getMinutes()).padStart(2, '0')}`
  const urgency = data.maxCVSS >= 9 ? '4小时内（P1）' : data.maxCVSS >= 7 ? '24小时内（P2）' : '72小时内（P3）'

  const events = data.events || []
  const groups = groupBySeverity(events)
  const criticalEvents = groups['critical'] || []
  const highEvents = groups['high'] || []
  const mediumEvents = groups['medium'] || []
  const lowEvents = [...(groups['low'] || []), ...(groups['info'] || [])]

  // ── 第二节：AI 解决方案（所有已生成方案的事件，按严重程度分组）──────
  const eventsWithSolution = events.filter(e => e.recommendationComplete && e.recommendation)
  const deepAnalysisSection = eventsWithSolution.length > 0
    ? SEVERITY_ORDER.flatMap(sev => {
        const sevEvts = eventsWithSolution.filter(e => e.severity === sev)
        if (sevEvts.length === 0) return []
        const label = severityLabel[sev] || sev
        return sevEvts.map((e, i) => [
          `### [${label.toUpperCase()}] ${e.title}`,
          '',
          `- **CVE：** ${e.cve_id || '暂无'} | **CVSS：** ${e.cvss || '-'} | **来源：** ${e.source || e.vendor || '-'}`,
          e.source_url ? `- **参考链接：** ${e.source_url}` : '',
          '',
          '**AI 应急处置方案：**',
          '',
          e.recommendation || '',
        ].filter(l => l !== undefined).join('\n'))
      }).join('\n\n')
    : '_当前无已完成的 AI 解决方案。_'

  // ── 第四节：完整事件清单（按严重程度分组）────────────────────────────
  const buildTable = (evts: EventDetail[]) => {
    if (evts.length === 0) return ''
    return [
      '| # | 标题 | CVSS | CVE | 来源 | 修复方案 |',
      '|---|------|------|-----|------|---------|',
      ...evts.map((e, i) =>
        `| ${i + 1} | ${e.title} | ${e.cvss || '-'} | ${e.cve_id || '-'} | ${e.source || e.vendor || '-'} | ${e.recommendationComplete ? '✅ 已生成' : '-'} |`
      ),
    ].join('\n')
  }

  const eventListSection = [
    criticalEvents.length > 0 ? `### 严重漏洞（P1 - 4小时内响应）\n\n${buildTable(criticalEvents)}` : '',
    highEvents.length > 0 ? `### 高危漏洞（P2 - 24小时内响应）\n\n${buildTable(highEvents)}` : '',
    mediumEvents.length > 0 ? `### 中危漏洞（P3 - 72小时内响应）\n\n${buildTable(mediumEvents)}` : '',
    lowEvents.length > 0 ? `### 低危 / 信息（P4 - 计划处理）\n\n${buildTable(lowEvents)}` : '',
  ].filter(Boolean).join('\n\n') || '暂无事件数据'

  // ── 第五节：分阶段修复建议 ────────────────────────────────────────────
  const p1List = criticalEvents.map(e => `- [ ] ${e.title}${e.cve_id ? ` (${e.cve_id})` : ''}`).join('\n') || '- 无'
  const p2List = highEvents.map(e => `- [ ] ${e.title}${e.cve_id ? ` (${e.cve_id})` : ''}`).join('\n') || '- 无'
  const p3List = mediumEvents.map(e => `- [ ] ${e.title}`).join('\n') || '- 无'

  // ── 第六节：Agent 执行日志 ────────────────────────────────────────────
  const agentTrace = logs.length > 0
    ? [
        '| 时间 | Agent | 状态 | 消息 |',
        '|------|-------|------|------|',
        ...logs.map(log => {
          const icon = log.status === 'success' ? '✅' : log.status === 'running' ? '⏳' : log.status === 'error' ? '❌' : '▸'
          const timeStr = log.timestamp ? new Date(log.timestamp).toLocaleTimeString('zh-CN') : '-'
          return `| ${timeStr} | ${log.agent} | ${icon} | ${log.message} |`
        }),
      ].join('\n')
    : '| - | - | - | 暂无轨迹记录 |'

  return `# 安全事件分析报告

**报告编号：** ${reportId} | **生成时间：** ${nowStr} | **分析范围：** ${data.count} 个事件

---

## 一、执行摘要

| 指标 | 值 |
|------|-----|
| 分析事件总数 | ${data.count} 个 |
| 严重漏洞（P1）| ${data.critical ?? 0} 个 |
| 高危漏洞（P2）| ${data.highRisk ?? 0} 个 |
| 中危漏洞（P3）| ${(groups['medium'] || []).length} 个 |
| 最高 CVSS 评分 | ${data.maxCVSS} |
| 建议优先响应时间 | ${urgency} |

---

## 二、AI 解决方案

${deepAnalysisSection}

---

## 三、完整事件清单

${eventListSection}

---

## 四、分阶段修复建议

**P1 - 4小时内完成（严重漏洞）：**

${p1List}

**P2 - 24小时内完成（高危漏洞）：**

${p2List}

**P3 - 72小时内完成（中危漏洞）：**

${p3List}

---

## 五、Agent 执行日志

${agentTrace}

---

## 参考规范

NIST SP 800-61r3 | CVSS v3.1 | CWE Top 25 | ISO/IEC 27035

> 本报告有效期 30 天，请及时归档。`
}

export default function ReportModal({ visible, onClose, data, logs, analysisText }: Props) {
  if (!visible || !data) return null

  const events = data.events || []
  const groups = groupBySeverity(events)
  const urgency = data.maxCVSS >= 9 ? '4小时内（P1）' : data.maxCVSS >= 7 ? '24小时内（P2）' : '72小时内（P3）'
  const now = new Date()
  const dateStr = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
  const timeStr = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`
  const reportId = `REPORT-${dateStr.replace(/-/g, '')}-${timeStr.replace(':', '')}`

  const handleDownload = () => {
    const md = buildMarkdown(data, logs, analysisText)
    const blob = new Blob([md], { type: 'text/markdown' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `安全分析报告_${dateStr}_${timeStr.replace(':', '')}.md`
    a.click()
    URL.revokeObjectURL(url)
  }

  const handleCopy = () => {
    const md = buildMarkdown(data, logs, analysisText)
    navigator.clipboard.writeText(md).catch(() => {/* 静默 */})
  }

  return (
    <div className="fixed inset-0 bg-black/40 flex items-center justify-center z-50" onClick={onClose}>
      <div
        className="bg-white rounded-2xl border border-gray-200 shadow-xl w-[900px] max-h-[90vh] flex flex-col"
        onClick={e => e.stopPropagation()}
      >
        {/* 头部 */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
          <div>
            <h2 className="text-base font-semibold text-gray-900">安全事件分析报告</h2>
            <p className="text-xs text-gray-400 mt-0.5">{reportId} · 共分析 {data.count} 个事件 · 报告有效期 30 天</p>
          </div>
          <button onClick={onClose} className="p-1.5 rounded text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* 内容区 */}
        <div className="flex-1 overflow-auto px-6 py-4 space-y-5">

          {/* 一、执行摘要 */}
          <div>
            <div className="text-sm font-semibold text-gray-800 mb-2">一、执行摘要</div>
            <div className={cn(
              'p-4 rounded-xl border grid grid-cols-2 sm:grid-cols-4 gap-3',
              data.maxCVSS >= 9 ? 'bg-red-50 border-red-200' : data.maxCVSS >= 7 ? 'bg-orange-50 border-orange-200' : 'bg-blue-50 border-blue-200'
            )}>
              {[
                { label: '分析事件', value: `${data.count} 个` },
                { label: '严重漏洞（P1）', value: `${data.critical ?? 0} 个`, highlight: (data.critical ?? 0) > 0 },
                { label: '高危漏洞（P2）', value: `${data.highRisk ?? 0} 个`, highlight: (data.highRisk ?? 0) > 0 },
                { label: '建议响应', value: urgency },
              ].map(({ label, value, highlight }) => (
                <div key={label} className="text-center">
                  <div className="text-xs text-gray-500 mb-1">{label}</div>
                  <div className={cn('text-sm font-bold', highlight ? 'text-red-600' : 'text-gray-800')}>{value}</div>
                </div>
              ))}
            </div>
          </div>

          {/* 二、AI 解决方案 */}
          {(() => {
            const eventsWithSolution = events.filter(e => e.recommendationComplete && e.recommendation)
            const noSolutionKeyEvents = [...(groups['critical'] || []), ...(groups['high'] || [])].filter(e => !e.recommendationComplete)
            if (eventsWithSolution.length === 0 && noSolutionKeyEvents.length === 0) return null
            return (
              <div>
                <div className="text-sm font-semibold text-gray-800 mb-2">
                  二、AI 解决方案
                  <span className="ml-2 text-xs font-normal text-gray-400">（{eventsWithSolution.length} 个事件已生成方案）</span>
                </div>
                <div className="space-y-3">
                  {/* 已生成方案的事件，按严重程度顺序展示 */}
                  {SEVERITY_ORDER.flatMap(sev => {
                    const sevEvts = eventsWithSolution.filter(e => e.severity === sev)
                    return sevEvts.map((e) => (
                      <div key={e.event_id} className="rounded-xl border border-gray-200 overflow-hidden">
                        <div className={cn(
                          'px-4 py-2.5 flex items-center gap-2 border-b border-gray-100',
                          e.severity === 'critical' ? 'bg-red-50' :
                          e.severity === 'high' ? 'bg-orange-50' :
                          e.severity === 'medium' ? 'bg-yellow-50' : 'bg-gray-50'
                        )}>
                          <span className={severityClass[e.severity] || 'tag-default'}>
                            {severityLabel[e.severity] || e.severity}
                          </span>
                          <span className="text-sm font-medium text-gray-800 flex-1 truncate">{e.title}</span>
                          {e.cvss && <span className="text-xs text-gray-500 shrink-0">CVSS {e.cvss}</span>}
                          {e.cve_id && <span className="text-xs font-mono text-gray-400 shrink-0">{e.cve_id}</span>}
                          {e.source_url && (
                            <a href={e.source_url} target="_blank" rel="noopener noreferrer"
                              className="text-gray-300 hover:text-primary-500 transition-colors shrink-0">
                              <ExternalLink className="w-3.5 h-3.5" />
                            </a>
                          )}
                        </div>
                        <div className="px-4 py-3">
                          <div className="text-xs text-gray-500 font-medium mb-2">AI 应急处置方案</div>
                          <div className="prose prose-sm max-w-none text-xs text-gray-700 leading-relaxed">
                            <ReactMarkdown>{normalizeMarkdown(e.recommendation || '')}</ReactMarkdown>
                          </div>
                        </div>
                      </div>
                    ))
                  })}
                  {/* 严重/高危但尚无方案（保底显示） */}
                  {noSolutionKeyEvents.map((e) => (
                    <div key={e.event_id} className="rounded-xl border border-gray-200 overflow-hidden opacity-60">
                      <div className={cn(
                        'px-4 py-2.5 flex items-center gap-2',
                        e.severity === 'critical' ? 'bg-red-50' : 'bg-orange-50'
                      )}>
                        <span className={severityClass[e.severity] || 'tag-default'}>
                          {severityLabel[e.severity] || e.severity}
                        </span>
                        <span className="text-sm font-medium text-gray-800 flex-1 truncate">{e.title}</span>
                        <span className="text-xs text-gray-400 italic shrink-0">方案未生成</span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )
          })()}

          {/* 三、完整事件清单（按严重程度分组） */}
          {events.length > 0 && (
            <div>
              <div className="text-sm font-semibold text-gray-800 mb-2">三、完整事件清单</div>
              <div className="space-y-3">
                {SEVERITY_ORDER.map(sev => {
                  const grpEvents = groups[sev]
                  if (!grpEvents?.length) return null
                  const grpLabel = sev === 'critical' ? '严重漏洞（P1）'
                    : sev === 'high' ? '高危漏洞（P2）'
                    : sev === 'medium' ? '中危漏洞（P3）'
                    : '低危 / 信息（P4）'
                  return (
                    <div key={sev}>
                      <div className="text-xs font-medium text-gray-500 mb-1.5">{grpLabel}</div>
                      <div className="rounded-xl border border-gray-200 overflow-hidden">
                        <table className="w-full text-xs">
                          <thead className="bg-gray-50 border-b border-gray-200">
                            <tr>
                              <th className="px-3 py-2 text-left text-gray-500 font-medium w-7">#</th>
                              <th className="px-3 py-2 text-left text-gray-500 font-medium">标题</th>
                              <th className="px-3 py-2 text-left text-gray-500 font-medium w-12">CVSS</th>
                              <th className="px-3 py-2 text-left text-gray-500 font-medium w-28">CVE</th>
                              <th className="px-3 py-2 text-left text-gray-500 font-medium w-20">来源</th>
                              <th className="px-3 py-2 text-left text-gray-500 font-medium w-20">修复方案</th>
                              <th className="px-3 py-2 w-6"></th>
                            </tr>
                          </thead>
                          <tbody className="divide-y divide-gray-100">
                            {grpEvents.map((e, i) => (
                              <tr key={e.id} className="hover:bg-gray-50">
                                <td className="px-3 py-2 text-gray-400">{i + 1}</td>
                                <td className="px-3 py-2 max-w-[220px]">
                                  <span className="truncate block text-gray-700">{e.title}</span>
                                </td>
                                <td className="px-3 py-2 text-gray-600">{e.cvss || '-'}</td>
                                <td className="px-3 py-2 font-mono text-gray-500 truncate">{e.cve_id || '-'}</td>
                                <td className="px-3 py-2 text-gray-500 truncate">{e.source || e.vendor || '-'}</td>
                                <td className="px-3 py-2">
                                  {e.recommendationComplete
                                    ? <span className="text-green-600 font-medium">✅ 已生成</span>
                                    : <span className="text-gray-300">-</span>
                                  }
                                </td>
                                <td className="px-3 py-2">
                                  {e.source_url && (
                                    <a href={e.source_url} target="_blank" rel="noopener noreferrer"
                                      className="text-gray-300 hover:text-primary-500 transition-colors">
                                      <ExternalLink className="w-3 h-3" />
                                    </a>
                                  )}
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          )}

          {/* 四、Agent 执行日志 */}
          {logs.length > 0 && (
            <div>
              <div className="text-sm font-semibold text-gray-800 mb-2">四、Agent 执行日志</div>
              <div className="rounded-xl border border-gray-200 overflow-hidden">
                <table className="w-full text-xs">
                  <thead className="bg-gray-50 border-b border-gray-200">
                    <tr>
                      <th className="px-3 py-2 text-left text-gray-500 font-medium w-20">时间</th>
                      <th className="px-3 py-2 text-left text-gray-500 font-medium w-28">Agent</th>
                      <th className="px-3 py-2 text-left text-gray-500 font-medium w-12">状态</th>
                      <th className="px-3 py-2 text-left text-gray-500 font-medium">消息</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-100">
                    {logs.map((log, i) => {
                      const icon = log.status === 'success' ? '✅' : log.status === 'running' ? '⏳' : log.status === 'error' ? '❌' : '▸'
                      const timeStr = log.timestamp ? new Date(log.timestamp).toLocaleTimeString('zh-CN') : '-'
                      return (
                        <tr key={i} className="hover:bg-gray-50">
                          <td className="px-3 py-2 text-gray-400 font-mono whitespace-nowrap">{timeStr}</td>
                          <td className="px-3 py-2 text-gray-600 font-medium">{log.agent}</td>
                          <td className="px-3 py-2 text-center">{icon}</td>
                          <td className="px-3 py-2 text-gray-600">{log.message}</td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* 参考规范 */}
          <div className="text-xs text-gray-400 border-t border-gray-100 pt-3">
            参考规范：NIST SP 800-61r3 | CVSS v3.1 | CWE Top 25 | ISO/IEC 27035 &nbsp;·&nbsp; 本报告有效期 30 天
          </div>
        </div>

        {/* 底部操作 */}
        <div className="px-6 py-4 border-t border-gray-100 flex items-center justify-between">
          <button onClick={onClose} className="btn-default">关闭</button>
          <div className="flex items-center gap-2">
            <button onClick={handleCopy} className="btn-default">
              <Copy className="w-4 h-4" />
              复制内容
            </button>
            <button onClick={handleDownload} className="btn-default">
              <Download className="w-4 h-4" />
              下载 Markdown
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
