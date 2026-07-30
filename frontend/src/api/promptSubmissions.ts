import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'

export type PromptSubmissionStatus = 'pending' | 'approved' | 'rejected'
export type PromptSubmissionType = 'TEXT' | 'STRUCTURED' | 'IMAGE' | 'VIDEO' | 'AUDIO'
export type PromptSubmissionCategory =
  | 'coding'
  | 'writing'
  | 'business'
  | 'creative'
  | 'education'
  | 'workflow'
  | 'productivity'
  | 'other'

export interface PromptSubmission {
  id: number
  user_id?: number
  username: string
  user_email?: string
  title: string
  description: string
  content: string
  type: PromptSubmissionType
  category: PromptSubmissionCategory
  media_url?: string
  status?: PromptSubmissionStatus
  review_note?: string
  reviewed_by?: number
  reviewed_at?: string
  created_at: string
  updated_at?: string
}

export interface CreatePromptSubmissionRequest {
  title: string
  description?: string
  content: string
  type: PromptSubmissionType
  category: PromptSubmissionCategory
  media_url?: string
}

export interface PromptLibraryTranslationConfig {
  enabled: boolean
  group_id: number
  model: string
  target_locale: string
  translated_count: number
}

export async function createPromptSubmission(request: CreatePromptSubmissionRequest): Promise<PromptSubmission> {
  const { data } = await apiClient.post<PromptSubmission>('/prompt-submissions', request)
  return data
}

export async function listApprovedPromptSubmissions(
  page = 1,
  pageSize = 12,
): Promise<BasePaginationResponse<PromptSubmission>> {
  const { data } = await apiClient.get<BasePaginationResponse<PromptSubmission>>('/prompt-submissions/approved', {
    params: { page, page_size: pageSize },
  })
  return data
}

export async function listPromptSubmissionsAdmin(
  page = 1,
  pageSize = 20,
  filters?: { status?: PromptSubmissionStatus | 'all'; search?: string },
): Promise<BasePaginationResponse<PromptSubmission>> {
  const { data } = await apiClient.get<BasePaginationResponse<PromptSubmission>>('/admin/prompt-submissions', {
    params: {
      page,
      page_size: pageSize,
      status: filters?.status === 'all' ? undefined : filters?.status,
      search: filters?.search?.trim() || undefined,
    },
  })
  return data
}

export async function reviewPromptSubmission(
  id: number,
  status: Exclude<PromptSubmissionStatus, 'pending'>,
  note = '',
): Promise<PromptSubmission> {
  const { data } = await apiClient.post<PromptSubmission>(`/admin/prompt-submissions/${id}/review`, {
    status,
    note,
  })
  return data
}

export async function getPromptLibraryTranslationConfig(): Promise<PromptLibraryTranslationConfig> {
  const { data } = await apiClient.get<PromptLibraryTranslationConfig>(
    '/admin/prompt-submissions/translation-config',
  )
  return data
}

export async function updatePromptLibraryTranslationConfig(request: {
  enabled: boolean
  group_id: number
  model: string
}): Promise<PromptLibraryTranslationConfig> {
  const { data } = await apiClient.put<PromptLibraryTranslationConfig>(
    '/admin/prompt-submissions/translation-config',
    request,
  )
  return data
}
