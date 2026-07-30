import { apiClient, buildGatewayUrl } from './client'

export interface PromptLibraryAuthor {
  username?: string
  name?: string | null
}

export interface PromptLibraryItem {
  id: string
  title: string
  description: string | null
  content: string
  type: string
  slug: string
  voteCount?: number
  viewCount?: number
  mediaUrl?: string | null
  userExamples?: Array<{ mediaUrl?: string | null }>
  author?: PromptLibraryAuthor
  aiTitle?: string
  aiDescription?: string
  aiScore?: number
  ikikCategory?: string
}

export interface PromptLibraryPage {
  prompts: PromptLibraryItem[]
  total: number
  page: number
  perPage: number
  totalPages: number
}

export interface PromptSearchPlan {
  keywords: string[]
}

export interface PromptRankResult {
  id: string
  title: string
  description: string
  score: number
}

export async function listPromptLibrary(params: {
  page?: number
  perPage?: number
  q?: string
  sort?: 'newest' | 'oldest' | 'upvotes'
  type?: 'TEXT' | 'STRUCTURED' | 'IMAGE' | 'VIDEO' | 'AUDIO'
  locale?: string
} = {}): Promise<PromptLibraryPage> {
  const { data } = await apiClient.get<PromptLibraryPage>('/prompt-library', {
    params: {
      page: params.page || 1,
      per_page: params.perPage || 12,
      q: params.q?.trim() || undefined,
      sort: params.sort || 'upvotes',
      type: params.type || undefined,
      locale: params.locale || undefined,
    },
  })
  return data
}

export async function listGatewayModels(apiKey: string, signal?: AbortSignal): Promise<string[]> {
  const response = await fetch(buildGatewayUrl('/v1/models'), {
    headers: { Authorization: `Bearer ${apiKey}` },
    signal,
  })
  if (!response.ok) {
    throw new Error(await gatewayErrorMessage(response))
  }
  const payload = await response.json() as { data?: Array<{ id?: string }> }
  return Array.from(new Set(
    (payload.data || [])
      .map(item => String(item.id || '').trim())
      .filter(Boolean),
  ))
}

export async function createPromptSearchPlan(options: {
  query: string
  apiKey: string
  model: string
}): Promise<PromptSearchPlan> {
  const content = await callChatCompletion({
    apiKey: options.apiKey,
    model: options.model,
    system: 'Convert the user request into up to three concise English search keywords or short phrases for a prompt library. Return JSON only: {"keywords":["..."]}. Do not answer the request.',
    user: options.query,
  })
  const parsed = parseJSONObject(content) as Partial<PromptSearchPlan>
  const keywords = Array.isArray(parsed.keywords)
    ? parsed.keywords.map(value => String(value).trim()).filter(Boolean).slice(0, 3)
    : []
  return { keywords: keywords.length > 0 ? keywords : [options.query.trim()] }
}

export async function rankPromptCandidates(options: {
  query: string
  apiKey: string
  model: string
  locale: string
  candidates: PromptLibraryItem[]
}): Promise<PromptRankResult[]> {
  const candidates = options.candidates.slice(0, 40).map(item => ({
    id: item.id,
    title: item.title,
    description: String(item.description || '').slice(0, 240),
  }))
  const language = options.locale.toLowerCase().startsWith('zh') ? 'Simplified Chinese' : 'English'
  const content = await callChatCompletion({
    apiKey: options.apiKey,
    model: options.model,
    system: `Rank prompt-library candidates for the user request. Return at most 12 useful results. Translate each display title and description into ${language}. Preserve product names, model names, code, and placeholders. Return JSON only: {"results":[{"id":"candidate id","title":"display title","description":"display description","score":0-100}]}. Use only candidate ids and do not include explanations.`,
    user: JSON.stringify({ query: options.query, candidates }),
  })
  const parsed = parseJSONObject(content) as { results?: unknown[] }
  if (!Array.isArray(parsed.results)) return []
  const allowed = new Set(candidates.map(item => item.id))
  return parsed.results
    .map((value) => value as Partial<PromptRankResult>)
    .filter(value => typeof value.id === 'string' && allowed.has(value.id))
    .map(value => ({
      id: String(value.id),
      title: String(value.title || '').trim(),
      description: String(value.description || '').trim(),
      score: Math.max(0, Math.min(100, Number(value.score) || 0)),
    }))
    .slice(0, 12)
}

async function callChatCompletion(options: {
  apiKey: string
  model: string
  system: string
  user: string
}): Promise<string> {
  const controller = new AbortController()
  const timeout = window.setTimeout(() => controller.abort(), 45000)
  try {
    const response = await fetch(buildGatewayUrl('/v1/chat/completions'), {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${options.apiKey}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        model: options.model,
        stream: false,
        messages: [
          { role: 'system', content: options.system },
          { role: 'user', content: options.user },
        ],
      }),
      signal: controller.signal,
    })
    if (!response.ok) {
      throw new Error(await gatewayErrorMessage(response))
    }
    const payload = await response.json() as {
      choices?: Array<{ message?: { content?: string | Array<{ type?: string; text?: string }> } }>
    }
    const message = payload.choices?.[0]?.message?.content
    if (typeof message === 'string') return message
    if (Array.isArray(message)) {
      return message.map(part => part.text || '').join('')
    }
    throw new Error('AI search returned an empty response')
  } finally {
    window.clearTimeout(timeout)
  }
}

async function gatewayErrorMessage(response: Response): Promise<string> {
  const text = await response.text()
  if (!text) return `Request failed (${response.status})`
  try {
    const payload = JSON.parse(text) as { error?: { message?: string }; message?: string }
    return payload.error?.message || payload.message || `Request failed (${response.status})`
  } catch {
    return text.slice(0, 240)
  }
}

function parseJSONObject(content: string): Record<string, unknown> {
  const trimmed = content.trim().replace(/^```(?:json)?\s*/i, '').replace(/\s*```$/, '')
  const start = trimmed.indexOf('{')
  const end = trimmed.lastIndexOf('}')
  if (start < 0 || end <= start) throw new Error('AI search returned invalid JSON')
  return JSON.parse(trimmed.slice(start, end + 1)) as Record<string, unknown>
}
