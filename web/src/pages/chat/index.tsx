import { useState, useRef, useEffect } from 'react'
import {
  Send,
  MoreHorizontal,
  Paperclip,
  ChevronDown,
  Layers,
  Loader2,
} from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { normalizeMarkdown } from '@/utils'
import { chatService, skillService, ChatSession } from '@/services'
import { Skill } from '@/services/skill'
import { ChatMessage } from '@/types'
import { cn } from '@/utils'
import { useContextStore } from '@/stores/contextStore'
import SessionList from './components/SessionList'

type MessageRole = 'user' | 'assistant'

interface Message {
  id: string
  role: MessageRole
  content: string
  timestamp: Date
  isStreaming?: boolean
  agentStatus?: string  // 路由/处理状态，独立于内容显示
}

type ChatMode = 'quick' | 'stream' | 'intent'

const MODE_NAMES: Record<ChatMode, string> = {
  quick: '快速',
  stream: '流式',
  intent: '多智能体',
}

export default function Chat() {
  const [sessions, setSessions] = useState<ChatSession[]>([])
  const [currentSessionId, setCurrentSessionId] = useState('')
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [mode, setMode] = useState<ChatMode>('quick')
  const [modeOpen, setModeOpen] = useState(false)
  const [toolsOpen, setToolsOpen] = useState(false)
  const [skills, setSkills] = useState<Skill[]>([])
  const [skillsPanelOpen, setSkillsPanelOpen] = useState(false)
  const [selectedSkill, setSelectedSkill] = useState<Skill | null>(null)
  const [skillParams, setSkillParams] = useState<Record<string, string>>({})
  const [uploadProgress, setUploadProgress] = useState(0)
  const [uploadingFileName, setUploadingFileName] = useState('')

  const messagesEndRef = useRef<HTMLDivElement>(null)
  const chatMessagesRef = useRef<HTMLDivElement>(null)
  const streamingContentRef = useRef('')
  const fileInputRef = useRef<HTMLInputElement>(null)
  const toolsBtnRef = useRef<HTMLButtonElement>(null)
  const modeBtnRef = useRef<HTMLButtonElement>(null)
  const toolsMenuRef = useRef<HTMLDivElement>(null)
  const modeDropdownRef = useRef<HTMLDivElement>(null)

  const { currentEventId, currentEventTitle, setContext } = useContextStore()

  // 初始化会话列表
  useEffect(() => {
    const loadedSessions = chatService.listSessions()
    if (loadedSessions.length === 0) {
      const newSession = chatService.createSession()
      setSessions([newSession])
      setCurrentSessionId(newSession.id)
    } else {
      setSessions(loadedSessions)
      const sessionId = chatService.getSessionId()
      setCurrentSessionId(sessionId)
      loadMessages(sessionId)
    }
  }, [])

  useEffect(() => {
    skillService.list().then(setSkills).catch(() => {})
  }, [])

  // 加载会话消息
  const loadMessages = (sessionId: string) => {
    const key = `chat_messages_${sessionId}`
    const data = localStorage.getItem(key)
    if (data) {
      setMessages(JSON.parse(data))
    } else {
      setMessages([])
    }
  }

  // 保存会话消息
  const saveMessages = (sessionId: string, msgs: Message[]) => {
    const key = `chat_messages_${sessionId}`
    localStorage.setItem(key, JSON.stringify(msgs))
  }

  // 切换会话
  const handleSelectSession = (sessionId: string) => {
    saveMessages(currentSessionId, messages)
    setCurrentSessionId(sessionId)
    chatService.switchSession(sessionId)
    loadMessages(sessionId)
  }

  // 新建会话
  const handleNewSession = () => {
    // 如果当前会话是空的，不创建新会话
    if (currentSessionId && messages.length === 0) {
      return
    }
    saveMessages(currentSessionId, messages)
    const newSession = chatService.createSession()
    setSessions([newSession, ...sessions])
    setCurrentSessionId(newSession.id)
    setMessages([])
  }

  // 删除会话
  const handleDeleteSession = (sessionId: string) => {
    chatService.deleteSession(sessionId)
    const key = `chat_messages_${sessionId}`
    localStorage.removeItem(key)
    const updatedSessions = sessions.filter(s => s.id !== sessionId)
    setSessions(updatedSessions)
    if (sessionId === currentSessionId) {
      if (updatedSessions.length > 0) {
        handleSelectSession(updatedSessions[0].id)
      } else {
        setCurrentSessionId('')
        setMessages([])
      }
    }
  }

  useEffect(() => {
    if (chatMessagesRef.current) {
      chatMessagesRef.current.scrollTop = chatMessagesRef.current.scrollHeight
    }
  }, [messages])

  // 自动保存消息
  useEffect(() => {
    if (currentSessionId && messages.length > 0) {
      saveMessages(currentSessionId, messages)
    }
  }, [messages, currentSessionId])

  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      const target = e.target as Node
      const inTools = toolsBtnRef.current?.contains(target) || toolsMenuRef.current?.contains(target)
      const inMode = modeBtnRef.current?.contains(target) || modeDropdownRef.current?.contains(target)
      if (!inTools) setToolsOpen(false)
      if (!inMode) setModeOpen(false)
    }
    document.addEventListener('click', handleClick)
    return () => document.removeEventListener('click', handleClick)
  }, [])

  const generateId = () => Math.random().toString(36).substring(2, 15)


  const handleSend = async () => {
    if (!input.trim() || isLoading) return

    // 如果没有当前会话，先创建一个
    let sessionId = currentSessionId
    if (!sessionId) {
      const newSession = chatService.createSession()
      setSessions([newSession, ...sessions])
      setCurrentSessionId(newSession.id)
      sessionId = newSession.id
    }

    let messageContent = input.trim()
    if (currentEventId && currentEventTitle) {
      messageContent = `[当前上下文: 事件#${currentEventId} "${currentEventTitle}"]\n${messageContent}`
    }

    const userMessage: Message = {
      id: generateId(),
      role: 'user',
      content: input.trim(),
      timestamp: new Date(),
    }
    const newMessages = [...messages, userMessage]
    setMessages(newMessages)
    setInput('')
    setIsLoading(true)

    // 更新会话标题（首条消息）
    if (messages.length === 0) {
      const title = input.trim().slice(0, 30) + (input.trim().length > 30 ? '...' : '')
      chatService.updateSession(sessionId, { title })
      setSessions(prevSessions => prevSessions.map(s => s.id === sessionId ? { ...s, title } : s))
    }

    // 更新会话时间和消息数
    chatService.updateSession(sessionId, {
      lastMessageAt: Date.now(),
      messageCount: messages.length + 1,
    })
    setSessions(prevSessions => prevSessions.map(s =>
      s.id === sessionId
        ? { ...s, lastMessageAt: Date.now(), messageCount: messages.length + 1 }
        : s
    ))

    const assistantMessage: Message = {
      id: generateId(),
      role: 'assistant',
      content: '',
      timestamp: new Date(),
      isStreaming: true,
    }
    setMessages((prev) => [...prev, assistantMessage])

    const history: ChatMessage[] = messages.map((m) => ({
      role: m.role,
      content: m.content,
    }))

    streamingContentRef.current = ''
    let updateTimer: ReturnType<typeof setTimeout> | null = null

    const flushContent = () => {
      setMessages((prev) =>
        prev.map((m) =>
          m.id === assistantMessage.id ? { ...m, content: streamingContentRef.current } : m
        )
      )
    }

    try {
      if (mode === 'quick') {
        const res = await chatService.chat({ message: messageContent, history })
        setMessages((prev) =>
          prev.map((m) =>
            m.id === assistantMessage.id
              ? { ...m, content: res.reply, isStreaming: false }
              : m
          )
        )
      } else if (mode === 'intent') {
        chatService.supervisorChat(
          messageContent,
          (agent, content) => {
            if (agent === 'status') {
              // 状态消息：更新 agentStatus 字段，不混入内容
              setMessages((prev) =>
                prev.map((m) =>
                  m.id === assistantMessage.id ? { ...m, agentStatus: content } : m
                )
              )
            } else {
              // 内容消息：累积到 streamingContent
              streamingContentRef.current += content
              flushContent()
            }
          },
          () => {
            setMessages((prev) =>
              prev.map((m) =>
                m.id === assistantMessage.id ? { ...m, isStreaming: false } : m
              )
            )
            setIsLoading(false)
          }
        )
      } else {
        await chatService.chatStream(
          { message: messageContent, history },
          (content) => {
            streamingContentRef.current += content
            if (!updateTimer) {
              updateTimer = setTimeout(() => {
                flushContent()
                updateTimer = null
              }, 50)
            }
          },
          () => {
            if (updateTimer) clearTimeout(updateTimer)
            flushContent()
            setMessages((prev) =>
              prev.map((m) =>
                m.id === assistantMessage.id ? { ...m, isStreaming: false } : m
              )
            )
            setIsLoading(false)
          },
          (error) => {
            setMessages((prev) =>
              prev.map((m) =>
                m.id === assistantMessage.id
                  ? { ...m, content: `错误: ${error.message}`, isStreaming: false }
                  : m
              )
            )
            setIsLoading(false)
          }
        )
      }
      if (mode !== 'intent' && mode !== 'stream') setIsLoading(false)
    } catch {
      setIsLoading(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file || isLoading) return
    const allowed = ['.txt', '.md', '.markdown']
    const name = file.name.toLowerCase()
    if (!allowed.some((ext) => name.endsWith(ext))) return

    setIsLoading(true)
    setUploadingFileName(file.name)
    setUploadProgress(0)

    try {
      await chatService.uploadFile(file, (progress) => {
        setUploadProgress(progress)
      })
      setMessages((prev) => [
        ...prev,
        {
          id: generateId(),
          role: 'assistant',
          content: `${file.name} 上传到知识库成功`,
          timestamp: new Date(),
        },
      ])
    } catch {
      setMessages((prev) => [
        ...prev,
        {
          id: generateId(),
          role: 'assistant',
          content: '文件上传失败',
          timestamp: new Date(),
        },
      ])
    } finally {
      setIsLoading(false)
      setUploadingFileName('')
      setUploadProgress(0)
    }
    e.target.value = ''
  }

  const handleSkillExecute = () => {
    if (!selectedSkill || isLoading) return

    // 如果没有当前会话，先创建一个
    let sessionId = currentSessionId
    if (!sessionId) {
      const newSession = chatService.createSession()
      setSessions([newSession, ...sessions])
      setCurrentSessionId(newSession.id)
      sessionId = newSession.id
    }

    setIsLoading(true)

    // 创建用户消息（只显示参数值，更自然）
    const paramsText = Object.values(skillParams).filter(v => v).join('、')
    const userMessage: Message = {
      id: generateId(),
      role: 'user',
      content: paramsText || selectedSkill.name,
      timestamp: new Date(),
    }

    // 创建助手消息
    const assistantMessage: Message = {
      id: generateId(),
      role: 'assistant',
      content: '',
      timestamp: new Date(),
      isStreaming: true,
    }

    setMessages((prev) => [...prev, userMessage, assistantMessage])

    // 更新会话标题（首条消息）
    if (messages.length === 0) {
      const title = `${selectedSkill.name}${paramsText ? `: ${paramsText.slice(0, 20)}` : ''}`
      chatService.updateSession(sessionId, { title })
      setSessions(prevSessions => prevSessions.map(s => s.id === sessionId ? { ...s, title } : s))
    }

    // 更新会话时间和消息数
    chatService.updateSession(sessionId, {
      lastMessageAt: Date.now(),
      messageCount: messages.length + 2,
    })
    setSessions(prevSessions => prevSessions.map(s =>
      s.id === sessionId
        ? { ...s, lastMessageAt: Date.now(), messageCount: messages.length + 2 }
        : s
    ))

    let content = ''
    let updateTimer: ReturnType<typeof setTimeout> | null = null

    const flushContent = () => {
      setMessages((prev) =>
        prev.map((m) =>
          m.id === assistantMessage.id ? { ...m, content } : m
        )
      )
    }

    skillService.execute(
      selectedSkill.id,
      skillParams,
      (type, text) => {
        if (type === 'step') {
          content += `[${text}]\n`
        } else {
          content = text  // 收到结果时清空进度提示
        }
        if (!updateTimer) {
          updateTimer = setTimeout(() => {
            flushContent()
            updateTimer = null
          }, 50)
        }
      },
      () => {
        if (updateTimer) clearTimeout(updateTimer)
        flushContent()
        setMessages((prev) =>
          prev.map((m) =>
            m.id === assistantMessage.id ? { ...m, isStreaming: false } : m
          )
        )
        setIsLoading(false)
        setSelectedSkill(null)
        setSkillParams({})
      }
    )
  }

  const centered = messages.length === 0

  return (
    <div className="flex h-[calc(100vh-4rem)] -m-8">
      <SessionList
        sessions={sessions}
        currentSessionId={currentSessionId}
        onSelectSession={handleSelectSession}
        onNewSession={handleNewSession}
        onDeleteSession={handleDeleteSession}
      />
      <div className="flex flex-col flex-1 min-h-0 bg-white">
        <div className={cn('chat-container view-panel flex-1 flex flex-col', centered && 'centered')}>
          <div className="welcome-greeting">
          <p>你好！我是安全事件智能研判助手</p>
        </div>

        <div className="chat-messages" ref={chatMessagesRef}>
          {messages.map((m, i) => (
            <div
              key={m.id}
              className={cn(
                'message',
                m.role,
                m.role === 'assistant' && i === messages.length - 1 && m.isStreaming && 'streaming'
              )}
            >
              {m.role === 'assistant' && (
                <div className="message-avatar">
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
                    <path
                      d="M12 2L15.09 8.26L22 9.27L17 14.14L18.18 21.02L12 17.77L5.82 21.02L7 14.14L2 9.27L8.91 8.26L12 2Z"
                      fill="white"
                    />
                  </svg>
                </div>
              )}
              <div className="message-content-wrapper">
                <div className="message-content">
                  {m.role === 'assistant' ? (
                    <>
                      {m.agentStatus && (
                        <div className="text-xs text-gray-400 mb-1 font-normal opacity-75">
                          {m.agentStatus}
                        </div>
                      )}
                      {m.content ? (
                        i === messages.length - 1 && m.isStreaming ? (
                          <>
                            <ReactMarkdown remarkPlugins={[remarkGfm]}>
                              {normalizeMarkdown(m.content)}
                            </ReactMarkdown>
                            <span className="inline-block w-1.5 h-4 bg-gray-500 animate-pulse ml-0.5" />
                          </>
                        ) : (
                          <ReactMarkdown remarkPlugins={[remarkGfm]}>
                            {normalizeMarkdown(m.content)}
                          </ReactMarkdown>
                        )
                      ) : (
                        <span />
                      )}
                    </>
                  ) : (
                    m.content
                  )}
                </div>
              </div>
            </div>
          ))}
          <div ref={messagesEndRef} />
        </div>

        <div className="chat-input-container">
          {currentEventId && (
            <div className="event-context-bar input-group-wrapper">
              <span className="event-context-label">当前事件：</span>
              <span className="event-context-title">
                {currentEventTitle || `#${currentEventId}`}
              </span>
              <button
                type="button"
                className="event-context-clear"
                onClick={() => setContext('', undefined, undefined)}
              >
                清除
              </button>
            </div>
          )}

          {selectedSkill && (
            <div className="input-group-wrapper mb-2 p-3 bg-gray-50 rounded-xl border border-gray-200">
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-medium text-gray-700">{selectedSkill.name}</span>
                <button
                  onClick={() => setSelectedSkill(null)}
                  className="text-gray-500 hover:text-gray-700 text-sm"
                >
                  关闭
                </button>
              </div>
              <div className="flex gap-2 items-end">
                {selectedSkill.params.map((p) => (
                  <input
                    key={p.name}
                    type={p.type === 'number' ? 'number' : 'text'}
                    placeholder={p.description}
                    value={skillParams[p.name] || ''}
                    onChange={(e) =>
                      setSkillParams({ ...skillParams, [p.name]: e.target.value })
                    }
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && !isLoading) {
                        e.preventDefault()
                        handleSkillExecute()
                      }
                    }}
                    className="input flex-1 text-sm"
                  />
                ))}
                <button
                  onClick={handleSkillExecute}
                  disabled={isLoading}
                  className="btn-primary btn-sm"
                >
                  {isLoading ? (
                    <Loader2 className="w-3 h-3 animate-spin" />
                  ) : (
                    '执行'
                  )}
                </button>
              </div>
            </div>
          )}

          {uploadingFileName && (
            <div className="input-group-wrapper mb-2 p-3 bg-blue-50 rounded-xl border border-blue-200">
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-medium text-gray-700">上传文件：{uploadingFileName}</span>
                <span className="text-sm font-semibold text-blue-600">{uploadProgress}%</span>
              </div>
              <div className="progress">
                <div className="progress-bar bg-blue-600" style={{ width: `${uploadProgress}%` }} />
              </div>
            </div>
          )}

          <div className="input-group-wrapper">
            <div className="input-wrapper">
              <input
                type="text"
                className="message-input"
                placeholder="问问安全事件智能研判助手"
                maxLength={1000}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                disabled={isLoading}
              />
              <div className="input-bottom-bar">
                <div className={cn('tools-btn-wrapper', toolsOpen && 'active')}>
                  <button
                    ref={toolsBtnRef}
                    type="button"
                    className={cn('tools-btn', toolsOpen && 'active')}
                    title="更多选项"
                    onClick={() => setToolsOpen(!toolsOpen)}
                  >
                    <MoreHorizontal className="tools-icon w-5 h-5" />
                  </button>
                  {toolsOpen && (
                    <div
                      ref={toolsMenuRef}
                      className="tools-menu"
                      style={{ position: 'absolute', bottom: 'calc(100% + 8px)', left: 0 }}
                    >
                      <div
                        className="tools-menu-item"
                        onClick={() => {
                          setSkillsPanelOpen(true)
                          setToolsOpen(false)
                        }}
                      >
                        <Layers className="w-5 h-5 flex-shrink-0 text-gray-600" />
                        <span>Skills</span>
                      </div>
                      <div
                        className="tools-menu-item"
                        onClick={() => {
                          fileInputRef.current?.click()
                          setToolsOpen(false)
                        }}
                      >
                        <Paperclip className="w-5 h-5 flex-shrink-0 text-gray-600" />
                        <span>上传文件</span>
                      </div>
                    </div>
                  )}
                </div>
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".txt,.md,.markdown"
                  className="hidden"
                  onChange={handleUpload}
                />
                <div className="right-actions">
                  <div className={cn('mode-selector-wrapper', modeOpen && 'active')}>
                    <button
                      ref={modeBtnRef}
                      type="button"
                      className={cn('mode-selector-btn', modeOpen && 'active')}
                      onClick={() => setModeOpen(!modeOpen)}
                    >
                      <span>{MODE_NAMES[mode]}</span>
                      <ChevronDown className="dropdown-arrow w-4 h-4" />
                    </button>
                    {modeOpen && (
                      <div
                        ref={modeDropdownRef}
                        className="mode-dropdown"
                        style={{ position: 'absolute', bottom: 'calc(100% + 8px)', right: 0 }}
                      >
                        {(['quick', 'stream', 'intent'] as const).map((m) => (
                          <div
                            key={m}
                            className={cn('dropdown-item', mode === m && 'active')}
                            onClick={() => {
                              setMode(m)
                              setModeOpen(false)
                            }}
                          >
                            <span>{MODE_NAMES[m]}</span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                  <button
                    type="button"
                    className="btn btn-primary rounded-full w-9 h-9 p-0"
                    title="发送"
                    disabled={!input.trim() || isLoading}
                    onClick={handleSend}
                  >
                    {isLoading ? (
                      <Loader2 className="w-5 h-5 animate-spin" />
                    ) : (
                      <Send className="w-5 h-5" />
                    )}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    {skillsPanelOpen && (
        <div className="skills-modal-overlay">
          <div className="skills-modal">
            <div className="skills-modal-header">
              <h3 className="skills-modal-title">Skills</h3>
              <button
                onClick={() => setSkillsPanelOpen(false)}
                className="p-2 rounded-lg text-gray-500 hover:bg-gray-200 transition-colors"
              >
                ✕
              </button>
            </div>
            <div className="skills-modal-body">
              <div className="grid grid-cols-2 gap-3">
                {skills.map((skill) => (
                  <div
                    key={skill.id}
                    onClick={() => {
                      setSelectedSkill(skill)
                      setSkillParams({})
                      setSkillsPanelOpen(false)
                    }}
                    className={cn(
                      'skill-card',
                      selectedSkill?.id === skill.id && 'selected'
                    )}
                  >
                    <div className="skill-card-name">{skill.name}</div>
                    <div className="skill-card-desc">{skill.description}</div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
