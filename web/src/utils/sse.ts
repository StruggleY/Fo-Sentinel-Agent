/**
 * streamFetch 统一标准 SSE 协议的读流工具。
 *
 * 解析后端推送的标准 SSE 格式：
 *   event: <type>\n
 *   data: <content>\n
 *   \n
 *
 * 流结束标志：data: [DONE]\n\n
 *
 * 适用所有 SSE 端点：intent、pipeline/stream、skills/execute、chat_stream
 */
export function streamFetch(
  url: string,
  body: unknown,
  onChunk: (type: string, content: string) => void,
  onDone: () => void,
  onError?: (e: Error) => void
): void {
  fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
    .then(async res => {
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
    .catch(e => onError?.(e instanceof Error ? e : new Error(String(e))))
}
