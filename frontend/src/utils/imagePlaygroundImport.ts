import type { ApiKey, PublicSettings } from '@/types'

const DEFAULT_IMAGE_PLAYGROUND_URL = 'https://image.ikik.net/'

export interface ImagePlaygroundImportPayload {
  keyId: number
  profileName: string
  apiUrl: string
  apiKey: string
  model: string
  apiMode: 'images'
  streamImages: true
}

interface OpenImagePlaygroundOptions {
  playgroundUrl?: string
  publicSettings?: PublicSettings | null
  model: string
  windowRef?: Window
}

function normalizeApiUrl(value: string): string {
  const trimmed = value.trim().replace(/\/+$/, '')
  return trimmed.endsWith('/v1') ? trimmed : `${trimmed}/v1`
}

export function createImagePlaygroundImportPayload(
  row: ApiKey,
  publicSettings?: PublicSettings | null,
  model = 'gpt-image-2',
): ImagePlaygroundImportPayload {
  const apiBaseUrl = publicSettings?.api_base_url || window.location.origin

  return {
    keyId: row.id,
    profileName: row.name?.trim() || `API Key #${row.id}`,
    apiUrl: normalizeApiUrl(apiBaseUrl),
    apiKey: row.key,
    model,
    apiMode: 'images',
    streamImages: true,
  }
}

export function buildImagePlaygroundUrl(
  playgroundUrl: string,
  payload: ImagePlaygroundImportPayload,
): URL {
  const target = new URL(playgroundUrl)
  target.searchParams.set('apiUrl', payload.apiUrl)
  target.searchParams.set('apiKey', payload.apiKey)
  target.searchParams.set('model', payload.model)
  target.searchParams.set('apiMode', payload.apiMode)
  target.searchParams.set('profileName', payload.profileName)
  target.searchParams.set('streamImages', String(payload.streamImages))
  return target
}

export function openImagePlayground(
  row: ApiKey,
  options: OpenImagePlaygroundOptions,
): Promise<void> {
  const windowRef = options.windowRef ?? window
  const configuredUrl = options.playgroundUrl
    || import.meta.env.VITE_IMAGE_PLAYGROUND_URL
    || DEFAULT_IMAGE_PLAYGROUND_URL
  const payload = createImagePlaygroundImportPayload(row, options.publicSettings, options.model)
  const target = buildImagePlaygroundUrl(configuredUrl, payload)
  const popup = windowRef.open(target.toString(), 'ikik-image-playground')

  return popup
    ? Promise.resolve()
    : Promise.reject(new Error('IMAGE_PLAYGROUND_POPUP_BLOCKED'))
}
