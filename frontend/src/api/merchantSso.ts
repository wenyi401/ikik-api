import { apiClient } from './client'
import type { MerchantSsoIntegration } from './admin/merchantSso'

export async function listIntegrations(): Promise<{ integrations: MerchantSsoIntegration[] }> {
  const { data } = await apiClient.get<{ integrations: MerchantSsoIntegration[] }>('/merchant-sso/integrations')
  return data
}

export async function login(merchantCode: string): Promise<{ redirect_url: string; first_login: boolean }> {
  const { data } = await apiClient.post<{ redirect_url: string; first_login: boolean }>(`/merchant-sso/${encodeURIComponent(merchantCode)}/login`)
  return data
}

export const merchantSsoAPI = { listIntegrations, login }
export default merchantSsoAPI
