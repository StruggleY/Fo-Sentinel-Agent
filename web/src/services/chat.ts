import api from './api'
import { streamFetch } from '@/utils/sse'
import {
  ChatMessage,
  ChatRequest,
  ChatResponse,
  ApiResponse,
} from '@/types'

// 会话接口
export interface ChatSession {
  id: string
  title: string
  createdAt: number
  lastMessageAt: number
  messageCount: number
}

// 生成会话ID
const generateSessionId = () => {
  return 'session-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9)
}

// 获取当前会话ID
const getCurrentSessionId = () => {
  return localStorage.getItem('current_session_id') || ''
}

// 设置当前会话ID
const setCurrentSessionId = (sessionId: string) => {
  localStorage.setItem('current_session_id', sessionId)
}

// 获取所有会话
const getSessions = (): ChatSession[] => {
  const data = localStorage.getItem('chat_sessions')
  return data ? JSON.parse(data) : []
}

// 保存会话列表
const saveSessions = (sessions: ChatSession[]) => {
  localStorage.setItem('chat_sessions', JSON.stringify(sessions))
}

export const chatService = {
  // 获取当前会话ID
  getSessionId: (): string => {
    let sessionId = getCurrentSessionId()
    if (!sessionId) {
      sessionId = generateSessionId()
      setCurrentSessionId(sessionId)
      // 创建新会话
      const sessions = getSessions()
      sessions.unshift({
        id: sessionId,
        title: '新对话',
        createdAt: Date.now(),
        lastMessageAt: Date.now(),
        messageCount: 0,
      })
      saveSessions(sessions)
    }
    return sessionId
  },

  // 获取所有会话
  listSessions: (): ChatSession[] => {
    return getSessions()
  },

  // 创建新会话
  createSession: (): ChatSession => {
    const sessionId = generateSessionId()
    const session: ChatSession = {
      id: sessionId,
      title: '新对话',
      createdAt: Date.now(),
      lastMessageAt: Date.now(),
      messageCount: 0,
    }
    const sessions = getSessions()
    sessions.unshift(session)
    saveSessions(sessions)
    setCurrentSessionId(sessionId)
    return session
  },

  // 切换会话
  switchSession: (sessionId: string) => {
    setCurrentSessionId(sessionId)
  },

  // 删除会话
  deleteSession: (sessionId: string) => {
    const sessions = getSessions().filter(s => s.id !== sessionId)
    saveSessions(sessions)
    if (getCurrentSessionId() === sessionId) {
      const newSession = sessions[0]
      if (newSession) {
        setCurrentSessionId(newSession.id)
      } else {
        localStorage.removeItem('current_session_id')
      }
    }
  },

  // 更新会话信息
  updateSession: (sessionId: string, updates: Partial<ChatSession>) => {
    const sessions = getSessions()
    const index = sessions.findIndex(s => s.id === sessionId)
    if (index !== -1) {
      sessions[index] = { ...sessions[index], ...updates }
      saveSessions(sessions)
    }
  },

  // Intent 意图驱动多 Agent 对话
  supervisorChat: (
    query: string,
    onMessage: (intent: string, content: string) => void,
    onDone: () => void
  ) => {
    streamFetch(
      '/api/chat/v1/intent_recognition',
      { query },
      (type, content) => onMessage(type, content),
      onDone
    )
  },

  // 普通对话
  async chat(data: ChatRequest): Promise<ChatResponse> {
    const res = await api.post<ApiResponse<{ answer: string }>>('/chat', {
      Id: chatService.getSessionId(),
      Question: data.message,
    })
    return { reply: res.data.data.answer }
  },

  // 流式对话
  chatStream: (
    data: { message: string; history?: ChatMessage[] },
    onMessage: (content: string) => void,
    onDone: () => void,
    onError: (error: Error) => void
  ): void => {
    streamFetch(
      '/api/chat_stream',
      { Id: chatService.getSessionId(), Question: data.message },
      (_type, content) => onMessage(content),
      onDone,
      onError
    )
  },

  // 文件上传
  async uploadFile(
    file: File,
    onProgress?: (progress: number) => void
  ): Promise<{ file_id: string; filename: string }> {
    const formData = new FormData()
    formData.append('file', file)
    const res = await api.post<ApiResponse<{ fileName: string; filePath: string; fileSize: number }>>(
      '/upload',
      formData,
      {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
        onUploadProgress: (progressEvent) => {
          if (onProgress && progressEvent.total) {
            const progress = Math.round((progressEvent.loaded * 100) / progressEvent.total)
            onProgress(progress)
          }
        },
      }
    )
    const d = res.data.data
    return { file_id: d.filePath || d.fileName, filename: d.fileName }
  },
}
