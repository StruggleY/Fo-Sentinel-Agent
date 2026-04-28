/**
 * streamFetch 统一标准 SSE 协议的读流工具。
 *
 * 解析后端推送的标准 SSE 格式：
 *   event: <type>\n
 *   data: <content>\n
 *   \n
 *
 * 支持的事件类型：
 *   - meta      会话元数据（sessionId、timestamp），在流开始时推送一次
 *   - status    处理状态通知（路由、Agent 切换）
 *   - chat/event/report/risk/solve  各 Agent 的内容流
 *   - error     错误通知
 *   - done      流结束
 *
 * 流结束标志：data: [DONE]\n\n
 *
 * 适用所有 SSE 端点：intent、pipeline/stream、chat_stream
 */
export function streamFetch(
  url: string,
  body: unknown,
  onChunk: (type: string, content: string) => void,
  onDone: () => void,
  onError?: (e: Error) => void,
  signal?: AbortSignal
): void {
  const token = localStorage.getItem('token')
  fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(body),
    signal,
  })
    .then(async res => {
      // 处理限流/并发/认证 HTTP 错误（非 SSE 流）
      if (!res.ok) {
        const errMap: Record<number, string> = {
          401: '请先登录',
          429: '请求过于频繁，请稍后重试',
          503: '当前请求过多，请稍后重试',
        }
        const msg = errMap[res.status] ?? `请求失败（${res.status}）`
        onError?.(new Error(msg))
        return
      }
      const reader = res.body?.getReader()
      if (!reader) { onDone(); return }

      const decoder = new TextDecoder()
      let buffer = ''
      let currentEvent = ''
      let currentData = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() ?? ''

        for (const line of lines) {
          if (line.startsWith('event: ')) {
            currentEvent = line.slice(7).trim()
          } else if (line.startsWith('data: ')) {
            const data = line.slice(6)
            if (data === '[DONE]') { onDone(); return }
            if (currentData) currentData += '\n'
            currentData += data
          } else if (line === '') {
            if (currentEvent && currentData) {
              if (currentEvent === 'done') { onDone(); return }
              if (currentEvent === 'error') { onError?.(new Error(currentData)); return }
              if (currentEvent !== 'connected') {
                onChunk(currentEvent, currentData)
              }
            }
            currentEvent = ''
            currentData = ''
          }
        }
      }
      onDone()
    })
    .catch(e => {
      // AbortError 是主动取消，不触发 onError
      if (e instanceof Error && e.name === 'AbortError') return
      onError?.(e instanceof Error ? e : new Error(String(e)))
    })
}
