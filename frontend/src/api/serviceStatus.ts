import { apiClient } from './client'

export type OpenAIProductID = 'chatgpt' | 'codex'
export type OpenAIProductStatus =
  | 'operational'
  | 'degraded'
  | 'partial_outage'
  | 'major_outage'

export type ProviderStatusID = 'claude' | 'grok' | 'gemini'
export type ProviderHealthStatus = OpenAIProductStatus | 'unknown'

export interface OpenAIStatusProduct {
  id: OpenAIProductID
  status: OpenAIProductStatus
  active_incidents: number
}

export interface OpenAIStatusIncident {
  id: string
  name: string
  status: string
  impact: string
  products: OpenAIProductID[]
  created_at: string
  updated_at: string
  resolved_at?: string
  latest_body: string
  source_url: string
}

export interface OpenAIStatusSnapshot {
  products: OpenAIStatusProduct[]
  active_incidents: OpenAIStatusIncident[]
  recent_resolved: OpenAIStatusIncident[]
  source_updated_at: string
  fetched_at: string
  stale: boolean
}

export interface ProviderStatusComponent {
  id: string
  name: string
  status: ProviderHealthStatus
}

export interface ProviderStatusIncident {
  id: string
  name: string
  status: string
  impact: string
  updated_at: string
  latest_body: string
}

export interface ProviderStatusProvider {
  id: ProviderStatusID
  status: ProviderHealthStatus
  components: ProviderStatusComponent[]
  active_incidents: ProviderStatusIncident[]
  source_updated_at: string
  stale: boolean
}

export interface ProviderStatusSnapshot {
  providers: ProviderStatusProvider[]
  fetched_at: string
  stale: boolean
}

export async function getOpenAIServiceStatus(options?: {
  signal?: AbortSignal
}): Promise<OpenAIStatusSnapshot> {
  const { data } = await apiClient.get<OpenAIStatusSnapshot>('/service-status/openai', {
    signal: options?.signal,
  })
  return data
}

export async function getProviderServiceStatus(options?: {
  signal?: AbortSignal
}): Promise<ProviderStatusSnapshot> {
  const { data } = await apiClient.get<ProviderStatusSnapshot>('/service-status/providers', {
    signal: options?.signal,
  })
  return data
}
