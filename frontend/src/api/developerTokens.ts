import { apiClient } from './client'

export const DEVELOPER_TOKEN_SCOPES = [
  'accounts:read',
  'accounts:write',
  'accounts:share',
  'bot:access'
] as const

export type DeveloperTokenScope = typeof DEVELOPER_TOKEN_SCOPES[number]

export interface DeveloperToken {
  id: number
  user_id: number
  name: string
  token_prefix: string
  scopes: DeveloperTokenScope[]
  status: string
  expires_at?: string | null
  last_used_at?: string | null
  last_used_ip?: string | null
  created_at: string
  updated_at: string
}

export interface CreateDeveloperTokenRequest {
  name: string
  scopes: DeveloperTokenScope[]
  expires_at?: string
}

export interface CreatedDeveloperToken {
  developer_token: DeveloperToken
  token: string
}

export async function listDeveloperTokens(): Promise<DeveloperToken[]> {
  const { data } = await apiClient.get<DeveloperToken[]>('/developer-tokens')
  return data
}

export async function createDeveloperToken(
  request: CreateDeveloperTokenRequest
): Promise<CreatedDeveloperToken> {
  const { data } = await apiClient.post<CreatedDeveloperToken>('/developer-tokens', request)
  return data
}

export async function revokeDeveloperToken(id: number): Promise<void> {
  await apiClient.delete(`/developer-tokens/${id}`)
}

export const developerTokensAPI = {
  list: listDeveloperTokens,
  create: createDeveloperToken,
  revoke: revokeDeveloperToken
}

export default developerTokensAPI
