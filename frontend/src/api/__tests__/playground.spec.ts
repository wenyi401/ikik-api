import { beforeEach, describe, expect, it, vi } from 'vitest'

const { client, getAccessToken } = vi.hoisted(() => ({
  client: { get: vi.fn() },
  getAccessToken: vi.fn(() => 'test-access-token'),
}))

vi.mock('@/api/client', () => ({
  apiClient: client,
  buildApiUrl: (path: string) => `/api/v1${path}`,
}))
vi.mock('@/api/authSession', () => ({ getAccessToken }))

import { listPlaygroundModels, streamPlaygroundChat } from '@/api/playground'

describe('playground api', () => {
  beforeEach(() => {
    client.get.mockReset()
    getAccessToken.mockClear()
    vi.unstubAllGlobals()
  })

  it('loads the models for the selected billed group', async () => {
    const result = { group_id: 7, models: ['gpt-5.5', 'gpt-5.4-mini'], default_model: 'gpt-5.5' }
    client.get.mockResolvedValue({ data: result })

    await expect(listPlaygroundModels(7)).resolves.toEqual(result)
    expect(client.get).toHaveBeenCalledWith('/playground/models', { params: { group_id: 7 } })
  })

  it('parses reasoning and content from an OpenAI SSE response', async () => {
    const sse = [
      'data: {"choices":[{"delta":{"reasoning_content":"think "}}]}',
      '',
      'data: {"choices":[{"delta":{"content":"hello"}}]}',
      '',
      'data: [DONE]',
      '',
    ].join('\n')
    const fetchMock = vi.fn().mockResolvedValue(new Response(sse, {
      status: 200,
      headers: { 'Content-Type': 'text/event-stream' },
    }))
    vi.stubGlobal('fetch', fetchMock)
    const updates: Array<[string, string]> = []

    await streamPlaygroundChat({
      group_id: 7,
      model: 'gpt-5.5',
      messages: [{ role: 'user', content: 'hi' }],
    }, (type, chunk) => updates.push([type, chunk]), new AbortController().signal)

    expect(updates).toEqual([['reasoning', 'think '], ['content', 'hello']])
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/playground/chat/completions', expect.objectContaining({
      method: 'POST',
      headers: expect.objectContaining({ Authorization: 'Bearer test-access-token' }),
      body: JSON.stringify({
        group_id: 7,
        model: 'gpt-5.5',
        messages: [{ role: 'user', content: 'hi' }],
        stream: true,
      }),
    }))
  })

  it('surfaces structured gateway errors', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({
      error: { message: 'Selected group is unavailable' },
    }), { status: 403 })))

    await expect(streamPlaygroundChat({
      group_id: 7,
      model: 'gpt-5.5',
      messages: [{ role: 'user', content: 'hi' }],
    }, vi.fn(), new AbortController().signal)).rejects.toThrow('Selected group is unavailable')
  })
})
