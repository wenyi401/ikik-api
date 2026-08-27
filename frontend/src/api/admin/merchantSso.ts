import { apiClient } from '../client'

export interface MerchantSsoIntegration {
  id: number
  merchant_code: string
  merchant_name: string
  enabled: boolean
  register_login_url: string
  login_url: string
  user_sync_url: string
  user_sync_auth_type: 'none' | 'hmac'
  allowed_redirect_hosts: string[]
  hmac_configured: boolean
  created_at: string
  updated_at: string
}

export interface MerchantSsoIntegrationInput {
  merchant_code: string
  merchant_name?: string
  enabled?: boolean
  register_login_url: string
  login_url: string
  user_sync_url: string
  user_sync_auth_type?: 'none' | 'hmac'
  allowed_redirect_hosts: string[]
}

export interface MerchantSsoSyncResult {
  matched: number
  created: number
  updated: number
  skipped: number
}

export interface MerchantSsoBinding {
  id: number
  integration_id: number
  user_id: number
  external_user_id: string
  external_account?: string
  email: string
  status?: string
  created_at: string
  updated_at: string
}

export async function list(): Promise<{ integrations: MerchantSsoIntegration[] }> {
  const { data } = await apiClient.get<{ integrations: MerchantSsoIntegration[] }>('/admin/merchant-sso/integrations')
  return data
}

export async function create(input: MerchantSsoIntegrationInput): Promise<MerchantSsoIntegration> {
  const { data } = await apiClient.post<MerchantSsoIntegration>('/admin/merchant-sso/integrations', input)
  return data
}

export async function update(id: number, input: Partial<MerchantSsoIntegrationInput>): Promise<MerchantSsoIntegration> {
  const { data } = await apiClient.put<MerchantSsoIntegration>(`/admin/merchant-sso/integrations/${id}`, input)
  return data
}

export async function syncUsers(id: number): Promise<MerchantSsoSyncResult> {
  const { data } = await apiClient.post<MerchantSsoSyncResult>(`/admin/merchant-sso/integrations/${id}/sync-users`)
  return data
}

export async function generateHmacSecret(id: number): Promise<{ hmac_secret: string; one_time: boolean }> {
  const { data } = await apiClient.post<{ hmac_secret: string; one_time: boolean }>(`/admin/merchant-sso/integrations/${id}/hmac-secret`)
  return data
}

export async function listBindings(id: number): Promise<{ bindings: MerchantSsoBinding[] }> {
  const { data } = await apiClient.get<{ bindings: MerchantSsoBinding[] }>(`/admin/merchant-sso/integrations/${id}/bindings`)
  return data
}

const merchantSsoAdminAPI = { list, create, update, syncUsers, generateHmacSecret, listBindings }
export default merchantSsoAdminAPI
