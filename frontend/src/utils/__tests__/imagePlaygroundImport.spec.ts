import { describe, expect, it, vi } from 'vitest'
import {
  buildImagePlaygroundUrl,
  createImagePlaygroundImportPayload,
  openImagePlayground,
} from '@/utils/imagePlaygroundImport'
import type { ApiKey } from '@/types'

describe('image playground import', () => {
  it('builds the native URL settings import used by the image playground', () => {
    const url = buildImagePlaygroundUrl('https://image.ikik.net/', {
      keyId: 42,
      profileName: 'Main image key',
      apiUrl: 'https://ikik.net/v1',
      apiKey: 'sk-test-image',
      model: 'gpt-image-2',
      apiMode: 'images',
      streamImages: true,
    })

    expect(url.origin).toBe('https://image.ikik.net')
    expect(url.searchParams.get('apiUrl')).toBe('https://ikik.net/v1')
    expect(url.searchParams.get('apiKey')).toBe('sk-test-image')
    expect(url.searchParams.get('model')).toBe('gpt-image-2')
    expect(url.searchParams.get('apiMode')).toBe('images')
    expect(url.searchParams.get('profileName')).toBe('Main image key')
    expect(url.searchParams.get('streamImages')).toBe('true')
  })

  it('creates an Images API profile from the selected key', () => {
    const row = {
      id: 42,
      name: 'Main image key',
      key: 'sk-test-image',
    } as ApiKey

    expect(createImagePlaygroundImportPayload(row, { api_base_url: 'https://ikik.net/' } as never, 'grok-imagine-image')).toEqual({
      keyId: 42,
      profileName: 'Main image key',
      apiUrl: 'https://ikik.net/v1',
      apiKey: 'sk-test-image',
      model: 'grok-imagine-image',
      apiMode: 'images',
      streamImages: true,
    })
  })

  it('opens the image playground with its native import parameters', async () => {
    const open = vi.fn(() => ({ focus: vi.fn() }) as unknown as Window)
    const row = {
      id: 42,
      name: 'Main image key',
      key: 'sk-test-image',
    } as ApiKey

    await openImagePlayground(row, {
      playgroundUrl: 'https://image.ikik.net/',
      publicSettings: { api_base_url: 'https://ikik.net/' } as never,
      model: 'gpt-image-2',
      windowRef: { open } as unknown as Window,
    })

    expect(open).toHaveBeenCalledOnce()
    const target = new URL(open.mock.calls[0][0])
    expect(target.searchParams.get('apiKey')).toBe('sk-test-image')
    expect(target.searchParams.get('apiUrl')).toBe('https://ikik.net/v1')
    expect(target.searchParams.get('model')).toBe('gpt-image-2')
    expect(target.searchParams.get('streamImages')).toBe('true')
  })

})
