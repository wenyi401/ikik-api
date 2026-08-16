import { apiClient, buildApiUrl } from './client'
import { getAccessToken } from './authSession'

export interface PetAsset {
  id: string
  pet_key: string
  display_name: string
  description: string
  sprite_version: 1 | 2
  sha256: string
  size_bytes: number
  width: number
  height: number
  license: string
  created_at: string
  asset_url: string
  is_builtin: boolean
}

export interface PetPreferences {
  selected_asset_id: string | null
  assistant_group_id: number | null
  assistant_model: string
  enabled: boolean
  size: 'small' | 'medium' | 'large'
  anchor: 'bottom-left' | 'bottom-right'
  reduced_motion: boolean
  activity_reactions: boolean
  position_x: number | null
  position_y: number | null
}

export interface PetActivityEvent {
  id: string
  type: 'api.request' | 'assistant.thinking' | 'assistant.done' | 'assistant.error'
  status: 'started' | 'completed' | 'failed'
  endpoint?: string
  request_id?: string
  occurred_at: string
}

export const petAPI = {
  async listAssets(): Promise<PetAsset[]> {
    const { data } = await apiClient.get<PetAsset[]>('/pet/assets')
    return data
  },
  async importAsset(file: File): Promise<PetAsset> {
    const body = new FormData()
    body.append('file', file)
    const { data } = await apiClient.post<PetAsset>('/pet/assets/import', body, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 60_000,
    })
    return data
  },
  async deleteAsset(id: string): Promise<void> {
    await apiClient.delete(`/pet/assets/${id}`)
  },
  async fetchSpritesheet(asset: PetAsset): Promise<Blob> {
    const url = asset.asset_url || `/pet/assets/${asset.id}/spritesheet`
    if (/^https:\/\//i.test(url) || url.startsWith('/pets/')) {
      const response = await fetch(url, {
        credentials: url.startsWith('/pets/') ? 'same-origin' : 'omit',
        cache: 'force-cache',
      })
      if (!response.ok) throw new Error(`Pet asset returned HTTP ${response.status}`)
      return response.blob()
    }
    // asset_url is an absolute API path, while apiClient already has /api/v1
    // as its base URL. Strip the duplicated prefix before passing it to Axios.
    const apiPath = url.startsWith('/api/v1/') ? url.slice('/api/v1'.length) : url
    const { data } = await apiClient.get<Blob>(apiPath, {
      responseType: 'blob',
      timeout: 60_000,
    })
    return data
  },
  async getPreferences(): Promise<PetPreferences> {
    const { data } = await apiClient.get<PetPreferences>('/pet/preferences')
    return data
  },
  async savePreferences(input: PetPreferences): Promise<PetPreferences> {
    const { data } = await apiClient.put<PetPreferences>('/pet/preferences', input)
    return data
  },
}

export async function openPetActivityStream(signal: AbortSignal): Promise<Response> {
  const token = getAccessToken()
  if (!token) throw new Error('Authentication required')
  return fetch(buildApiUrl('/pet/activity/stream'), {
    method: 'GET',
    headers: {
      Accept: 'text/event-stream',
      Authorization: `Bearer ${token}`,
    },
    credentials: 'include',
    cache: 'no-store',
    signal,
  })
}
