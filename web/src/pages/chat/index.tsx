import { useState, useRef, useEffect } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { ChevronDown, ChevronRight, Loader2, ListChecks, ThumbsUp, ThumbsDown } from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { normalizeMarkdown, cn } from '@/utils'
import { chatService, ChatSession } from '@/services'
import { ragevalService } from '@/services/rageval'
import { useContextStore } from '@/stores/contextStore'
import SessionList from './components/SessionList'
import WelcomeScreen from './components/WelcomeScreen'
import ChatInput from './components/ChatInput'

type MessageRole = 'user' | 'assistant'

interface Message {
  id: string
  role: MessageRole
  content: string
  timestamp: Date
  isStreaming?: boolean
  agentStatus?: string
  // Plan Agent 规划过程字段
  planning?: string          // 中间步骤内容（plan_step 事件累积）
  isPlanRunning?: boolean    // true = 规划执行中
  planDone?: boolean         // true = 规划结束，最终答案已到达
  planStartAt?: number       // 首个 plan_step 到达时的时间戳
  planDuration?: number      // 规划用时（秒）
}

export default function Chat() {
  const [sessions, setSessions] = useState<ChatSession[]>([])
  const [currentSessionId, setCurrentSessionId] = useState('')
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [deepThinking, setDeepThinking] = useState(() => localStorage.getItem('chat_deep_thinking') === 'true')
  const [webSearch, setWebSearch] = useState(() => localStorage.getItem('chat_web_search') === 'true')
  // 记录每条消息的点赞/踩状态（key = message index）
  const [votes, setVotes] = useState<Record<number, 1 | -1>>({})

  const chatScrollRef = useRef<HTMLDivElement>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const streamingContentRef = useRef('')
  const abortControllerRef = useRef<AbortController | null>(null)
  // 记录当前"正在 loading"的消息 ID，防止旧 stream 的 onDone 错误清除新 stream 的 loading 状态
  const loadingStreamRef = useRef<string | null>(null)

  const { currentEventId, currentEventTitle } = useContextStore()
  const navigate = useNavigate()
  const location = useLocation()

  // ── 组件卸载时不 abort（让 stream 在后台跑完，onDone 会写入 localStorage）
  useEffect(() => {
    return () => { /* no abort: background stream saves result on completion */ }
  }, [])

  // ── 初始化 ───────────────────────────────────────────────────────────────
  useEffect(() => {
    const loaded = chatService.listSessions()
    if (loaded.length === 0) {
      const s = chatService.createSession()
      setSessions([s])
      setCurrentSessionId(s.id)
    } else {
      setSessions(loaded)
      const sid = chatService.getSessionId()
      setCurrentSessionId(sid)
      loadMessages(sid)
    }
  }, [])

  useEffect(() => {
    if (chatScrollRef.current) {
      chatScrollRef.current.scrollTop = chatScrollRef.current.scrollHeight
    }
  }, [messages])

  // 从事件列表跳转：直接发送消息
  useEffect(() => {
    const state = location.state as { query?: string } | null
    if (!state?.query) return
    navigate(location.pathname, { replace: true, state: null })
    handleSend(state.query)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.state])

  // ── 会话管理 ─────────────────────────────────────────────────────────────
  const loadMessages = (sid: string) => {
    const raw = localStorage.getItem(`chat_messages_${sid}`)
    setMessages(raw ? JSON.parse(raw) : [])
    // 恢复输入框草稿
    setInput(localStorage.getItem(`chat_draft_${sid}`) ?? '')
    // 恢复点赞/踩状态
    const votesRaw = localStorage.getItem(`chat_votes_${sid}`)
    setVotes(votesRaw ? JSON.parse(votesRaw) : {})
  }

  const saveMessages = (sid: string, msgs: Message[]) => {
    localStorage.setItem(`chat_messages_${sid}`, JSON.stringify(msgs))
  }

  useEffect(() => {
    if (currentSessionId && messages.length > 0) {
      saveMessages(currentSessionId, messages)
    }
  }, [messages, currentSessionId])

  // 持久化深度思考 / 联网搜索开关（跨会话、跨刷新保持用户选择）
  useEffect(() => {
    localStorage.setItem('chat_deep_thinking', String(deepThinking))
  }, [deepThinking])

  useEffect(() => {
    localStorage.setItem('chat_web_search', String(webSearch))
  }, [webSearch])

  // 持久化输入框草稿
  useEffect(() => {
    if (!currentSessionId) return
    if (input) {
      localStorage.setItem(`chat_draft_${currentSessionId}`, input)
    } else {
      localStorage.removeItem(`chat_draft_${currentSessionId}`)
    }
  }, [input, currentSessionId])

  // 持久化点赞/踩状态
  useEffect(() => {
    if (!currentSessionId) return
    if (Object.keys(votes).length > 0) {
      localStorage.setItem(`chat_votes_${currentSessionId}`, JSON.stringify(votes))
    }
  }, [votes, currentSessionId])

  const handleSelectSession = (sid: string) => {
    // 将进行中的流式消息标为完成后保存，防止切换回来时看到"卡住"的 streaming 状态
    const finalMessages = messages.map(m =>
      m.isStreaming ? { ...m, isStreaming: false, agentStatus: undefined, isPlanRunning: false } : m
    )
    saveMessages(currentSessionId, finalMessages)
    loadingStreamRef.current = null  // 旧 stream 的 onDone 不再清除 loading
    setCurrentSessionId(sid)
    setIsLoading(false)
    chatService.switchSession(sid)
    loadMessages(sid)
  }

  const handleNewSession = () => {
    if (currentSessionId && messages.length === 0) return
    const finalMessages = messages.map(m =>
      m.isStreaming ? { ...m, isStreaming: false, agentStatus: undefined, isPlanRunning: false } : m
    )
    saveMessages(currentSessionId, finalMessages)
    loadingStreamRef.current = null
    const s = chatService.createSession()
    setSessions([s, ...sessions])
    setCurrentSessionId(s.id)
    setMessages([])
    setInput('')
    setVotes({})
    setIsLoading(false)
    setDeepThinking(false)
    setWebSearch(false)
  }

  const handleDeleteSession = (sid: string) => {
    chatService.deleteSession(sid)
    localStorage.removeItem(`chat_messages_${sid}`)
    localStorage.removeItem(`chat_draft_${sid}`)
    localStorage.removeItem(`chat_votes_${sid}`)
    const updated = sessions.filter((s) => s.id !== sid)
    setSessions(updated)
    if (sid === currentSessionId) {
      if (updated.length > 0) handleSelectSession(updated[0].id)
      else { setCurrentSessionId(''); setMessages([]) }
    }
  }

  // ── 发送消息（overrideText 来自技能卡片或事件跳转） ──────────────────────────
  const handleSend = async (overrideText?: string) => {
    const rawContent = overrideText !== undefined ? overrideText : input
    if (!rawContent.trim() || isLoading) return

    let sid = currentSessionId
    if (!sid) {
      const s = chatService.createSession()
      setSessions([s, ...sessions])
      setCurrentSessionId(s.id)
      sid = s.id
    }

    let messageContent = rawContent.trim()
    if (currentEventId && currentEventTitle) {
      messageContent = `[当前上下文: 事件#${currentEventId} "${currentEventTitle}"]\n${messageContent}`
    }

    const userMessage: Message = {
      id: genId(),
      role: 'user',
      content: rawContent.trim(),
      timestamp: new Date(),
    }
    setMessages((prev) => [...prev, userMessage])
    if (overrideText === undefined) setInput('')
    setIsLoading(true)

    // 更新会话标题
    if (messages.length === 0) {
      const title = rawContent.trim().slice(0, 30) + (rawContent.trim().length > 30 ? '...' : '')
      chatService.updateSession(sid, { title })
      setSessions((prev) => prev.map((s) => (s.id === sid ? { ...s, title } : s)))
    }
    chatService.updateSession(sid, { lastMessageAt: Date.now(), messageCount: messages.length + 1 })
    setSessions((prev) =>
      prev.map((s) =>
        s.id === sid ? { ...s, lastMessageAt: Date.now(), messageCount: messages.length + 1 } : s
      )
    )

    const assistantMessage: Message = {
      id: genId(),
      role: 'assistant',
      content: '',
      timestamp: new Date(),
      isStreaming: true,
    }
    setMessages((prev) => [...prev, assistantMessage])

    streamingContentRef.current = ''

    // 中止上一个请求（如有），新建控制器
    abortControllerRef.current?.abort()
    const abortCtrl = new AbortController()
    abortControllerRef.current = abortCtrl

    // 捕获此次 stream 归属（用于 onDone 写入正确的 localStorage key）
    const capturedSessionId = sid
    const capturedMessageId = assistantMessage.id
    loadingStreamRef.current = capturedMessageId

    const flushContent = () => {
      setMessages((prev) =>
        prev.map((m) =>
          m.id === assistantMessage.id ? { ...m, content: streamingContentRef.current } : m
        )
      )
    }

    try {
      // 始终走多智能体（意图识别路由），deepThinking 为深度思考模式
      chatService.multiAgentChat(
        messageContent,
        deepThinking,
        webSearch,
        (agent, content) => {
          if (agent === 'plan_step') {
            // Plan Agent 中间步骤：每次 onStep 推送的是完整的一段文字（非增量），
            // 用换行追加到 planning 字段，展示在规划过程折叠块中
            setMessages((prev) =>
              prev.map((m) =>
                m.id === assistantMessage.id
                  ? {
                      ...m,
                      planning: (m.planning ? m.planning + '\n\n' : '') + content,
                      isPlanRunning: true,
                      planStartAt: m.planStartAt ?? Date.now(),
                    }
                  : m
              )
            )
          } else if (agent === 'status') {
            setMessages((prev) =>
              prev.map((m) =>
                m.id === assistantMessage.id ? { ...m, agentStatus: content } : m
              )
            )
          } else if (agent === 'meta') {
            try {
              const meta = JSON.parse(content) as Record<string, string>
              if (meta.sessionId && !meta.deepThinking) {
                setMessages((prev) =>
                  prev.map((m) =>
                    m.id === assistantMessage.id
                      ? { ...m, agentStatus: '多智能体处理中...' }
                      : m
                  )
                )
              }
            } catch { /* ignore */ }
          } else {
            // 内容到达时，结束规划阶段（如有），计算用时
            setMessages((prev) =>
              prev.map((m) => {
                if (m.id !== assistantMessage.id) return m
                const planDuration = m.planStartAt && m.isPlanRunning
                  ? Math.round((Date.now() - m.planStartAt) / 1000)
                  : undefined
                return {
                  ...m,
                  isPlanRunning: false,
                  planDone: m.isPlanRunning ? true : m.planDone,
                  planDuration: m.planDuration ?? planDuration,
                }
              })
            )
            streamingContentRef.current += content
            flushContent()
          }
        },
        () => {
          setMessages((prev) =>
            prev.map((m) =>
              m.id === capturedMessageId
                ? {
                    ...m,
                    isStreaming: false,
                    isPlanRunning: false,
                    planDone: (m.planning?.length ?? 0) > 0 ? true : m.planDone,
                    agentStatus: undefined,
                  }
                : m
            )
          )
          // 只有本次 stream 仍是"活跃 stream"时才清除 loading，
          // 防止后台旧 stream 完成时误清除新 session 的 loading 状态
          if (loadingStreamRef.current === capturedMessageId) {
            setIsLoading(false)
            loadingStreamRef.current = null
          }
          // 直接写入 localStorage，即使组件已卸载（页面跳转后回来仍能看到完整回复）
          try {
            const raw = localStorage.getItem(`chat_messages_${capturedSessionId}`)
            if (raw) {
              const msgs = JSON.parse(raw) as Message[]
              const finalContent = streamingContentRef.current
              const updated = msgs.map(m =>
                m.id === capturedMessageId
                  ? { ...m, content: finalContent, isStreaming: false, isPlanRunning: false, agentStatus: undefined }
                  : m
              )
              localStorage.setItem(`chat_messages_${capturedSessionId}`, JSON.stringify(updated))
            }
          } catch { /* ignore */ }
        },
        abortCtrl.signal
      )
    } catch {
      setIsLoading(false)
    }
  }

  // ── 文件上传成功回调 ──────────────────────────────────────────────────────
  const handleFileUploadSuccess = () => {
    setMessages((prev) => [
      ...prev,
      { id: genId(), role: 'assistant', content: '文件已上传到知识库，正在异步索引…', timestamp: new Date() },
    ])
  }

  // ── 消息反馈 ──────────────────────────────────────────────────────────────
  const submitFeedback = async (messageIndex: number, vote: 1 | -1) => {
    // toggle：再次点击同一个按钮 → 取消反馈
    const current = votes[messageIndex]
    const newVote = current === vote ? 0 : vote
    setVotes(prev => {
      const next = { ...prev }
      if (newVote === 0) delete next[messageIndex]
      else next[messageIndex] = newVote as 1 | -1
      return next
    })
    try {
      await ragevalService.submitFeedback(currentSessionId, messageIndex, newVote as 1 | -1 | 0)
    } catch {
      // 静默失败，不打扰用户
    }
  }

  const genId = () => Math.random().toString(36).substring(2, 15)

  // ── 渲染 ─────────────────────────────────────────────────────────────────
  return (
    <div className="absolute inset-0 flex overflow-hidden">
      {/* 左侧会话列表 */}
      <SessionList
        sessions={sessions}
        currentSessionId={currentSessionId}
        onSelectSession={handleSelectSession}
        onNewSession={handleNewSession}
        onDeleteSession={handleDeleteSession}
      />

      {/* 右侧主区域 */}
      <div className="relative flex flex-1 flex-col min-h-0 overflow-hidden">
        {/* 渐变背景层（欢迎屏与对话区共用） */}
        <div className="pointer-events-none absolute inset-0 bg-gradient-to-br from-[#F8FAFC] via-white to-[#EFF6FF]" />
        <div
          className="pointer-events-none absolute inset-0 opacity-[0.35]"
          style={{
            backgroundImage:
              'linear-gradient(to right, rgba(59,130,246,0.07) 1px, transparent 1px), linear-gradient(to bottom, rgba(59,130,246,0.07) 1px, transparent 1px)',
            backgroundSize: '40px 40px',
          }}
        />
        <div className="pointer-events-none absolute -top-32 right-[-40px] h-72 w-72 rounded-full bg-[#BFDBFE]/50 blur-3xl" />
        <div className="pointer-events-none absolute -bottom-36 left-[-80px] h-80 w-80 rounded-full bg-[#FDE68A]/30 blur-3xl" />

        {messages.length === 0 ? (
          /* ── 欢迎屏：标题 + 居中输入框 + 预设卡片 ── */
          <div className="relative flex-1 min-h-0 overflow-y-auto chat-sidebar-scroll">
            <WelcomeScreen
              onPresetSelect={(text) => setInput(text)}
              isLoading={isLoading}
              inputSlot={
                  <div className="relative w-full space-y-1">
                    <ChatInput
                      value={input}
                      onChange={setInput}
                      onSend={handleSend}
                      isLoading={isLoading}
                      onFileUpload={handleFileUploadSuccess}
                      deepThinking={deepThinking}
                      onDeepThinkingChange={setDeepThinking}
                      webSearch={webSearch}
                      onWebSearchChange={setWebSearch}
                    />
                  </div>
              }
            />
          </div>
        ) : (
          /* ── 对话区 ── */
          <>
            <div
              ref={chatScrollRef}
              className="relative flex-1 min-h-0 overflow-y-auto chat-sidebar-scroll"
            >
              <div className="max-w-[760px] ml-[max(32px,calc(50vw-660px))] px-6 py-8 space-y-2">
                {messages.map((m, i) => (
                  <MessageBubble
                    key={m.id}
                    message={m}
                    isLast={i === messages.length - 1}
                    messageIndex={i}
                    vote={votes[i]}
                    onVote={submitFeedback}
                  />
                ))}
                <div ref={messagesEndRef} />
              </div>
            </div>

            {/* 底部输入区 */}
            <div className="relative border-t border-white/60 bg-white/60 backdrop-blur-sm px-4 py-4">
              <div className="max-w-[760px] ml-[max(32px,calc(50vw-660px))] space-y-2">
                {/* ChatInput */}
                <div className="relative">
                  <ChatInput
                    value={input}
                    onChange={setInput}
                    onSend={handleSend}
                    isLoading={isLoading}
                    onFileUpload={handleFileUploadSuccess}
                    deepThinking={deepThinking}
                    onDeepThinkingChange={setDeepThinking}
                    webSearch={webSearch}
                    onWebSearchChange={setWebSearch}
                  />
                </div>
              </div>
            </div>
          </>
        )}

      </div>
    </div>
  )
}

// ── MessageBubble ─────────────────────────────────────────────────────────────

interface MessageBubbleProps {
  message: Message
  isLast: boolean
  messageIndex: number
  vote?: 1 | -1
  onVote: (messageIndex: number, vote: 1 | -1) => void
}

function MessageBubble({ message, isLast, messageIndex, vote, onVote }: MessageBubbleProps) {
  if (message.role === 'user') {
    return (
      <div className="flex justify-end mb-4">
        <div
          className="max-w-[72%] rounded-3xl rounded-br-lg px-5 py-3 text-sm leading-relaxed text-white"
          style={{
            background: 'linear-gradient(135deg, #3B82F6 0%, #6366F1 100%)',
            boxShadow: '0 2px 12px rgba(59,130,246,0.25)',
          }}
        >
          {message.content}
        </div>
      </div>
    )
  }

  // assistant
  return (
    <div className="flex items-start gap-3 mb-4">
      {/* AI 头像 */}
      <div
        className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full text-white mt-1"
        style={{ background: 'linear-gradient(135deg, #3B82F6 0%, #6366F1 100%)' }}
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
          <path d="M12 2L15.09 8.26L22 9.27L17 14.14L18.18 21.02L12 17.77L5.82 21.02L7 14.14L2 9.27L8.91 8.26L12 2Z" fill="white" />
        </svg>
      </div>

      <div className="min-w-0 flex-1 space-y-2">
        {/* Agent 状态 */}
        {message.agentStatus && (
          <div className="inline-flex items-center gap-1.5 rounded-full border border-[#BFDBFE] bg-[#EFF6FF] px-3 py-1 text-xs text-[#2563EB]">
            <span className="h-1.5 w-1.5 rounded-full bg-[#3B82F6] animate-pulse" />
            {message.agentStatus}
          </div>
        )}

        {/* Plan Agent 规划过程区块 */}
        {(message.planning !== undefined) && (
          <PlanningBlock
            planning={message.planning || ''}
            isPlanRunning={!!message.isPlanRunning}
            isDone={!!message.planDone}
            planDuration={message.planDuration}
          />
        )}

        {/* 主内容 */}
        {message.content ? (
          <div className="rounded-3xl rounded-tl-lg border border-[#F0F0F0] bg-white px-5 py-3.5 shadow-[0_1px_6px_rgba(0,0,0,0.05)]">
            <div className="chat-markdown text-sm leading-relaxed text-[#1F2937]">
              {isLast && message.isStreaming ? (
                <>
                  <ReactMarkdown remarkPlugins={[remarkGfm]}>
                    {normalizeMarkdown(message.content)}
                  </ReactMarkdown>
                  <span className="inline-block h-4 w-1.5 animate-pulse bg-[#3B82F6] ml-0.5 rounded-sm align-text-bottom" />
                </>
              ) : (
                <ReactMarkdown remarkPlugins={[remarkGfm]}>
                  {normalizeMarkdown(message.content)}
                </ReactMarkdown>
              )}
            </div>
            {/* 点赞/踩按钮（流式完成后显示） */}
            {!message.isStreaming && (
              <div className="flex items-center gap-1 mt-2 pt-2 border-t border-gray-50">
                <div className="relative group/like">
                  <button
                    onClick={() => onVote(messageIndex, 1)}
                    className={cn(
                      'p-1 rounded hover:bg-emerald-50 transition-colors',
                      vote === 1 ? 'text-emerald-600' : 'text-gray-300 hover:text-emerald-500',
                    )}
                  >
                    <ThumbsUp className="w-3.5 h-3.5" />
                  </button>
                  <div className="pointer-events-none absolute bottom-full left-1/2 mb-1.5 -translate-x-1/2 whitespace-nowrap rounded-lg bg-[#1F2937] px-2 py-1 text-[11px] font-medium text-white opacity-0 shadow-md transition-opacity duration-150 group-hover/like:opacity-100">
                    有帮助
                    <span className="absolute left-1/2 top-full -translate-x-1/2 border-4 border-transparent border-t-[#1F2937]" />
                  </div>
                </div>
                <div className="relative group/dislike">
                  <button
                    onClick={() => onVote(messageIndex, -1)}
                    className={cn(
                      'p-1 rounded hover:bg-red-50 transition-colors',
                      vote === -1 ? 'text-red-500' : 'text-gray-300 hover:text-red-400',
                    )}
                  >
                    <ThumbsDown className="w-3.5 h-3.5" />
                  </button>
                  <div className="pointer-events-none absolute bottom-full left-1/2 mb-1.5 -translate-x-1/2 whitespace-nowrap rounded-lg bg-[#1F2937] px-2 py-1 text-[11px] font-medium text-white opacity-0 shadow-md transition-opacity duration-150 group-hover/dislike:opacity-100">
                    没帮助
                    <span className="absolute left-1/2 top-full -translate-x-1/2 border-4 border-transparent border-t-[#1F2937]" />
                  </div>
                </div>
              </div>
            )}
          </div>
        ) : (
          isLast && message.isStreaming && !message.isPlanRunning && (
            <div className="inline-flex items-center gap-1.5 rounded-3xl rounded-tl-lg border border-[#F0F0F0] bg-white px-5 py-4 shadow-[0_1px_6px_rgba(0,0,0,0.05)]">
              {[0, 1, 2].map((i) => (
                <span
                  key={i}
                  className="h-2 w-2 rounded-full bg-[#CBD5E1]"
                  style={{ animation: `chat-bounce 1.2s ease-in-out ${i * 0.2}s infinite` }}
                />
              ))}
            </div>
          )
        )}
      </div>
    </div>
  )
}

// ── PlanningBlock ─────────────────────────────────────────────────────────────

interface PlanningBlockProps {
  planning: string
  isPlanRunning: boolean
  isDone: boolean
  planDuration?: number
}

function PlanningBlock({ planning, isPlanRunning, isDone, planDuration }: PlanningBlockProps) {
  const [expanded, setExpanded] = useState(false)

  if (isPlanRunning) {
    return (
      <div className="rounded-2xl border border-[#A7F3D0] bg-[#ECFDF5] p-4">
        <div className="flex items-center gap-2 text-[#059669]">
          <Loader2 className="h-4 w-4 animate-spin flex-shrink-0" />
          <span className="text-sm font-medium">规划执行中...</span>
        </div>
        {planning && (
          <div className="mt-3 flex items-start gap-2 text-sm text-[#065F46]">
            <ListChecks className="mt-0.5 h-4 w-4 flex-shrink-0 text-[#059669]" />
            <div className="chat-markdown leading-relaxed min-w-0 flex-1">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>
                {normalizeMarkdown(planning)}
              </ReactMarkdown>
              <span className="ml-1 inline-block h-4 w-1.5 animate-pulse bg-[#059669] align-middle rounded-sm" />
            </div>
          </div>
        )}
      </div>
    )
  }

  if (!isDone || !planning) return null

  return (
    <div className="overflow-hidden rounded-2xl border border-[#A7F3D0] bg-[#ECFDF5]">
      <button
        type="button"
        onClick={() => setExpanded((v) => !v)}
        className="flex w-full items-center gap-2 px-4 py-3 text-left transition-colors hover:bg-[#A7F3D0]/30"
      >
        <div className="flex flex-1 items-center gap-2">
          <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-[#A7F3D0]">
            <ListChecks className="h-4 w-4 text-[#059669]" />
          </div>
          <span className="text-sm font-medium text-[#059669]">规划执行过程</span>
          <span className="rounded-full bg-[#A7F3D0] px-2 py-0.5 text-xs text-[#065F46]">
            {planDuration !== undefined ? `执行了 ${planDuration} 秒` : '已完成'}
          </span>
        </div>
        {expanded ? (
          <ChevronDown className="h-4 w-4 text-[#059669]" />
        ) : (
          <ChevronRight className="h-4 w-4 text-[#059669]" />
        )}
      </button>
      {expanded && (
        <div className="border-t border-[#A7F3D0] px-4 pb-4">
          <div className="mt-3 chat-markdown text-sm leading-relaxed text-[#065F46]">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>
              {normalizeMarkdown(planning)}
            </ReactMarkdown>
          </div>
        </div>
      )}
    </div>
  )
}

