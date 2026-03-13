import { useState, useEffect, useRef } from 'react'
import { FileText, BrainCircuit, RotateCcw } from 'lucide-react'
import { useLocation } from 'react-router-dom'
import { useEventStore } from '@/stores/eventStore'
import { useAnalyzeStore } from '@/stores/analyzeStore'
import AgentFlowGraph from './components/AgentFlowGraph'
import ThinkingConsole from './components/ThinkingConsole'
import StatsBar from './components/StatsBar'
import StartButton from './components/StartButton'
import ResultPanel from './components/ResultPanel'
import AnalysisModeSelect, { type AnalysisMode } from './components/AnalysisModeSelect'
import EventPickerModal from './components/EventPickerModal'
import { eventService } from '@/services/event'

export default function EventAnalysis() {
  const { clearLogs, isProcessing, setProcessing, agentLogs, addLog, terminateRunningLogs } = useEventStore()
  const { riskData, analysisText, savedAt, setResult, clearResult, updateEventSolution, setReportGenerating } = useAnalyzeStore()
  const location = useLocation()
  const abortControllerRef = useRef<AbortController | null>(null)
  const [analysisMode, setAnalysisMode] = useState<AnalysisMode>(() => {
    return location.state?.mode === 'specific' ? 'specific' : 'today'
  })
  const [selectedEventIds, setSelectedEventIds] = useState<string[]>(() => {
    return location.state?.preSelectedIds ?? []
  })
  const [showEventPicker, setShowEventPicker] = useState(false)

  // 从事件列表页跳转时，若携带了预选事件则自动读取（仅首次挂载）
  useEffect(() => {
    if (location.state?.mode === 'specific' && Array.isArray(location.state?.preSelectedIds) && location.state.preSelectedIds.length > 0) {
      setAnalysisMode('specific')
      setSelectedEventIds(location.state.preSelectedIds)
    }
  }, [])

  // ── 模式与选择 ────────────────────────────────────────────────────────────────
  const handleModeChange = (mode: AnalysisMode) => {
    setAnalysisMode(mode)
    if (mode === 'specific') {
      setShowEventPicker(true)
    }
  }

  const handleSolutionUpdate = (eventId: string, content: string, complete: boolean) => {
    if (!complete) return
    updateEventSolution(eventId, content)
  }

  // ── 分析主流程 ────────────────────────────────────────────────────────────────
  const startAnalysis = async () => {
    if (analysisMode === 'specific' && selectedEventIds.length === 0) {
      setShowEventPicker(true)
      return
    }
    clearLogs()
    setProcessing(true)
    clearResult()

    const localLogs: typeof agentLogs = []
    const addLocalLog = (log: Parameters<typeof addLog>[0]) => {
      addLog(log)
      localLogs.push(log)
    }

    try {
      const statsResult = await eventService.getStats()
      const realCritical = statsResult.by_severity?.critical || 0
      const realHigh = statsResult.by_severity?.high || 0
      const realTotal = statsResult.total || 0

      const mapEvent = (e: { id: string; title: string; severity: string; source?: string; source_url?: string; cve_id?: string; cvss_score?: number }, idx: number) => ({
        id: idx + 1,
        event_id: e.id,
        title: e.title,
        desc: '',
        cve_id: e.cve_id || '',
        cvss: e.cvss_score || (e.severity === 'critical' ? 9.0 : e.severity === 'high' ? 7.0 : 5.0),
        severity: e.severity,
        vendor: '',
        product: '',
        source: e.source || '',
        source_url: e.source_url || '',
      })

      let severeEvents: ReturnType<typeof mapEvent>[]
      if (analysisMode === 'specific') {
        const allRes = await eventService.list({ size: 100 })
        severeEvents = allRes.list
          .filter(e => selectedEventIds.includes(e.id))
          .map((e, idx) => mapEvent(e, idx))
      } else {
        const [critResult, highResult] = await Promise.all([
          eventService.list({ severity: 'critical', size: 20 }),
          eventService.list({ severity: 'high', size: 10 }),
        ])
        severeEvents = [...critResult.list, ...highResult.list].slice(0, 30).map((e, idx) => mapEvent(e, idx))
      }

      const dispCritical = analysisMode === 'specific'
        ? severeEvents.filter(e => e.severity === 'critical').length
        : realCritical
      const dispHigh = analysisMode === 'specific'
        ? severeEvents.filter(e => e.severity === 'high').length
        : realHigh
      const dispTotal = analysisMode === 'specific' ? severeEvents.length : realTotal
      const dispMaxCVSS = dispCritical > 0 ? 9.0 : dispHigh > 0 ? 7.0 : 5.0

      const buildSpecificQuery = () => {
        if (severeEvents.length === 0) return `分析以下事件 ID：${selectedEventIds.join(', ')}`
        const titles = severeEvents.map(e => e.title).join('；')
        return `分析以下安全事件（共 ${severeEvents.length} 条）：${titles}。请评估这些事件的威胁态势并给出处置建议。`
      }
      const query = analysisMode === 'latest'
        ? '分析最新10条安全事件，重点关注高危漏洞'
        : analysisMode === 'today'
          ? '分析今日新增安全事件，评估当前威胁态势'
          : buildSpecificQuery()

      const controller = new AbortController()
      abortControllerRef.current = controller

      const response = await fetch('/api/event/v1/pipeline/stream', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query }),
        signal: controller.signal,
      })
      if (!response.ok) throw new Error(`HTTP ${response.status}`)
      const reader = response.body?.getReader()
      if (!reader) throw new Error('No reader')
      const decoder = new TextDecoder()
      let buffer = ''
      let fullText = ''

      const agentSequence = ['数据采集Agent', '提取Agent', '去重Agent', '风险评估Agent', '解决方案Agent']
      const thresholds = [1, 6, 14, 24, Infinity]
      let chunkCount = 0
      let currentStage = 0

      const stageStartMessages: Record<string, string> = {
        '提取Agent': `正在解析事件特征：CVE 编号、CVSS 评分、漏洞类型...`,
        '去重Agent': `正在对比事件指纹哈希，过滤重复数据...`,
        '风险评估Agent': `正在评估威胁等级：发现 ${dispCritical} 个严重漏洞，${dispHigh} 个高危漏洞`,
        '解决方案Agent': `正在综合 CVSS ${dispMaxCVSS} 分析生成修复建议...`,
      }
      const stageDoneMessages: Record<string, string> = {
        '数据采集Agent': `采集完成，共获取 ${dispTotal} 条安全事件`,
        '提取Agent': `特征提取完成，识别到 ${dispCritical + dispHigh} 条高危事件`,
        '去重Agent': `去重完成，有效事件 ${dispTotal} 条`,
        '风险评估Agent': `风险评估完成，最高 CVSS ${dispMaxCVSS}，需立即处置 ${dispCritical} 条`,
        '解决方案Agent': `分析完成，AI 研判结论已生成`,
      }

      const advanceTo = (nextStage: number) => {
        if (nextStage <= currentStage || nextStage >= agentSequence.length) return
        addLocalLog({ agent: agentSequence[currentStage], status: 'success', message: stageDoneMessages[agentSequence[currentStage]] || '处理完成', timestamp: new Date().toISOString() })
        currentStage = nextStage
        addLocalLog({ agent: agentSequence[nextStage], status: 'running', message: stageStartMessages[agentSequence[nextStage]] || '处理中...', timestamp: new Date().toISOString() })
      }

      const finalize = (text: string) => {
        for (let s = currentStage; s < agentSequence.length - 1; s++) {
          addLocalLog({ agent: agentSequence[s], status: 'success', message: stageDoneMessages[agentSequence[s]] || '处理完成', timestamp: new Date().toISOString() })
        }
        addLocalLog({ agent: '解决方案Agent', status: 'success', message: stageDoneMessages['解决方案Agent'], timestamp: new Date().toISOString() })
        setProcessing(false)
        setResult({
          riskData: { maxCVSS: dispMaxCVSS, count: dispTotal, avgRisk: 0, critical: dispCritical, highRisk: dispHigh, events: severeEvents },
          analysisText: text,
        })
      }

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''
        for (const line of lines) {
          if (!line.startsWith('data: ')) continue
          const raw = line.slice(6)
          if (raw === '[DONE]') { finalize(fullText); return }
          try {
            const data = JSON.parse(raw)
            if (data.type === 'content' && data.content) {
              fullText += data.content
              chunkCount++
              if (chunkCount === 1) {
                addLocalLog({ agent: '数据采集Agent', status: 'running', message: `正在采集安全事件数据，数据库共 ${dispTotal} 条记录...`, timestamp: new Date().toISOString() })
              }
              for (let s = 1; s < thresholds.length; s++) {
                if (chunkCount === thresholds[s] && s > currentStage) { advanceTo(s); break }
              }
            } else if (data.type === 'done') {
              finalize(fullText); return
            } else if (data.type === 'error') {
              addLocalLog({ agent: agentSequence[Math.min(currentStage, agentSequence.length - 1)], status: 'error', message: data.content || '分析失败', timestamp: new Date().toISOString() })
              setProcessing(false)
              return
            }
          } catch { /* 忽略解析错误 */ }
        }
      }
      finalize(fullText)
    } catch (err) {
      if (err instanceof Error && err.name !== 'AbortError') {
        console.error('[EventAnalysis] 分析失败:', err)
      }
      setProcessing(false)
    } finally {
      abortControllerRef.current = null
    }
  }

  const stopAnalysis = () => {
    abortControllerRef.current?.abort()
    abortControllerRef.current = null
    setProcessing(false)
    terminateRunningLogs()
  }

  return (    <div className="relative flex flex-col bg-white overflow-hidden -m-8" style={{ height: 'calc(100vh - 64px)' }}>
      {/* 顶部操作栏 */}
      <div className="px-6 py-3 border-b border-gray-200 flex items-center justify-between shrink-0">
        <div className="flex items-center gap-2.5">
          <BrainCircuit className="w-5 h-5 text-primary-500" />
          <span className="text-base font-semibold text-gray-900">多智能体协作研判</span>
          {savedAt && !isProcessing && (
            <span className="text-xs text-gray-400">
              上次分析：{new Date(savedAt).toLocaleString('zh-CN')}
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          {riskData && !isProcessing && (
            <>
              <button
                onClick={() => { clearResult(); clearLogs() }}
                className="btn-default text-xs text-gray-500 flex items-center gap-1.5"
                title="清除当前研判结果，重新分析"
              >
                <RotateCcw className="w-3.5 h-3.5" />
                清除结果
              </button>
              <button
                onClick={() => setReportGenerating(true)}
                className="btn-primary flex items-center gap-1.5"
              >
                <FileText className="w-4 h-4" />
                生成报告
              </button>
            </>
          )}
          <AnalysisModeSelect
            value={analysisMode}
            onChange={handleModeChange}
            disabled={isProcessing}
            selectedCount={analysisMode === 'specific' ? selectedEventIds.length : undefined}
          />
          <StartButton isProcessing={isProcessing} onStart={startAnalysis} onStop={stopAnalysis} hasData={!!riskData} />
        </div>
      </div>

      {/* 数据统计横条 */}
      <div className="px-4 py-2 border-b border-gray-100 bg-gray-50 shrink-0">
        <StatsBar data={riskData} isProcessing={isProcessing} />
      </div>

      {/* 主内容区：左右两栏 */}
      <div className="flex-1 flex overflow-hidden">
        <div className="w-[960px] shrink-0 flex flex-col border-r border-gray-200 overflow-hidden">
          <div className="h-[260px] shrink-0 border-b border-gray-100">
            <AgentFlowGraph logs={agentLogs} isProcessing={isProcessing} />
          </div>
          <div className="flex-1 overflow-hidden p-3">
            <ThinkingConsole logs={agentLogs} isProcessing={isProcessing} />
          </div>
        </div>
        <div className="flex-1 overflow-hidden">
          <ResultPanel data={riskData} isProcessing={isProcessing} onSolutionUpdate={handleSolutionUpdate} />
        </div>
      </div>

      {/* 事件选择弹窗 */}
      <EventPickerModal
        visible={showEventPicker}
        onClose={() => setShowEventPicker(false)}
        onConfirm={(ids) => setSelectedEventIds(ids)}
        selectedIds={selectedEventIds}
      />
    </div>
  )
}
