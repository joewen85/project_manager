// Shared Server-Sent-Events client for the AI endpoints. LLM generation is
// driven over SSE (the `/stream` route variants) rather than a plain POST so the
// connection streams immediately and stays alive — this avoids proxy/gateway
// timeouts on slow completions. The backend emits an event contract of
// status / delta / result / done / error (see aiAssistantSSEWriter).

const defaultAIStreamTimeoutMs = 35000
const configuredAIStreamTimeoutMs = Number(import.meta.env.VITE_AI_ASSISTANT_TIMEOUT_MS)
// Per-chunk watchdog: aborts only after this many ms with no data, not a hard total cap.
const aiStreamRequestTimeoutMs = Number.isFinite(configuredAIStreamTimeoutMs) && configuredAIStreamTimeoutMs > 0
  ? configuredAIStreamTimeoutMs
  : defaultAIStreamTimeoutMs

const apiBaseURL = String(import.meta.env.VITE_API_BASE_URL || '/api/v1').replace(/\/+$/, '')

const buildAIStreamURL = (path: string) => `${apiBaseURL}${path}/stream`

const parseSSEBlock = (block: string) => {
  let event = 'message'
  const dataLines: string[] = []
  for (const rawLine of block.split('\n')) {
    const line = rawLine.endsWith('\r') ? rawLine.slice(0, -1) : rawLine
    if (line.startsWith('event:')) {
      event = line.slice('event:'.length).trim()
    } else if (line.startsWith('data:')) {
      dataLines.push(line.slice('data:'.length).trimStart())
    }
  }
  const dataText = dataLines.join('\n').trim()
  return { event, data: dataText ? JSON.parse(dataText) as unknown : null }
}

export const postAIStream = async <T,>(
  path: string,
  payload: Record<string, unknown>,
  onStatus: (message: string) => void,
  onDelta?: (text: string) => void
): Promise<T> => {
  const controller = new AbortController()
  let timeoutID: number | undefined
  const refreshTimeout = () => {
    if (timeoutID !== undefined) window.clearTimeout(timeoutID)
    timeoutID = window.setTimeout(() => controller.abort(), aiStreamRequestTimeoutMs)
  }
  refreshTimeout()
  try {
    const token = localStorage.getItem('token') || ''
    const headers: Record<string, string> = {
      Accept: 'text/event-stream',
      'Content-Type': 'application/json'
    }
    if (token) headers.Authorization = `Bearer ${token}`

    const response = await fetch(buildAIStreamURL(path), {
      method: 'POST',
      headers,
      body: JSON.stringify(payload),
      signal: controller.signal
    })
    if (!response.ok) {
      const errorBody = await response.json().catch(() => null) as { message?: string } | null
      throw new Error(errorBody?.message || `请求失败（${response.status}）`)
    }
    if (!response.body) {
      throw new Error('浏览器不支持流式响应')
    }

    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let result: T | null = null
    let done = false
    while (!done) {
      const chunk = await reader.read()
      if (chunk.done) break
      refreshTimeout()
      buffer += decoder.decode(chunk.value, { stream: true })
      let boundary = buffer.indexOf('\n\n')
      while (boundary >= 0) {
        const block = buffer.slice(0, boundary).trim()
        buffer = buffer.slice(boundary + 2)
        if (block) {
          const message = parseSSEBlock(block)
          if (message.event === 'status' && message.data && typeof message.data === 'object' && 'message' in message.data) {
            onStatus(String((message.data as { message?: string }).message || ''))
          }
          if (message.event === 'delta' && message.data && typeof message.data === 'object' && 'text' in message.data) {
            onDelta?.(String((message.data as { text?: string }).text || ''))
          }
          if (message.event === 'error' && message.data && typeof message.data === 'object' && 'message' in message.data) {
            throw new Error(String((message.data as { message?: string }).message || 'AI 生成失败'))
          }
          if (message.event === 'result') {
            result = message.data as T
          }
          if (message.event === 'done') {
            done = true
            break
          }
        }
        boundary = buffer.indexOf('\n\n')
      }
    }
    if (!result) {
      throw new Error('AI 没有返回结果')
    }
    return result
  } finally {
    if (timeoutID !== undefined) window.clearTimeout(timeoutID)
  }
}
