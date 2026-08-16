import { getAccessToken } from './authSession'
import { apiClient, buildApiUrl } from './client'

export type PlaygroundRole = 'system' | 'user' | 'assistant'
export type PlaygroundStreamUpdateType = 'content' | 'reasoning'

export interface PlaygroundMessageInput {
  role: PlaygroundRole
  content: string
}

export interface PlaygroundModelsResponse {
  group_id: number
  models: string[]
  default_model: string
}

export interface PlaygroundChatRequest {
  group_id: number
  model: string
  messages: PlaygroundMessageInput[]
  temperature?: number
  top_p?: number
  max_tokens?: number
}

export async function listPlaygroundModels(groupID: number): Promise<PlaygroundModelsResponse> {
  const { data } = await apiClient.get<PlaygroundModelsResponse>('/playground/models', {
    params: { group_id: groupID },
  })
  return data
}

export async function streamPlaygroundChat(
  input: PlaygroundChatRequest,
  onUpdate: (type: PlaygroundStreamUpdateType, chunk: string) => void,
  signal: AbortSignal,
): Promise<void> {
  const token = getAccessToken()
  const response = await fetch(buildApiUrl('/playground/chat/completions'), {
    method: 'POST',
    headers: {
      Accept: 'text/event-stream',
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    credentials: 'include',
    cache: 'no-store',
    body: JSON.stringify({ ...input, stream: true }),
    signal,
  })
  if (!response.ok) {
    throw new Error(await playgroundErrorMessage(response))
  }
  if (!response.body) throw new Error('Playground returned an empty response')

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let finished = false

  const consumeBlock = (block: string) => {
    const data = block
      .split('\n')
      .filter((line) => line.startsWith('data:'))
      .map((line) => line.slice(5).trimStart())
      .join('\n')
      .trim()
    if (!data) return
    if (data === '[DONE]') {
      finished = true
      return
    }
    const payload = JSON.parse(data) as {
      error?: { message?: string }
      choices?: Array<{
        delta?: { content?: string; reasoning_content?: string }
      }>
    }
    if (payload.error?.message) throw new Error(payload.error.message)
    for (const choice of payload.choices || []) {
      if (choice.delta?.reasoning_content) onUpdate('reasoning', choice.delta.reasoning_content)
      if (choice.delta?.content) onUpdate('content', choice.delta.content)
    }
  }

  while (!finished) {
    const { value, done } = await reader.read()
    buffer += decoder.decode(value, { stream: !done }).replace(/\r\n/g, '\n')
    let boundary = buffer.indexOf('\n\n')
    while (boundary >= 0) {
      const block = buffer.slice(0, boundary)
      buffer = buffer.slice(boundary + 2)
      consumeBlock(block)
      if (finished) break
      boundary = buffer.indexOf('\n\n')
    }
    if (done) {
      if (buffer.trim()) consumeBlock(buffer)
      break
    }
  }
}

async function playgroundErrorMessage(response: Response): Promise<string> {
  const text = await response.text()
  if (!text) return `Request failed (${response.status})`
  try {
    const payload = JSON.parse(text) as {
      error?: { message?: string }
      message?: string
    }
    return payload.error?.message || payload.message || `Request failed (${response.status})`
  } catch {
    return text.slice(0, 300)
  }
}
